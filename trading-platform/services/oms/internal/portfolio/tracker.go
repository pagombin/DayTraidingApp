package portfolio

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"trading-platform/oms/internal/broker"
	"trading-platform/oms/internal/model"
	"trading-platform/oms/internal/risk"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type PortfolioTracker struct {
	positions      map[string]*model.Position
	account        broker.AccountInfo
	dailyPnL       decimal.Decimal
	realizedToday  decimal.Decimal
	startOfDayEquity decimal.Decimal
	mu             sync.RWMutex
	pool           *pgxpool.Pool
	rdb            *redis.Client
	brokerAdapter  broker.BrokerAdapter
	circuitBreaker *risk.CircuitBreaker
	publisher      *Publisher
	log            *zap.SugaredLogger
}

func NewPortfolioTracker(pool *pgxpool.Pool, rdb *redis.Client, ba broker.BrokerAdapter, cb *risk.CircuitBreaker, log *zap.SugaredLogger) *PortfolioTracker {
	pt := &PortfolioTracker{
		positions:     make(map[string]*model.Position),
		pool:          pool,
		rdb:           rdb,
		brokerAdapter: ba,
		circuitBreaker: cb,
		log:           log,
	}
	pt.publisher = NewPublisher(rdb, log)
	return pt
}

func (pt *PortfolioTracker) ReconcileWithBroker(ctx context.Context) error {
	pt.log.Info("Reconciling with broker...")

	acc, err := pt.brokerAdapter.GetAccount(ctx)
	if err != nil {
		pt.log.Warnw("Failed to get broker account for reconciliation", "error", err)
		// Continue with defaults for paper mode
		pt.mu.Lock()
		pt.account = broker.AccountInfo{
			Equity:      decimal.NewFromInt(100000),
			Cash:        decimal.NewFromInt(100000),
			BuyingPower: decimal.NewFromInt(200000),
		}
		pt.startOfDayEquity = pt.account.Equity
		pt.mu.Unlock()
		return nil
	}

	pt.mu.Lock()
	pt.account = *acc
	pt.startOfDayEquity = acc.Equity
	pt.mu.Unlock()

	positions, err := pt.brokerAdapter.GetPositions(ctx)
	if err != nil {
		pt.log.Warnw("Failed to get broker positions", "error", err)
		return nil
	}

	pt.mu.Lock()
	pt.positions = make(map[string]*model.Position)
	for _, pos := range positions {
		pt.positions[pos.Symbol] = pos
	}
	pt.mu.Unlock()

	pt.log.Infow("Reconciliation complete", "equity", acc.Equity.StringFixed(2), "positions", len(positions))
	return nil
}

func (pt *PortfolioTracker) GetSnapshot() model.PortfolioSnapshot {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	snapshot := model.PortfolioSnapshot{
		Timestamp:        time.Now(),
		AccountID:        "default",
		TotalEquity:      pt.account.Equity,
		Cash:             pt.account.Cash,
		BuyingPower:      pt.account.BuyingPower,
		RealizedPnLToday: pt.realizedToday,
		PositionCount:    len(pt.positions),
	}

	var unrealizedPnL, longExposure, shortExposure, netDelta decimal.Decimal

	for _, pos := range pt.positions {
		snapshot.Positions = append(snapshot.Positions, *pos)
		unrealizedPnL = unrealizedPnL.Add(pos.UnrealizedPnL)

		mv := pos.MarketValue
		if pos.Quantity > 0 {
			longExposure = longExposure.Add(mv.Abs())
			netDelta = netDelta.Add(decimal.NewFromInt(int64(pos.Quantity)))
		} else {
			shortExposure = shortExposure.Add(mv.Abs())
			netDelta = netDelta.Add(decimal.NewFromInt(int64(pos.Quantity)))
		}

		if pos.Delta != nil {
			netDelta = netDelta.Add(*pos.Delta)
		}
	}

	snapshot.UnrealizedPnL = unrealizedPnL
	snapshot.DailyPnL = unrealizedPnL.Add(pt.realizedToday)
	snapshot.LongExposure = longExposure
	snapshot.ShortExposure = shortExposure
	snapshot.NetExposure = longExposure.Sub(shortExposure)
	snapshot.GrossExposure = longExposure.Add(shortExposure)
	snapshot.NetDelta = netDelta

	return snapshot
}

