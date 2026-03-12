package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"trading-platform/config-service/internal/handler"
	"trading-platform/config-service/internal/store"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger.
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	// Build PostgreSQL connection string from environment variables.
	dbHost := envOrDefault("DB_HOST", "localhost")
	dbPort := envOrDefault("DB_PORT", "5432")
	dbUser := envOrDefault("DB_USER", "postgres")
	dbPassword := envOrDefault("DB_PASSWORD", "postgres")
	dbName := envOrDefault("DB_NAME", "config")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName,
	)

	// Connect to PostgreSQL.
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(context.Background()); err != nil {
		logger.Fatal("failed to ping database", zap.Error(err))
	}
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
		if err := pool.Ping(c.Context()); err != nil {
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
