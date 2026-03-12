package handler

import (
	"trading-platform/config-service/internal/store"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// Handler holds dependencies for HTTP route handlers.
type Handler struct {
	store  *store.ConfigStore
	logger *zap.Logger
}

// New creates a new Handler with the given store and logger.
func New(s *store.ConfigStore, logger *zap.Logger) *Handler {
	return &Handler{
		store:  s,
		logger: logger,
	}
}

// Register attaches all config routes to the Fiber app.
func (h *Handler) Register(app *fiber.App) {
	api := app.Group("/api/config")
	api.Get("/", h.ListConfig)
	api.Get("/:key", h.GetConfig)
	api.Put("/:key", h.UpdateConfig)
}

// ListConfig handles GET /api/config and returns all config items grouped by category.
func (h *Handler) ListConfig(c *fiber.Ctx) error {
	category := c.Query("category")

	items, err := h.store.GetAll(c.Context(), category)
	if err != nil {
		h.logger.Error("failed to list config", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to list configuration",
		})
	}

	// Group items by category.
	grouped := make(map[string][]store.ConfigItem)
	for _, item := range items {
		grouped[item.Category] = append(grouped[item.Category], item)
	}

	return c.JSON(fiber.Map{
		"config": grouped,
	})
}

// GetConfig handles GET /api/config/:key and returns a single config item.
func (h *Handler) GetConfig(c *fiber.Ctx) error {
	key := c.Params("key")

	item, err := h.store.Get(c.Context(), key)
	if err != nil {
		h.logger.Error("failed to get config", zap.String("key", key), zap.Error(err))
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "configuration key not found",
		})
	}

	return c.JSON(item)
}

// updateRequest is the expected body for PUT /api/config/:key.
type updateRequest struct {
	Value     string `json:"value"`
	UpdatedBy string `json:"updated_by"`
}

// UpdateConfig handles PUT /api/config/:key and updates a config value.
func (h *Handler) UpdateConfig(c *fiber.Ctx) error {
	key := c.Params("key")

	var req updateRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid request body",
		})
	}

	if req.Value == "" || req.UpdatedBy == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "value and updated_by are required",
		})
	}

	item, err := h.store.Update(c.Context(), key, req.Value, req.UpdatedBy)
	if err != nil {
		h.logger.Error("failed to update config", zap.String("key", key), zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to update configuration",
		})
	}

	return c.JSON(item)
}