func (pt *PortfolioTracker) OnTick(symbol string, price decimal.Decimal) {
	pt.mu.Lock()
	pos, exists := pt.positions[symbol]
	if !exists {
		pt.mu.Unlock()
		return
	}

	pos.CurrentPrice = price
	pos.MarketValue = price.Mul(decimal.NewFromInt(int64(pos.Quantity)))
	pos.UnrealizedPnL = pos.MarketValue.Sub(pos.AvgCost.Mul(decimal.NewFromInt(int64(pos.Quantity))))

	if !pt.account.Equity.IsZero() {
		pos.WeightPct = pos.MarketValue.Abs().Div(pt.account.Equity).Mul(decimal.NewFromInt(100))
	}
	pt.mu.Unlock()

	// Publish update (throttled by publisher)
	snapshot := pt.GetSnapshot()
	pt.publisher.PublishSnapshot(context.Background(), &snapshot)

	// Check circuit breaker
	pt.circuitBreaker.Evaluate(&snapshot)
}

func (pt *PortfolioTracker) StartTickSubscription(ctx context.Context) {
	go func() {
		sub := pt.rdb.PSubscribe(ctx, "market:ticks:*")
		ch := sub.Channel()
		defer sub.Close()

		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-ch:
				var tick struct {
					Symbol string `json:"symbol"`
					Last   string `json:"last"`
				}
				if err := json.Unmarshal([]byte(msg.Payload), &tick); err != nil {
					continue
				}
				if tick.Symbol == "" || tick.Last == "" {
					continue
				}
				price, err := decimal.NewFromString(tick.Last)
				if err != nil {
					continue
				}
				pt.OnTick(tick.Symbol, price)
			}
		}
	}()
}

