package server

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
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"go.uber.org/zap"
)

var startTime = time.Now()

func Run() {
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

func subscribeRedisToWS(rdb *redis.Client, hub *ws.Hub, log *zap.SugaredLogger) {
	ctx := context.Background()
	sub := rdb.Subscribe(ctx, "health:updates", "config:changed")
	defer sub.Close()

	ch := sub.Channel()
	for msg := range ch {
		hub.Broadcast([]byte(fmt.Sprintf(`{"channel":"%s","data":%s}`, msg.Channel, msg.Payload)))
	}
}
