package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trading-platform/oms/internal/autonomy"
	"trading-platform/oms/internal/broker"
	"trading-platform/oms/internal/broker/alpaca"
	"trading-platform/oms/internal/broker/paper"
	"trading-platform/oms/internal/config"
	"trading-platform/oms/internal/fill"
	"trading-platform/oms/internal/health"
	"trading-platform/oms/internal/killswitch"
	"trading-platform/oms/internal/model"
	"trading-platform/oms/internal/order"
	"trading-platform/oms/internal/portfolio"
	"trading-platform/oms/internal/risk"
	sigpkg "trading-platform/oms/internal/signal"
	"trading-platform/oms/pkg/audit"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

func main() {
	cfg := config.Load()

	logger, _ := zap.NewProduction()
	if cfg.LogLevel == "debug" {
		logger, _ = zap.NewDevelopment()
	}
	defer logger.Sync()
	log := logger.Sugar()

	log.Info("Starting OMS service...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to PostgreSQL
	dbURL := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.DBUser, cfg.DBPassword, cfg.DBHost, cfg.DBPort, cfg.DBName)

	poolConfig, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		log.Fatalw("Failed to parse DB config", "error", err)
	}
	poolConfig.MaxConns = 15
	poolConfig.MinConns = 3

	var pool *pgxpool.Pool
	for attempt := 0; attempt < 5; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		if err == nil {
			if err = pool.Ping(ctx); err == nil {
				break
			}
		}
		log.Warnw("DB connection failed, retrying...", "attempt", attempt+1, "error", err)
		time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
	}
	if err != nil {
		log.Fatalw("Failed to connect to PostgreSQL", "error", err)
	}
	defer pool.Close()
	log.Info("Connected to PostgreSQL")

	// Connect to Redis
	opt, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		log.Fatalw("Failed to parse Redis URL", "error", err)
	}
	rdb := redis.NewClient(opt)
	for attempt := 0; attempt < 5; attempt++ {
		if err = rdb.Ping(ctx).Err(); err == nil {
			break
		}
		log.Warnw("Redis connection failed, retrying...", "attempt", attempt+1, "error", err)
		time.Sleep(time.Duration(1<<uint(attempt)) * time.Second)
	}
	if err != nil {
		log.Fatalw("Failed to connect to Redis", "error", err)
	}
	defer rdb.Close()
	log.Info("Connected to Redis")

	// Load config from DB
	if err := cfg.LoadFromDB(pool); err != nil {
		log.Warnw("Failed to load config from DB, using defaults", "error", err)
	}
	cfg.SubscribeChanges(ctx, rdb, log, pool)

	// Initialize components
	auditLogger := audit.NewLogger(pool, log)
	persistence := order.NewPersistence(pool, log)
	stateMachine := order.NewStateMachine(persistence, rdb, auditLogger, log)
	riskManager := risk.NewRiskManager(cfg, rdb, log)
	circuitBreaker := risk.NewCircuitBreaker(cfg, rdb, log)
	autonomyGate := autonomy.NewAutonomyGate(cfg, log)
	signalReceiver := sigpkg.NewReceiver(rdb, log)

	// Initialize broker adapter
	var brokerAdapter broker.BrokerAdapter
	if cfg.PaperMode {
		log.Info("Starting in PAPER TRADING mode")
		brokerAdapter = paper.NewPaperSimulator(rdb, log)
	} else {
		baseURL := cfg.AlpacaLiveURL
		log.Infow("Starting in LIVE mode", "broker", "alpaca")
		brokerAdapter = alpaca.NewClient(cfg.AlpacaAPIKey, cfg.AlpacaAPISecret, baseURL, log)
	}

	// Kill switch
	ks := killswitch.NewKillSwitch(brokerAdapter, rdb, auditLogger, log)

	// Portfolio tracker
	portfolioTracker := portfolio.NewPortfolioTracker(pool, rdb, brokerAdapter, circuitBreaker, log)
	if err := portfolioTracker.ReconcileWithBroker(ctx); err != nil {
		log.Warnw("Failed to reconcile with broker on startup", "error", err)
	}
	portfolioTracker.LoadPositionsFromDB(ctx)

	// Fill processor
	fillProcessor := fill.NewProcessor(pool, rdb, stateMachine, persistence, auditLogger, log)
	updateCh, _ := brokerAdapter.StreamUpdates(ctx)
	fillProcessor.StartFillListener(ctx, updateCh)

	// Start portfolio tick subscription
	portfolioTracker.StartTickSubscription(ctx)

	// Start signal receiver
	signalReceiver.Start(ctx)

	// Process signals in background
	go processSignals(ctx, signalReceiver, autonomyGate, riskManager, stateMachine,
		persistence, portfolioTracker, brokerAdapter, ks, auditLogger, log)

	// Register with watchdog
	go func() {
		time.Sleep(3 * time.Second)
		_, err := pool.Exec(ctx,
			`INSERT INTO registered_services (name, url, check_type, host, port, critical)
			 VALUES ('oms', 'http://oms:8084/healthz', 'http', 'oms', 8084, true)
			 ON CONFLICT (name) DO UPDATE SET url = EXCLUDED.url`)
		if err != nil {
			log.Warnf("Failed to register with watchdog: %v", err)
		} else {
			log.Info("Registered with watchdog")
		}
	}()

	// HTTP server
	startTime := time.Now()
	app := fiber.New(fiber.Config{
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	})
	app.Use(recover.New())

	healthHandler := health.NewHandler(pool, rdb, startTime)
	app.Get("/healthz", healthHandler.Healthz)
	app.Get("/readyz", healthHandler.Readyz)

	app.Get("/status", func(c *fiber.Ctx) error {
		snapshot := portfolioTracker.GetSnapshot()
		return c.JSON(fiber.Map{
			"status":          "running",
			"mode":            brokerAdapter.Name(),
			"kill_switch":     ks.IsActive(),
			"positions":       snapshot.PositionCount,
			"total_equity":    snapshot.TotalEquity.StringFixed(2),
			"daily_pnl":       snapshot.DailyPnL.StringFixed(2),
			"autonomy_level":  cfg.GetAutonomyLevel(),
			"circuit_breaker": string(circuitBreaker.GetState()),
		})
	})

	mode := "paper"
	if !cfg.PaperMode {
		mode = "live"
	}
	snapshot := portfolioTracker.GetSnapshot()
	log.Infow("OMS started",
		"mode", mode,
		"positions", snapshot.PositionCount,
		"equity", snapshot.TotalEquity.StringFixed(2),
		"kill_switch", ks.IsActive(),
		"autonomy_level", cfg.GetAutonomyLevel())

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
		<-sigCh
		log.Info("Shutting down OMS...")
		cancel()
		portfolioTracker.SaveDailySnapshot(context.Background())
		rdb.Publish(context.Background(), "oms:status", `{"event":"shutting_down"}`)
		time.Sleep(2 * time.Second)
		app.Shutdown()
	}()

	log.Infow("HTTP server starting", "port", cfg.HTTPPort)
	if err := app.Listen(":" + cfg.HTTPPort); err != nil {
		log.Fatalw("HTTP server failed", "error", err)
	}
}

