package portfolio

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"trading-platform/oms/internal/model"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Publisher struct {
	rdb          *redis.Client
	log          *zap.SugaredLogger
	lastPublish  time.Time
	mu           sync.Mutex
}

func NewPublisher(rdb *redis.Client, log *zap.SugaredLogger) *Publisher {
	return &Publisher{rdb: rdb, log: log}
}

func (p *Publisher) PublishSnapshot(ctx context.Context, snapshot *model.PortfolioSnapshot) {
	p.mu.Lock()
	if time.Since(p.lastPublish) < 500*time.Millisecond {
		p.mu.Unlock()
		return // Throttle to max 2/second
	}
	p.lastPublish = time.Now()
	p.mu.Unlock()

	data, err := json.Marshal(map[string]interface{}{
		"type":              "portfolio_update",
		"total_equity":      snapshot.TotalEquity.StringFixed(2),
		"cash":              snapshot.Cash.StringFixed(2),
		"buying_power":      snapshot.BuyingPower.StringFixed(2),
		"unrealized_pnl":    snapshot.UnrealizedPnL.StringFixed(2),
		"realized_pnl_today": snapshot.RealizedPnLToday.StringFixed(2),
		"daily_pnl":         snapshot.DailyPnL.StringFixed(2),
		"position_count":    snapshot.PositionCount,
		"net_delta":         snapshot.NetDelta.StringFixed(1),
		"long_exposure":     snapshot.LongExposure.StringFixed(2),
		"short_exposure":    snapshot.ShortExposure.StringFixed(2),
	})
	if err != nil {
		return
	}

	// Publish to Redis for WebSocket relay and store as cache
	p.rdb.Publish(ctx, "portfolio:snapshot", string(data))
	p.rdb.Set(ctx, "portfolio:latest", string(data), 0)
}
