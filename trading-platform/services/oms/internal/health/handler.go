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
	rdb       *redis.Client
	startTime time.Time
}

func NewHandler(pool *pgxpool.Pool, rdb *redis.Client, startTime time.Time) *Handler {
	return &Handler{pool: pool, rdb: rdb, startTime: startTime}
}

func (h *Handler) Healthz(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"service": "oms",
		"uptime":  time.Since(h.startTime).String(),
	})
}

func (h *Handler) Readyz(c *fiber.Ctx) error {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := h.pool.Ping(ctx); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not ready",
			"reason": "database connection failed",
		})
	}

	if err := h.rdb.Ping(ctx).Err(); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"status": "not ready",
			"reason": "redis connection failed",
		})
	}

	return c.JSON(fiber.Map{"status": "ready"})
}
