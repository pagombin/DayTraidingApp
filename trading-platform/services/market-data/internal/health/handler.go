package health

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

type Handler struct {
	pool      *pgxpool.Pool
	redis     *redis.Client
	startTime time.Time
	isReady   func() bool
}

func NewHandler(pool *pgxpool.Pool, rdb *redis.Client, startTime time.Time, isReady func() bool) *Handler {
	return &Handler{
		pool:      pool,
		redis:     rdb,
		startTime: startTime,
		isReady:   isReady,
	}
}

// Healthz is the liveness probe — returns 200 if the process is running.
func (h *Handler) Healthz(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status": "ok",
		"uptime": time.Since(h.startTime).Truncate(time.Second).String(),
	})
}

// Readyz is the readiness probe — checks all dependencies.
func (h *Handler) Readyz(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(c.Context(), 3*time.Second)
	defer cancel()

	checks := fiber.Map{}
	allOk := true

	// Check PostgreSQL
	if err := h.pool.Ping(ctx); err != nil {
		checks["postgres"] = "unhealthy: " + err.Error()
		allOk = false
	} else {
		checks["postgres"] = "ok"
	}

	// Check Redis
	if err := h.redis.Ping(ctx).Err(); err != nil {
		checks["redis"] = "unhealthy: " + err.Error()
		allOk = false
	} else {
		checks["redis"] = "ok"
	}

	// Check provider connection
	if h.isReady != nil && !h.isReady() {
		checks["provider"] = "not connected"
		allOk = false
	} else {
		checks["provider"] = "connected"
	}

	status := fiber.StatusOK
	if !allOk {
		status = fiber.StatusServiceUnavailable
	}

	return c.Status(status).JSON(fiber.Map{
		"status": map[bool]string{true: "ready", false: "not_ready"}[allOk],
		"checks": checks,
		"uptime": time.Since(h.startTime).Truncate(time.Second).String(),
	})
}
