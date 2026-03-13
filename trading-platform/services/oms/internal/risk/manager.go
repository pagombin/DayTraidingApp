package risk

import (
	"context"
	"sync"
	"time"

	"trading-platform/oms/internal/config"
	"trading-platform/oms/internal/model"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type RiskManager struct {
	cfg          *config.Config
	rdb          *redis.Client
	log          *zap.SugaredLogger
	recentOrders []recentOrder
	mu           sync.Mutex
}

type recentOrder struct {
	symbol    string
	side      model.OrderSide
	quantity  int
	timestamp time.Time
}

func NewRiskManager(cfg *config.Config, rdb *redis.Client, log *zap.SugaredLogger) *RiskManager {
	return &RiskManager{
		cfg:          cfg,
		rdb:          rdb,
		log:          log,
		recentOrders: make([]recentOrder, 0),
	}
}

func (rm *RiskManager) CheckAll(ctx context.Context, order *model.Order, portfolio *model.PortfolioSnapshot) ([]model.CheckResult, bool) {
	var checks []model.CheckResult
	allPassed := true

	riskCfg := rm.cfg.GetRisk()

	run := func(result model.CheckResult) {
		checks = append(checks, result)
		if !result.Passed {
			allPassed = false
		}
	}

	run(rm.checkDailyLoss(portfolio, riskCfg))
	run(rm.checkPositionSize(order, portfolio, riskCfg))
	run(rm.checkPortfolioRisk(order, portfolio, riskCfg))
	run(rm.checkSingleTradeLoss(order, riskCfg))
	run(rm.checkOrderRate(riskCfg))
	run(rm.checkRestrictedSymbol(order, riskCfg))
	run(rm.checkSectorConcentration(order, portfolio, riskCfg))
	run(rm.checkStopLossRequired(order, portfolio, riskCfg))
	run(rm.checkDeltaExposure(order, portfolio, riskCfg))
	run(rm.checkPDT(order, portfolio))
	run(rm.checkMarketHours())
	run(rm.checkDuplicateOrder(order))

	// Record this order for duplicate detection
	rm.mu.Lock()
	rm.recentOrders = append(rm.recentOrders, recentOrder{
		symbol:    order.Symbol,
		side:      order.Side,
		quantity:  order.Quantity,
		timestamp: time.Now(),
	})
	rm.mu.Unlock()

	return checks, allPassed
}

func (rm *RiskManager) GetRiskStatus(portfolio *model.PortfolioSnapshot) map[string]interface{} {
	riskCfg := rm.cfg.GetRisk()
	dailyUsed := decimal.Zero
	if portfolio != nil {
		dailyUsed = portfolio.DailyPnL.Abs()
	}

	dailyPct := decimal.Zero
	if !riskCfg.MaxDailyLoss.IsZero() {
		dailyPct = dailyUsed.Div(riskCfg.MaxDailyLoss).Mul(decimal.NewFromInt(100))
	}

	largestPositionPct := decimal.Zero
	if portfolio != nil && !portfolio.TotalEquity.IsZero() {
		for _, pos := range portfolio.Positions {
			pct := pos.MarketValue.Abs().Div(portfolio.TotalEquity).Mul(decimal.NewFromInt(100))
			if pct.GreaterThan(largestPositionPct) {
				largestPositionPct = pct
			}
		}
	}

	netDelta := decimal.Zero
	if portfolio != nil {
		netDelta = portfolio.NetDelta
	}

	return map[string]interface{}{
		"daily_loss_used":      dailyUsed.StringFixed(2),
		"daily_loss_limit":     riskCfg.MaxDailyLoss.StringFixed(2),
		"daily_loss_pct":       dailyPct.StringFixed(1),
		"largest_position_pct": largestPositionPct.StringFixed(1),
		"position_limit_pct":   riskCfg.MaxPositionSizePct.StringFixed(1),
		"net_delta":            netDelta.StringFixed(1),
		"delta_limit":          riskCfg.MaxDeltaExposure.StringFixed(1),
		"orders_per_minute":    rm.countRecentOrders(),
		"orders_limit":         riskCfg.MaxOrdersPerMinute,
		"circuit_breaker":      "normal",
	}
}

func (rm *RiskManager) countRecentOrders() int {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	cutoff := time.Now().Add(-60 * time.Second)
	count := 0
	for _, o := range rm.recentOrders {
		if o.timestamp.After(cutoff) {
			count++
		}
	}
	return count
}
