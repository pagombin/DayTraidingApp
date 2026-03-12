package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"trading-platform/watchdog/internal/alerter"
	"trading-platform/watchdog/internal/monitor"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to PostgreSQL
	dbHost := getEnv("DB_HOST", "postgres")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "trading")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		logger.Fatal("failed to parse database config", zap.Error(err))
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatal("failed to ping database", zap.Error(err))
	}
	logger.Info("connected to PostgreSQL")

	// Connect to Redis
	redisURL := getEnv("REDIS_URL", "redis://redis:6379/0")
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		logger.Fatal("failed to parse Redis URL", zap.Error(err))
	}

	rdb := redis.NewClient(redisOpts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Fatal("failed to connect to Redis", zap.Error(err))
	}
	defer rdb.Close()
	logger.Info("connected to Redis")

	// Initialize alerter
	alt := alerter.New(logger)

	// Initialize and start monitor
	mon := monitor.New(logger, pool, rdb, alt)
	go mon.Start(ctx)

	// Start Fiber HTTP server
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Get("/healthz", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "healthy",
			"service": "watchdog",
			"time":    time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		if err := app.Listen(":8082"); err != nil {
			logger.Fatal("failed to start HTTP server", zap.Error(err))
		}
	}()
	logger.Info("watchdog service started on :8082")

	<-quit
	logger.Info("shutting down watchdog service")
	cancel()

	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}

	logger.Info("watchdog service stopped")
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
