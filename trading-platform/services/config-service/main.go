package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"trading-platform/config-service/internal/handler"
	"trading-platform/config-service/internal/store"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Build PostgreSQL connection string from environment variables.
	dbHost := envOrDefault("DB_HOST", "localhost")
	dbPort := envOrDefault("DB_PORT", "5432")
	dbUser := envOrDefault("DB_USER", "trading")
	dbPassword := envOrDefault("DB_PASSWORD", "changeme_in_production")
	dbName := envOrDefault("DB_NAME", "trading_platform")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName,
	)

	// Connect to PostgreSQL with pool configuration and retry.
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		logger.Fatal("failed to parse database config", zap.Error(err))
	}

	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.HealthCheckPeriod = 30 * time.Second

	var pool *pgxpool.Pool
	for attempt := 0; attempt < 5; attempt++ {
		connectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		pool, err = pgxpool.NewWithConfig(connectCtx, poolConfig)
		cancel()
		if err == nil {
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
			pingErr := pool.Ping(pingCtx)
			pingCancel()
			if pingErr == nil {
				break
			}
			pool.Close()
			err = pingErr
		}
		if attempt == 4 {
			logger.Fatal("failed to connect to database after retries", zap.Error(err))
		}
		backoff := time.Duration(1<<uint(attempt)) * time.Second
		logger.Warn("database connection failed, retrying",
			zap.Int("attempt", attempt+1), zap.Duration("backoff", backoff), zap.Error(err))
		time.Sleep(backoff)
	}
	defer pool.Close()
	logger.Info("connected to database")

	// Create store and handler.
	configStore := store.New(pool, logger)
	configHandler := handler.New(configStore, logger)

	// Create Fiber app.
	app := fiber.New(fiber.Config{
		AppName:               "config-service",
		DisableStartupMessage: false,
	})

	// Health check endpoint.
	app.Get("/healthz", func(c *fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status": "unhealthy",
				"error":  err.Error(),
			})
		}
		return c.JSON(fiber.Map{
			"status": "healthy",
		})
	})

	// Register config CRUD routes.
	configHandler.Register(app)

	// Graceful shutdown on SIGTERM.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		<-quit
		logger.Info("shutting down server")
		if err := app.Shutdown(); err != nil {
			logger.Error("server shutdown error", zap.Error(err))
		}
	}()

	// Start server.
	port := envOrDefault("PORT", "8083")
	logger.Info("starting config-service", zap.String("port", port))
	if err := app.Listen(":" + port); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

func envOrDefault(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
