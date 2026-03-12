package health

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

type Handler struct {
	db        *pgxpool.Pool
	redis     *goredis.Client
	startTime time.Time
}

func NewHandler(db *pgxpool.Pool, redis *goredis.Client, startTime time.Time) *Handler {
	return &Handler{db: db, redis: redis, startTime: startTime}
}

func (h *Handler) Healthz(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "ok",
		"service":   "gateway",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"uptime":    time.Since(h.startTime).String(),
	})
}

func (h *Handler) Readyz(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	checks := map[string]string{}
	allReady := true

	if err := h.db.Ping(ctx); err != nil {
		checks["postgres"] = "unhealthy: " + err.Error()
		allReady = false
	} else {
		checks["postgres"] = "healthy"
	}

	if err := h.redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
		allReady = false
	} else {
		checks["redis"] = "healthy"
	}

	status := fiber.StatusOK
	statusText := "ready"
	if !allReady {
		status = fiber.StatusServiceUnavailable
		statusText = "not_ready"
	}

	return c.Status(status).JSON(fiber.Map{
		"status":    statusText,
		"checks":    checks,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}
