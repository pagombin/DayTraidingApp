package risk

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type PDTTracker struct {
	pool *pgxpool.Pool
	log  *zap.SugaredLogger
}

func NewPDTTracker(pool *pgxpool.Pool, log *zap.SugaredLogger) *PDTTracker {
	return &PDTTracker{pool: pool, log: log}
}

func (p *PDTTracker) GetDayTradeCount(ctx context.Context) (int, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	fiveDaysAgo := time.Now().AddDate(0, 0, -5)
	var count int
	err := p.pool.QueryRow(dbCtx,
		"SELECT COUNT(*) FROM day_trades WHERE trade_date >= $1",
		fiveDaysAgo).Scan(&count)
	return count, err
}

func (p *PDTTracker) RecordDayTrade(ctx context.Context, symbol string, qty int, buyPrice, sellPrice float64) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pnl := (sellPrice - buyPrice) * float64(qty)
	_, err := p.pool.Exec(dbCtx,
		"INSERT INTO day_trades (symbol, quantity, buy_price, sell_price, pnl, trade_date) VALUES ($1, $2, $3, $4, $5, $6)",
		symbol, qty, buyPrice, sellPrice, pnl, time.Now().Format("2006-01-02"))
	if err != nil {
		p.log.Warnw("Failed to record day trade", "error", err, "symbol", symbol)
	}
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
