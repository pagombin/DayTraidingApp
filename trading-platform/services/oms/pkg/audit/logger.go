package audit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Entry struct {
	Action     string                 `json:"action"`
	EntityType string                 `json:"entity_type"`
	EntityID   string                 `json:"entity_id"`
	Details    map[string]interface{} `json:"details"`
	UserID     string                 `json:"user_id,omitempty"`
}

type Logger struct {
	pool *pgxpool.Pool
	log  *zap.SugaredLogger
}

func NewLogger(pool *pgxpool.Pool, log *zap.SugaredLogger) *Logger {
	return &Logger{pool: pool, log: log}
}

func (l *Logger) Log(ctx context.Context, entry Entry) {
	if entry.UserID == "" {
		entry.UserID = "oms"
	}

	detailsJSON, _ := json.Marshal(entry.Details)

	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := l.pool.Exec(dbCtx,
		"INSERT INTO audit_log (action, entity_type, entity_id, details, user_id) VALUES ($1, $2, $3, $4, $5)",
		entry.Action, entry.EntityType, entry.EntityID, detailsJSON, entry.UserID)
	if err != nil {
		l.log.Errorw("Failed to write audit log", "error", err, "action", entry.Action, "entity", entry.EntityID)
	}
}
