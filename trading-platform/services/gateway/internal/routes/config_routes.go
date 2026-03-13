package routes

import (
	"encoding/json"
	"time"

	"trading-platform/gateway/pkg/redis"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type ConfigHandler struct {
	db    *pgxpool.Pool
	redis *redis.Client
	log   *zap.SugaredLogger
}

func NewConfigHandler(db *pgxpool.Pool, redis *redis.Client, log *zap.SugaredLogger) *ConfigHandler {
	return &ConfigHandler{db: db, redis: redis, log: log}
}

type ConfigItem struct {
	Key         string           `json:"key"`
	Value       json.RawMessage  `json:"value"`
	Category    string           `json:"category"`
	Label       string           `json:"label"`
	Description *string          `json:"description"`
	ValueType   string           `json:"value_type"`
	Constraints *json.RawMessage `json:"constraints"`
	UpdatedAt   time.Time        `json:"updated_at"`
	UpdatedBy   string           `json:"updated_by"`
}

func (h *ConfigHandler) GetAll(c *fiber.Ctx) error {
	category := c.Query("category")

	ctx, cancel := dbCtx()
	defer cancel()

	var query string
	var args []interface{}
	if category != "" {
		query = "SELECT key, value, category, label, description, value_type, constraints, updated_at, updated_by FROM app_config WHERE category = $1 ORDER BY key"
		args = []interface{}{category}
	} else {
		query = "SELECT key, value, category, label, description, value_type, constraints, updated_at, updated_by FROM app_config ORDER BY category, key"
	}

	rows, err := h.db.Query(ctx, query, args...)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer rows.Close()

	grouped := map[string][]ConfigItem{}
	for rows.Next() {
		var item ConfigItem
		if err := rows.Scan(&item.Key, &item.Value, &item.Category, &item.Label, &item.Description,
			&item.ValueType, &item.Constraints, &item.UpdatedAt, &item.UpdatedBy); err != nil {
			continue
		}
		grouped[item.Category] = append(grouped[item.Category], item)
	}

	return c.JSON(fiber.Map{"config": grouped})
}

func (h *ConfigHandler) GetByKey(c *fiber.Ctx) error {
	key := c.Params("key")

	ctx, cancel := dbCtx()
	defer cancel()

	var item ConfigItem
	err := h.db.QueryRow(ctx,
		"SELECT key, value, category, label, description, value_type, constraints, updated_at, updated_by FROM app_config WHERE key = $1",
		key).Scan(&item.Key, &item.Value, &item.Category, &item.Label, &item.Description,
		&item.ValueType, &item.Constraints, &item.UpdatedAt, &item.UpdatedBy)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "config key not found"})
	}

	return c.JSON(item)
}

func (h *ConfigHandler) Update(c *fiber.Ctx) error {
	key := c.Params("key")
	username, _ := c.Locals("username").(string)
	if username == "" {
		username = "system"
	}

	var body struct {
		Value json.RawMessage `json:"value"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	ctx, cancel := dbCtx()
	defer cancel()

	// Update the config value
	result, err := h.db.Exec(ctx,
		"UPDATE app_config SET value = $1, updated_at = NOW(), updated_by = $2 WHERE key = $3",
		body.Value, username, key)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if result.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "config key not found"})
	}

	// Write audit log
	auditDetails, _ := json.Marshal(map[string]interface{}{
		"key":       key,
		"new_value": body.Value,
	})
	_, _ = h.db.Exec(ctx,
		"INSERT INTO audit_log (action, entity_type, entity_id, details, user_id) VALUES ($1, $2, $3, $4, $5)",
		"config_changed", "config", key, auditDetails, username)

	// Publish hot-reload event
	payload, _ := json.Marshal(map[string]interface{}{
		"key":   key,
		"value": body.Value,
	})
	_ = h.redis.Publish(ctx, "config:changed", string(payload)).Err()

	h.log.Infow("Config updated", "key", key, "by", username)

	return c.JSON(fiber.Map{"message": "config updated", "key": key})
}

func (h *ConfigHandler) BulkUpdate(c *fiber.Ctx) error {
	username, _ := c.Locals("username").(string)
	if username == "" {
		username = "system"
	}

	var body struct {
		Items []struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		} `json:"items"`
	}
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}

	ctx, cancel := dbCtx()
	defer cancel()

	tx, err := h.db.Begin(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to start transaction"})
	}
	defer tx.Rollback(ctx)

	updated := 0
	for _, item := range body.Items {
		result, err := tx.Exec(ctx,
			"UPDATE app_config SET value = $1, updated_at = NOW(), updated_by = $2 WHERE key = $3",
			item.Value, username, item.Key)
		if err != nil {
			continue
		}
		if result.RowsAffected() > 0 {
			updated++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to commit"})
	}

	// Publish bulk change event
	_ = h.redis.Publish(ctx, "config:changed", `{"bulk":true}`).Err()

	return c.JSON(fiber.Map{"message": "bulk update complete", "updated": updated})
}
