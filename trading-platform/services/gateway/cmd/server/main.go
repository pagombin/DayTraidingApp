package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trading-platform/gateway/internal/auth"
	"trading-platform/gateway/internal/config"
	"trading-platform/gateway/internal/routes"
	"trading-platform/gateway/internal/ws"
	"trading-platform/gateway/pkg/db"
	"trading-platform/gateway/pkg/redis"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"go.uber.org/zap"
)

var startTime = time.Now()

func main() {
	// Initialize logger
	var zapLog *zap.Logger
	var err error
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "debug" {
		zapLog, err = zap.NewDevelopment()
	} else {
		zapLog, err = zap.NewProduction()
	}
	if err != nil {
		panic(fmt.Sprintf("failed to initialize logger: %v", err))
	}
	defer zapLog.Sync()

	sugar := zapLog.Sugar()
	sugar.Info("Starting API Gateway...")

	// Connect to PostgreSQL
	pool, err := db.Connect()
	if err != nil {
		sugar.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer pool.Close()
	sugar.Info("Connected to PostgreSQL")

	// Connect to Redis
	rdb, err := redis.Connect()
	if err != nil {
		sugar.Fatalf("Failed to connect to Redis: %v", err)
	}
	defer rdb.Close()
	sugar.Info("Connected to Redis")

	// Ensure config seed data exists
	if err := seedConfigIfEmpty(pool, sugar); err != nil {
		sugar.Warnf("Config seed check: %v", err)
	}

	// Initialize config hot-reload
	configMgr := config.NewManager(rdb.Client, sugar)
	go configMgr.SubscribeChanges(context.Background())

	// Initialize JWT manager
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "generate_a_real_secret"
	}
	jwtMgr := auth.NewJWTManager(jwtSecret)

	// Create WebSocket hub
	hub := ws.NewHub(sugar)
	go hub.Run()

	// Subscribe to Redis health updates and forward to WebSocket
	go subscribeRedisToWS(rdb, hub, sugar)

	// Create Fiber app
	app := fiber.New(fiber.Config{
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	})

	app.Use(recover.New())
	app.Use(requestid.New())
	app.Use(logger.New(logger.Config{
		Format:     "${time} | ${status} | ${latency} | ${method} | ${path} | ${ip} | ${reqHeader:X-Request-ID}\n",
		TimeFormat: time.RFC3339,
	}))
	corsOrigin := os.Getenv("CORS_ORIGIN")
	if corsOrigin == "" {
		corsOrigin = "http://localhost:3000"
	}
	app.Use(cors.New(cors.Config{
		AllowOrigins: corsOrigin,
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin,Content-Type,Accept,Authorization",
	}))

	// Register routes
	routes.Setup(app, pool, rdb, jwtMgr, hub, sugar, startTime)

	// Start WebSocket server on separate port
	wsApp := fiber.New()
	wsApp.Use(cors.New(cors.Config{
		AllowOrigins: corsOrigin,
	}))
	ws.SetupRoutes(wsApp, hub)

	// Start servers
	go func() {
		wsPort := os.Getenv("WS_PORT")
		if wsPort == "" {
			wsPort = "8081"
		}
		sugar.Infof("WebSocket server starting on :%s", wsPort)
		if err := wsApp.Listen(":" + wsPort); err != nil {
			sugar.Fatalf("WebSocket server failed: %v", err)
		}
	}()

	go func() {
		port := os.Getenv("GATEWAY_PORT")
		if port == "" {
			port = "8080"
		}
		sugar.Infof("API Gateway starting on :%s", port)
		if err := app.Listen(":" + port); err != nil {
			sugar.Fatalf("API server failed: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	sugar.Info("Shutting down gracefully...")
	_ = app.ShutdownWithTimeout(30 * time.Second)
	_ = wsApp.ShutdownWithTimeout(30 * time.Second)
	sugar.Info("Server stopped")
}

func seedConfigIfEmpty(pool *pgxpool.Pool, log *zap.SugaredLogger) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var count int
	err := pool.QueryRow(ctx, "SELECT COUNT(*) FROM app_config").Scan(&count)
	if err != nil {
		return fmt.Errorf("failed to check app_config: %w", err)
	}
	if count > 0 {
		log.Infof("Config already seeded (%d items)", count)
		return nil
	}

	log.Info("Seeding default configuration...")
	_, err = pool.Exec(ctx, `
INSERT INTO app_config (key, value, category, label, description, value_type, constraints) VALUES
('broker.provider', '"alpaca"', 'broker', 'Broker', 'Which broker to use for trading', 'select',
 '{"options": ["alpaca", "ibkr", "tradier", "schwab"]}'),
('broker.api_key', '""', 'broker', 'API Key', 'Your broker API key', 'string', NULL),
('broker.api_secret', '""', 'broker', 'API Secret', 'Your broker API secret (stored encrypted)', 'string', NULL),
('broker.paper_mode', 'true', 'broker', 'Paper Trading Mode', 'When enabled, trades use simulated money', 'boolean', NULL),
('risk.max_daily_loss', '500', 'risk', 'Maximum Daily Loss ($)', 'If losses reach this amount, all trading stops', 'number',
 '{"min": 50, "max": 100000, "step": 50}'),
('risk.max_position_pct', '5', 'risk', 'Max Position Size (%)', 'Maximum percentage of portfolio in any single trade', 'number',
 '{"min": 1, "max": 25, "step": 1}'),
('risk.max_portfolio_risk_pct', '15', 'risk', 'Max Total Portfolio Risk (%)', 'Maximum total risk across all open positions', 'number',
 '{"min": 5, "max": 50, "step": 5}'),
('risk.require_stop_loss', 'true', 'risk', 'Require Stop Loss', 'Every trade must have a stop-loss order', 'boolean', NULL),
('risk.max_orders_per_minute', '10', 'risk', 'Max Orders Per Minute', 'Rate limit to prevent runaway algorithms', 'number',
 '{"min": 1, "max": 60, "step": 1}'),
('data.provider', '"alpaca"', 'data', 'Market Data Provider', 'Where real-time price data comes from', 'select',
 '{"options": ["alpaca", "polygon", "tradier", "alpha_vantage"]}'),
('data.api_key', '""', 'data', 'Data API Key', 'API key for your market data provider', 'string', NULL),
('notifications.email', '""', 'notifications', 'Email Address', 'Where to send email alerts', 'string', NULL),
('notifications.sms_phone', '""', 'notifications', 'Phone Number', 'SMS alerts for critical events', 'string', NULL),
('notifications.slack_webhook', '""', 'notifications', 'Slack Webhook URL', 'Post alerts to a Slack channel', 'string', NULL),
('notifications.discord_webhook', '""', 'notifications', 'Discord Webhook URL', 'Post alerts to a Discord channel', 'string', NULL),
('notifications.telegram_bot_token', '""', 'notifications', 'Telegram Bot Token', 'Telegram bot for alerts', 'string', NULL),
('notifications.telegram_chat_id', '""', 'notifications', 'Telegram Chat ID', 'Telegram chat to post alerts to', 'string', NULL),
('watchlist.symbols', '["SPY","QQQ","AAPL","MSFT","NVDA","AMZN","GOOG","TSLA","META","JPM"]', 'watchlist',
 'Watchlist', 'Symbols you want to monitor and trade', 'json', NULL),
('system.log_level', '"info"', 'system', 'Log Level', 'How verbose the system logs are', 'select',
 '{"options": ["debug", "info", "warning", "error"]}'),
('system.auto_update', 'true', 'system', 'Auto-Update', 'Check for updates automatically', 'boolean', NULL),
('system.backup_reminder_days', '30', 'system', 'Backup Reminder (days)', 'How often to remind you to back up data', 'number',
 '{"min": 7, "max": 90, "step": 7}')
ON CONFLICT (key) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("failed to seed config: %w", err)
	}
	log.Info("Default configuration seeded successfully")
	return nil
}

func subscribeRedisToWS(rdb *redis.Client, hub *ws.Hub, log *zap.SugaredLogger) {
	ctx := context.Background()
	sub := rdb.Subscribe(ctx, "health:updates", "config:changed")
	defer sub.Close()

	ch := sub.Channel()
	for msg := range ch {
		hub.Broadcast([]byte(fmt.Sprintf(`{"channel":"%s","data":%s}`, msg.Channel, msg.Payload)))
	}
}