func (pt *PortfolioTracker) SaveDailySnapshot(ctx context.Context) error {
	snapshot := pt.GetSnapshot()
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := pt.pool.Exec(dbCtx,
		`INSERT INTO portfolio_snapshots (account_id, total_equity, cash, buying_power, unrealized_pnl, realized_pnl, daily_pnl, position_count, long_exposure, short_exposure)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		snapshot.AccountID, snapshot.TotalEquity, snapshot.Cash, snapshot.BuyingPower,
		snapshot.UnrealizedPnL, snapshot.RealizedPnLToday, snapshot.DailyPnL,
		snapshot.PositionCount, snapshot.LongExposure, snapshot.ShortExposure)
	return err
}

func (pt *PortfolioTracker) GetPortfolioHistory(ctx context.Context, days int) ([]map[string]interface{}, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := pt.pool.Query(dbCtx,
		`SELECT total_equity, cash, daily_pnl, position_count, snapshot_at
		 FROM portfolio_snapshots WHERE account_id = 'default'
		 ORDER BY snapshot_at DESC LIMIT $1`, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var equity, cash, dailyPnL decimal.Decimal
		var posCount int
		var snapshotAt time.Time
		if err := rows.Scan(&equity, &cash, &dailyPnL, &posCount, &snapshotAt); err != nil {
			continue
		}
		history = append(history, map[string]interface{}{
			"total_equity":   equity.StringFixed(2),
			"cash":           cash.StringFixed(2),
			"daily_pnl":      dailyPnL.StringFixed(2),
			"position_count": posCount,
			"date":           snapshotAt.Format("2006-01-02"),
		})
	}
	return history, nil
}

func (pt *PortfolioTracker) GetPnLBreakdown(ctx context.Context) ([]map[string]interface{}, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := pt.pool.Query(dbCtx,
		`SELECT strategy_id, symbol, SUM(realized_pnl) as total_pnl, COUNT(*) as trade_count
		 FROM trade_journal WHERE closed_at >= CURRENT_DATE
		 GROUP BY strategy_id, symbol ORDER BY total_pnl DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var breakdown []map[string]interface{}
	for rows.Next() {
		var strategyID, symbol string
		var totalPnL decimal.Decimal
		var tradeCount int
		if err := rows.Scan(&strategyID, &symbol, &totalPnL, &tradeCount); err != nil {
			continue
		}
		breakdown = append(breakdown, map[string]interface{}{
			"strategy_id": strategyID,
			"symbol":      symbol,
			"total_pnl":   totalPnL.StringFixed(2),
			"trade_count": tradeCount,
		})
	}

	if breakdown == nil {
		breakdown = []map[string]interface{}{}
	}

	return breakdown, nil
}

func (pt *PortfolioTracker) GetPositionsForAPI() []map[string]interface{} {
	pt.mu.RLock()
	defer pt.mu.RUnlock()

	var positions []map[string]interface{}
	for _, pos := range pt.positions {
		p := map[string]interface{}{
			"position_id":   pos.PositionID,
			"symbol":        pos.Symbol,
			"quantity":      pos.Quantity,
			"avg_cost":      pos.AvgCost.StringFixed(2),
			"current_price": pos.CurrentPrice.StringFixed(2),
			"market_value":  pos.MarketValue.StringFixed(2),
			"unrealized_pnl": pos.UnrealizedPnL.StringFixed(2),
			"weight_pct":    pos.WeightPct.StringFixed(1),
			"sector":        pos.Sector,
			"strategy_id":   pos.StrategyID,
			"side":          "long",
		}
		if pos.Quantity < 0 {
			p["side"] = "short"
		}
		positions = append(positions, p)
	}

	if positions == nil {
		positions = []map[string]interface{}{}
	}
	return positions
}

// LoadPositionsFromDB restores position state from DB on startup
func (pt *PortfolioTracker) LoadPositionsFromDB(ctx context.Context) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := pt.pool.Query(dbCtx,
		`SELECT position_id, account_id, strategy_id, symbol, quantity, avg_cost,
		 COALESCE(current_price, avg_cost), opened_at
		 FROM positions WHERE account_id = 'default'`)
	if err != nil {
		return err
	}
	defer rows.Close()

	pt.mu.Lock()
	defer pt.mu.Unlock()

	for rows.Next() {
		pos := &model.Position{}
		if err := rows.Scan(&pos.PositionID, &pos.AccountID, &pos.StrategyID,
			&pos.Symbol, &pos.Quantity, &pos.AvgCost, &pos.CurrentPrice, &pos.OpenedAt); err != nil {
			continue
		}
		pos.MarketValue = pos.CurrentPrice.Mul(decimal.NewFromInt(int64(pos.Quantity)))
		pos.UnrealizedPnL = pos.MarketValue.Sub(pos.AvgCost.Mul(decimal.NewFromInt(int64(pos.Quantity))))
		pt.positions[pos.Symbol] = pos
	}

	pt.log.Infow("Loaded positions from DB", "count", len(pt.positions))
	return nil
}

func (pt *PortfolioTracker) GetRiskChecksForOrder(ctx context.Context, orderID string) ([]map[string]interface{}, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := pt.pool.Query(dbCtx,
		"SELECT check_name, passed, reason, details FROM risk_check_results WHERE order_id = $1 ORDER BY id",
		orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var checkName, reason string
		var passed bool
		var details json.RawMessage

		if err := rows.Scan(&checkName, &passed, &reason, &details); err != nil {
			continue
		}

		result := map[string]interface{}{
			"check":  checkName,
			"passed": passed,
			"reason": reason,
		}
		if details != nil {
			var d map[string]interface{}
			if json.Unmarshal(details, &d) == nil {
				result["details"] = d
			}
		}
		results = append(results, result)
	}

	if results == nil {
		results = []map[string]interface{}{}
	}
	return results, nil
}

// Stub: provides latest price for a position via Redis lookup
func (pt *PortfolioTracker) getLatestPrice(ctx context.Context, symbol string) decimal.Decimal {
	key := fmt.Sprintf("market:latest:%s", symbol)
	val, err := pt.rdb.Get(ctx, key).Result()
	if err != nil {
		return decimal.Zero
	}
	var quote struct {
		Last string `json:"last"`
	}
	if json.Unmarshal([]byte(val), &quote) == nil {
		p, _ := decimal.NewFromString(quote.Last)
		return p
	}
	return decimal.Zero
}
