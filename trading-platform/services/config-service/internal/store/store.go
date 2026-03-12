package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// ConfigItem represents a single configuration entry.
type ConfigItem struct {
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	Category  string    `json:"category"`
	UpdatedBy string    `json:"updated_by"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConfigStore provides CRUD operations for configuration items backed by PostgreSQL.
type ConfigStore struct {
	pool   *pgxpool.Pool
	logger *zap.Logger
}

// New creates a new ConfigStore with the given connection pool and logger.
func New(pool *pgxpool.Pool, logger *zap.Logger) *ConfigStore {
	return &ConfigStore{
		pool:   pool,
		logger: logger,
	}
}

// Get retrieves a single configuration item by key.
func (s *ConfigStore) Get(ctx context.Context, key string) (*ConfigItem, error) {
	item := &ConfigItem{}
	err := s.pool.QueryRow(ctx,
		`SELECT key, value, category, updated_by, updated_at
		 FROM config_items
		 WHERE key = $1`, key,
	).Scan(&item.Key, &item.Value, &item.Category, &item.UpdatedBy, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get config %q: %w", key, err)
	}
	return item, nil
}

// GetAll retrieves all configuration items, optionally filtered by category.
// If category is empty, all items are returned.
func (s *ConfigStore) GetAll(ctx context.Context, category string) ([]ConfigItem, error) {
	var query string
	var args []interface{}

	if category != "" {
		query = `SELECT key, value, category, updated_by, updated_at
		         FROM config_items
		         WHERE category = $1
		         ORDER BY category, key`
		args = append(args, category)
	} else {
		query = `SELECT key, value, category, updated_by, updated_at
		         FROM config_items
		         ORDER BY category, key`
	}

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("get all config: %w", err)
	}
	defer rows.Close()

	var items []ConfigItem
	for rows.Next() {
		var item ConfigItem
		if err := rows.Scan(&item.Key, &item.Value, &item.Category, &item.UpdatedBy, &item.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan config item: %w", err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate config items: %w", err)
	}

	return items, nil
}

// Update updates a configuration item's value and records the change in the audit log.
// It returns the updated item.
func (s *ConfigStore) Update(ctx context.Context, key, value, updatedBy string) (*ConfigItem, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Fetch the old value for the audit log.
	var oldValue string
	err = tx.QueryRow(ctx,
		`SELECT value FROM config_items WHERE key = $1`, key,
	).Scan(&oldValue)
	if err != nil {
		return nil, fmt.Errorf("get old value for %q: %w", key, err)
	}

	// Update the config item.
	item := &ConfigItem{}
	err = tx.QueryRow(ctx,
		`UPDATE config_items
		 SET value = $1, updated_by = $2, updated_at = NOW()
		 WHERE key = $3
		 RETURNING key, value, category, updated_by, updated_at`,
		value, updatedBy, key,
	).Scan(&item.Key, &item.Value, &item.Category, &item.UpdatedBy, &item.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("update config %q: %w", key, err)
	}

	// Write to audit log.
	_, err = tx.Exec(ctx,
		`INSERT INTO audit_log (config_key, old_value, new_value, changed_by, changed_at)
		 VALUES ($1, $2, $3, $4, NOW())`,
		key, oldValue, value, updatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("write audit log for %q: %w", key, err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	s.logger.Info("config updated",
		zap.String("key", key),
		zap.String("updated_by", updatedBy),
	)

	return item, nil
}
