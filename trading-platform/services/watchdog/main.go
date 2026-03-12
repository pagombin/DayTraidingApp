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

	// Connect to PostgreSQL with pool configuration and retry
	pool, err := connectDB(ctx, logger)
	if err != nil {
		logger.Fatal("failed to connect to database after retries", zap.Error(err))
	}
	defer pool.Close()
	logger.Info("connected to PostgreSQL")

	// Connect to Redis with pool configuration and retry
	rdb, err := connectRedis(ctx, logger)
	if err != nil {
		logger.Fatal("failed to connect to Redis after retries", zap.Error(err))
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

	port := getEnv("PORT", "8082")
	go func() {
		if err := app.Listen(":" + port); err != nil {
			logger.Fatal("failed to start HTTP server", zap.Error(err))
		}
	}()
	logger.Info("watchdog service started", zap.String("port", port))

	<-quit
	logger.Info("shutting down watchdog service")
	cancel()

	if err := app.ShutdownWithTimeout(10 * time.Second); err != nil {
		logger.Error("server shutdown error", zap.Error(err))
	}

	logger.Info("watchdog service stopped")
}

func connectDB(ctx context.Context, logger *zap.Logger) (*pgxpool.Pool, error) {
	dbHost := getEnv("DB_HOST", "postgres")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "trading")
	dbPassword := getEnv("DB_PASSWORD", "changeme_in_production")
	dbName := getEnv("DB_NAME", "trading_platform")

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	poolConfig.MaxConns = 10
	poolConfig.MinConns = 2
	poolConfig.MaxConnLifetime = 30 * time.Minute
	poolConfig.MaxConnIdleTime = 5 * time.Minute
	poolConfig.HealthCheckPeriod = 30 * time.Second

	// Retry connection with exponential backoff
	var pool *pgxpool.Pool
	for attempt := 0; attempt < 5; attempt++ {
		connectCtx, connectCancel := context.WithTimeout(ctx, 10*time.Second)
		pool, err = pgxpool.NewWithConfig(connectCtx, poolConfig)
		connectCancel()
		if err == nil {
			pingCtx, pingCancel := context.WithTimeout(ctx, 5*time.Second)
			pingErr := pool.Ping(pingCtx)
			pingCancel()
			if pingErr == nil {
				return pool, nil
			}
			pool.Close()
			err = pingErr
		}
		backoff := time.Duration(1<<uint(attempt)) * time.Second
		logger.Warn("database connection failed, retrying",
			zap.Int("attempt", attempt+1), zap.Duration("backoff", backoff), zap.Error(err))
		time.Sleep(backoff)
	}
	return nil, fmt.Errorf("exhausted retries: %w", err)
}

func connectRedis(ctx context.Context, logger *zap.Logger) (*redis.Client, error) {
	redisURL := getEnv("REDIS_URL", "redis://redis:6379/0")
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Redis URL: %w", err)
	}

	redisOpts.PoolSize = 10
	redisOpts.MinIdleConns = 3
	redisOpts.ReadTimeout = 5 * time.Second
	redisOpts.WriteTimeout = 5 * time.Second
	redisOpts.DialTimeout = 5 * time.Second

	rdb := redis.NewClient(redisOpts)

	// Retry connection with exponential backoff
	for attempt := 0; attempt < 5; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = rdb.Ping(pingCtx).Err()
		cancel()
		if err == nil {
			return rdb, nil
		}
		backoff := time.Duration(1<<uint(attempt)) * time.Second
		logger.Warn("Redis connection failed, retrying",
			zap.Int("attempt", attempt+1), zap.Duration("backoff", backoff), zap.Error(err))
		time.Sleep(backoff)
	}
	return nil, fmt.Errorf("exhausted retries: %w", err)
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		return val
	}
	return fallback
}