func processSignals(ctx context.Context, receiver *sigpkg.Receiver, gate *autonomy.AutonomyGate,
	riskMgr *risk.RiskManager, sm *order.StateMachine, persist *order.Persistence,
	tracker *portfolio.PortfolioTracker, ba broker.BrokerAdapter,
	ks *killswitch.KillSwitch, al *audit.Logger, log *zap.SugaredLogger) {

	for {
		select {
		case <-ctx.Done():
			return
		case sig := <-receiver.Signals():
			if ks.IsActive() {
				log.Warnw("Kill switch active, ignoring signal", "symbol", sig.Symbol)
				continue
			}

			// Create order from signal
			ord := &model.Order{
				OrderID:     uuid.New(),
				StrategyID:  sig.StrategyID,
				AccountID:   "default",
				Symbol:      sig.Symbol,
				Side:        sig.Side,
				OrderType:   sig.OrderType,
				Quantity:    sig.Quantity,
				LimitPrice:  sig.LimitPrice,
				StopPrice:   sig.StopPrice,
				TimeInForce: model.TIFDay,
				SignalID:    sig.SignalID,
				State:       model.OrderPending,
				CreatedAt:   time.Now().UTC(),
				UpdatedAt:   time.Now().UTC(),
			}

			// Autonomy gate
			proceed, state, reason := gate.Evaluate(sig)
			if !proceed {
				log.Infow("Signal blocked by autonomy gate", "reason", reason, "symbol", sig.Symbol)
				continue
			}

			ord.State = state

			// Persist the order
			if err := persist.CreateOrder(ctx, ord); err != nil {
				log.Errorw("Failed to create order", "error", err, "symbol", sig.Symbol)
				continue
			}

			if state == model.OrderAwaitingApproval {
				gate.AddPendingApproval(ord)
				log.Infow("Order awaiting approval", "order_id", ord.OrderID, "symbol", ord.Symbol)
				continue
			}

			// Risk checks
			snapshot := tracker.GetSnapshot()
			checks, allPassed := riskMgr.CheckAll(ctx, ord, &snapshot)
			persist.SaveRiskCheckResults(ctx, ord.OrderID, checks)

			if !allPassed {
				rejectionReason := ""
				for _, c := range checks {
					if !c.Passed {
						rejectionReason = c.Reason
						break
					}
				}
				ord.RejectionReason = rejectionReason
				sm.Transition(ctx, ord, model.OrderRejected, rejectionReason)
				log.Infow("Order rejected by risk manager", "order_id", ord.OrderID, "reason", rejectionReason)
				continue
			}

			// Submit to broker
			sm.Transition(ctx, ord, model.OrderSubmitted, "Passed risk checks")
			brokerID, err := ba.SubmitOrder(ctx, ord)
			if err != nil {
				sm.Transition(ctx, ord, model.OrderFailed, err.Error())
				log.Errorw("Failed to submit order to broker", "error", err, "order_id", ord.OrderID)
				continue
			}

			ord.BrokerOrderID = brokerID
			persist.SaveOrderState(ctx, ord)
			sm.Transition(ctx, ord, model.OrderAccepted, "Broker accepted")
			log.Infow("Order submitted to broker", "order_id", ord.OrderID, "broker_id", brokerID)
		}
	}
}
