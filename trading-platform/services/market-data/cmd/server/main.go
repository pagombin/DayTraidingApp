package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"trading-platform/market-data/internal/aggregator"
	"trading-platform/market-data/internal/backfill"
	"trading-platform/market-data/internal/calendar"
	"trading-platform/market-data/internal/config"
	"trading-platform/market-data/internal/health"
	"trading-platform/market-data/internal/metrics"
	"trading-platform/market-data/internal/model"
	"trading-platform/market-data/internal/normalizer"
	"trading-platform/market-data/internal/provider"
	"trading-platform/market-data/internal/provider/alpaca"
	mockprov "trading-platform/market-data/internal/provider/mock"
	"trading-platform/market-data/internal/publisher"
	"trading-platform/market-data/internal/validator"
)

var startTime = time.Now()

func main() {
	// Initialize logger
	var zapLog *zap.Logger
	var err error
	cfg := config.Load()
	if cfg.LogLevel == "debug" {
		zapLog, err = zap.NewDevelopment()
	} else {
		zapLog, err = zap.NewProduction()
	}
	if err != nil {
		panic(fmt.Sprintf("failed to init logger: %v", err))
	}
	defer zapLog.Sync()
	logger := zapLog.Named("market-data")
	sugar := logger.Sugar()

	sugar.Info("Starting Market Data Service...")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to PostgreSQL
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		sugar.Fatalf("Failed to parse DB config: %v", err)
	}
	poolCfg.MaxConns = 20
	poolCfg.MinConns = 5
	poolCfg.MaxConnLifetime = 30 * time.Minute
	poolCfg.HealthCheckPeriod = 30 * time.Second

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		sugar.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pool.Close()
	sugar.Info("Connected to PostgreSQL")

	// Connect to Redis
	redisOpts, err := goredis.ParseURL(cfg.RedisURL)
	if err != nil {
		sugar.Fatalf("Failed to parse Redis URL: %v", err)
	}
	redisOpts.PoolSize = 10
	redisOpts.MinIdleConns = 3
	rdb := goredis.NewClient(redisOpts)
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		sugar.Fatalf("Failed to connect to Redis: %v", err)
	}
	sugar.Info("Connected to Redis")

	// Initialize calendar
	cal := calendar.NewUSMarketCalendar()
	sugar.Infof("Market status: %s", cal.MarketStatus(time.Now()))

	// Load watchlist
	symbols, err := config.LoadWatchlist(ctx, pool)
	if err != nil {
		sugar.Warnf("Failed to load watchlist, using defaults: %v", err)
		symbols = []string{"SPY", "QQQ", "AAPL", "MSFT", "GOOGL", "AMZN", "NVDA", "META", "TSLA", "IWM"}
	}
	sugar.Infof("Watchlist: %v (%d symbols)", symbols, len(symbols))

	// Initialize data provider
	var prov provider.DataProvider
	if cfg.AlpacaAPIKey != "" && cfg.AlpacaAPISecret != "" {
		prov = alpaca.NewAlpacaProvider(cfg.AlpacaAPIKey, cfg.AlpacaAPISecret, cfg.AlpacaDataURL, cfg.AlpacaRestURL, logger)
		sugar.Info("Using Alpaca data provider")
	} else {
		prov = mockprov.NewMockProvider(logger)
		sugar.Warn("No Alpaca API key configured — using mock data provider")
	}

	// Initialize pipeline components
	met := metrics.New()
	met.StartRateTracker()
	defer met.Stop()

	redisPublisher := publisher.NewRedisPublisher(rdb, logger)
	redisPublisher.Start(ctx)
	defer redisPublisher.Stop()

	dbWriter := publisher.NewDBWriter(pool, logger)
	dbWriter.Start(ctx)
	defer dbWriter.Stop()

	barAgg := aggregator.NewBarAggregator()
	barAgg.Start()
	defer barAgg.Stop()

	// Track last known prices for validation
	lastKnown := &sync.Map{}

	// Connect to provider
	go func() {
		if err := prov.Connect(ctx); err != nil {
			sugar.Errorf("Provider connect failed: %v", err)
		}
	}()

	// Wait briefly for connection, then subscribe
	time.Sleep(2 * time.Second)
	if err := prov.Subscribe(ctx, symbols); err != nil {
		sugar.Warnf("Initial subscribe failed: %v", err)
	}

	// Start tick rate tracker for Alpaca provider
	if ap, ok := prov.(*alpaca.AlpacaProvider); ok {
		ap.StartTickRateTracker(ctx)
	}

	// Fetch latest snapshots via REST so dashboard has data even when market is closed
	go func() {
		time.Sleep(3 * time.Second) // let connections settle
		sugar.Info("Fetching latest quote snapshots via REST...")

		if ap, ok := prov.(*alpaca.AlpacaProvider); ok {
			// Use batch snapshot endpoint for efficiency and richer data (volume, prev close)
			results, err := ap.FetchMultiSnapshots(ctx, symbols)
			if err != nil {
				sugar.Warnw("Multi-snapshot fetch failed, falling back to individual", "error", err)
				for _, sym := range symbols {
					tick, err := prov.FetchLatestQuote(ctx, sym)
					if err != nil {
						sugar.Debugw("Snapshot fetch failed", "symbol", sym, "error", err)
						continue
					}
					if tick != nil {
						redisPublisher.PublishTick(ctx, *tick)
					}
				}
			} else {
				for _, r := range results {
					if err := redisPublisher.PublishTick(ctx, r.Tick); err != nil {
						sugar.Debugw("Snapshot publish failed", "symbol", r.Tick.Symbol, "error", err)
					}
					// Store prev close in Redis for change calculation
					if r.PrevClose > 0 {
						key := fmt.Sprintf("market:prevclose:%s", r.Tick.Symbol)
						rdb.Set(ctx, key, fmt.Sprintf("%.4f", r.PrevClose), 24*time.Hour)
					}
				}
				sugar.Infow("Multi-snapshot fetch complete", "count", len(results))
			}
		} else {
			for _, sym := range symbols {
				tick, err := prov.FetchLatestQuote(ctx, sym)
				if err != nil {
					sugar.Debugw("Snapshot fetch failed", "symbol", sym, "error", err)
					continue
				}
				if tick != nil {
					redisPublisher.PublishTick(ctx, *tick)
				}
			}
		}

		sugar.Info("Snapshot fetch complete")
	}()

	// Periodic snapshot refresh — ensures data stays current even if WebSocket has gaps
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if ap, ok := prov.(*alpaca.AlpacaProvider); ok {
					results, err := ap.FetchMultiSnapshots(ctx, symbols)
					if err != nil {
						sugar.Debugw("Periodic snapshot refresh failed", "error", err)
						continue
					}
					for _, r := range results {
						redisPublisher.PublishTick(ctx, r.Tick)
						if r.PrevClose > 0 {
							key := fmt.Sprintf("market:prevclose:%s", r.Tick.Symbol)
							rdb.Set(ctx, key, fmt.Sprintf("%.4f", r.PrevClose), 24*time.Hour)
						}
					}
				}
			}
		}
	}()

	// Main ingestion loop — ticks
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case tick, ok := <-prov.TickChannel():
				if !ok {
					return
				}
				tick = normalizer.NormalizeTick(tick)

				// Validate
				var prev *model.MarketTick
				if v, ok := lastKnown.Load(tick.Symbol); ok {
					t := v.(model.MarketTick)
					prev = &t
				}
				if err := validator.ValidateTick(tick, prev); err != nil {
					met.ValidationRejected.Add(1)
					sugar.Debugw("Tick rejected", "error", err)
					continue
				}

				// Merge with last known state to avoid overwriting fields with zeros.
				// Quote ticks have bid/ask but no volume; trade ticks have last/volume but no bid/ask.
				if prev != nil {
					if tick.Bid.IsZero() && prev.Bid.IsPositive() {
						tick.Bid = prev.Bid
					}
					if tick.Ask.IsZero() && prev.Ask.IsPositive() {
						tick.Ask = prev.Ask
					}
					if tick.Volume == 0 && prev.Volume > 0 {
						tick.Volume = prev.Volume
					}
					if tick.Last.IsZero() && prev.Last.IsPositive() {
						tick.Last = prev.Last
					}
				}

				lastKnown.Store(tick.Symbol, tick)
				met.TotalTicks.Add(1)

				// Publish to Redis
				if err := redisPublisher.PublishTick(ctx, tick); err != nil {
					met.RedisPublishErrors.Add(1)
				}

				// Add to DB buffer
				dbWriter.AddTick(tick)

				// Add to bar aggregator
				barAgg.AddTick(tick)
			}
		}
	}()

	// Bar processing loop — from provider
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case bar, ok := <-prov.BarChannel():
				if !ok {
					return
				}
				bar = normalizer.NormalizeBar(bar)
				if err := redisPublisher.PublishBar(ctx, bar); err != nil {
					sugar.Debugw("Failed to publish bar", "error", err)
				}
			}
		}
	}()

	// Bar processing loop — from aggregator
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case bar, ok := <-barAgg.OutputChannel():
				if !ok {
					return
				}
				if err := redisPublisher.PublishBar(ctx, bar); err != nil {
					sugar.Debugw("Failed to publish aggregated bar", "error", err)
				}
			}
		}
	}()

	// Status update loop
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case state, ok := <-prov.StatusChannel():
				if !ok {
					return
				}
				sugar.Infow("Provider state changed", "state", state)
				if state == model.StateReconnecting {
					met.ReconnectionCount.Add(1)
				}
			}
		}
	}()

	// Subscribe to config hot-reload for watchlist changes
	symbolsMu := &sync.Mutex{}
	go config.SubscribeConfigChanges(ctx, rdb, logger, func(newSymbols []string) {
		symbolsMu.Lock()
		defer symbolsMu.Unlock()

		currentSet := make(map[string]bool)
		for _, s := range symbols {
			currentSet[s] = true
		}
		newSet := make(map[string]bool)
		for _, s := range newSymbols {
			newSet[s] = true
		}

		var toAdd, toRemove []string
		for _, s := range newSymbols {
			if !currentSet[s] {
				toAdd = append(toAdd, s)
			}
		}
		for _, s := range symbols {
			if !newSet[s] {
				toRemove = append(toRemove, s)
			}
		}

		if len(toAdd) > 0 {
			prov.Subscribe(ctx, toAdd)
		}
		if len(toRemove) > 0 {
			prov.Unsubscribe(ctx, toRemove)
		}
		symbols = newSymbols
	})

	// Run historical backfill in background
	go func() {
		time.Sleep(5 * time.Second)
		bf := backfill.NewHistoricalBackfiller(prov, dbWriter, pool, logger)
		if err := bf.BackfillAll(ctx, symbols); err != nil {
			sugar.Warnf("Historical backfill error: %v", err)
		}
	}()

	// Register with watchdog
	go func() {
		time.Sleep(3 * time.Second)
		_, err := pool.Exec(ctx,
			`INSERT INTO registered_services (name, url, check_type, host, port, critical)
			 VALUES ('market-data', 'http://market-data:8083/healthz', 'http', 'market-data', 8083, true)
			 ON CONFLICT (name) DO UPDATE SET url = EXCLUDED.url`)
		if err != nil {
			sugar.Warnf("Failed to register with watchdog: %v", err)
		} else {
			sugar.Info("Registered with watchdog")
		}
	}()

	// HTTP server for health + status
	app := fiber.New(fiber.Config{DisableStartupMessage: true})
	healthHandler := health.NewHandler(pool, rdb, startTime, prov.IsConnected)
	app.Get("/healthz", healthHandler.Healthz)
	app.Get("/readyz", healthHandler.Readyz)
	app.Get("/status", func(c *fiber.Ctx) error {
		result := fiber.Map{
			"connected":     prov.IsConnected(),
			"provider":      prov.Name(),
			"symbol_count":  len(symbols),
			"market_status": cal.MarketStatus(time.Now()),
			"metrics":       met.Snapshot(),
			"db_buffer":     dbWriter.BufferSize(),
		}
		if ap, ok := prov.(*alpaca.AlpacaProvider); ok {
			result["provider_status"] = ap.GetStatus()
		}
		return c.JSON(result)
	})

	go func() {
		sugar.Infof("HTTP server starting on :%s", cfg.HTTPPort)
		if err := app.Listen(":" + cfg.HTTPPort); err != nil {
			sugar.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	sugar.Info("Shutting down Market Data Service...")
	cancel()
	prov.Disconnect()
	_ = app.ShutdownWithTimeout(10 * time.Second)
	sugar.Info("Market Data Service stopped")
}
