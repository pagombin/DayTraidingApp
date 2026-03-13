package risk

import (
	"fmt"
	"time"

	"trading-platform/oms/internal/config"
	"trading-platform/oms/internal/model"

	"github.com/shopspring/decimal"
)

func (rm *RiskManager) checkDailyLoss(portfolio *model.PortfolioSnapshot, cfg config.RiskConfig) model.CheckResult {
	result := model.CheckResult{Check: "daily_loss", Passed: true}

	if portfolio == nil {
		result.Reason = "Daily loss check passed (no portfolio data yet)"
		return result
	}

	dailyLoss := portfolio.DailyPnL.Neg()
	if dailyLoss.GreaterThan(cfg.MaxDailyLoss) {
		result.Passed = false
		result.Reason = fmt.Sprintf("Trading is paused — your daily loss limit of $%s has been reached. All strategies are paused until tomorrow. Your existing positions are unchanged.",
			cfg.MaxDailyLoss.StringFixed(0))
		result.Details = map[string]interface{}{
			"daily_loss": dailyLoss.StringFixed(2),
			"limit":      cfg.MaxDailyLoss.StringFixed(2),
		}
	} else {
		result.Reason = "Daily loss within limits"
	}
	return result
}

func (rm *RiskManager) checkPositionSize(order *model.Order, portfolio *model.PortfolioSnapshot, cfg config.RiskConfig) model.CheckResult {
	result := model.CheckResult{Check: "position_size", Passed: true}

	if portfolio == nil || portfolio.TotalEquity.IsZero() {
		result.Reason = "Position size check passed (no portfolio data)"
		return result
	}

	// Estimate order value using limit price or latest price
	orderPrice := decimal.NewFromInt(100) // fallback
	if order.LimitPrice != nil {
		orderPrice = *order.LimitPrice
	} else {
		// Try to get current price from existing position
		for _, pos := range portfolio.Positions {
			if pos.Symbol == order.Symbol && !pos.CurrentPrice.IsZero() {
				orderPrice = pos.CurrentPrice
				break
			}
		}
	}

	// Calculate existing position value for this symbol
	existingValue := decimal.Zero
	for _, pos := range portfolio.Positions {
		if pos.Symbol == order.Symbol {
			existingValue = pos.MarketValue.Abs()
		}
	}

	newOrderValue := orderPrice.Mul(decimal.NewFromInt(int64(order.Quantity)))
	totalPositionValue := existingValue.Add(newOrderValue)
	positionPct := totalPositionValue.Div(portfolio.TotalEquity).Mul(decimal.NewFromInt(100))

	if positionPct.GreaterThan(cfg.MaxPositionSizePct) {
		result.Passed = false
		result.Reason = fmt.Sprintf("This trade would put %s%% of your portfolio in %s. Your maximum is %s%%. Reduce the size or increase the limit in Settings.",
			positionPct.StringFixed(1), order.Symbol, cfg.MaxPositionSizePct.StringFixed(0))
		result.Details = map[string]interface{}{
			"position_pct": positionPct.StringFixed(1),
			"limit_pct":    cfg.MaxPositionSizePct.StringFixed(0),
			"symbol":       order.Symbol,
		}
	} else {
		result.Reason = "Position size within limits"
	}
	return result
}

func (rm *RiskManager) checkPortfolioRisk(order *model.Order, portfolio *model.PortfolioSnapshot, cfg config.RiskConfig) model.CheckResult {
	result := model.CheckResult{Check: "portfolio_risk", Passed: true}

	if portfolio == nil || portfolio.TotalEquity.IsZero() {
		result.Reason = "Portfolio risk check passed (no portfolio data)"
		return result
	}

	currentRiskPct := portfolio.GrossExposure.Div(portfolio.TotalEquity).Mul(decimal.NewFromInt(100))

	orderPrice := decimal.NewFromInt(100)
	if order.LimitPrice != nil {
		orderPrice = *order.LimitPrice
	}
	additionalExposure := orderPrice.Mul(decimal.NewFromInt(int64(order.Quantity)))
	newRiskPct := portfolio.GrossExposure.Add(additionalExposure).Div(portfolio.TotalEquity).Mul(decimal.NewFromInt(100))

	if newRiskPct.GreaterThan(cfg.MaxPortfolioRiskPct) {
		result.Passed = false
		result.Reason = fmt.Sprintf("Your total portfolio risk is %s%% of equity. Adding this trade would bring it to %s%%, which exceeds your %s%% limit.",
			currentRiskPct.StringFixed(1), newRiskPct.StringFixed(1), cfg.MaxPortfolioRiskPct.StringFixed(0))
		result.Details = map[string]interface{}{
			"current_risk_pct": currentRiskPct.StringFixed(1),
			"new_risk_pct":     newRiskPct.StringFixed(1),
			"limit_pct":        cfg.MaxPortfolioRiskPct.StringFixed(0),
		}
	} else {
		result.Reason = "Portfolio risk within limits"
	}
	return result
}

func (rm *RiskManager) checkSingleTradeLoss(order *model.Order, cfg config.RiskConfig) model.CheckResult {
	result := model.CheckResult{Check: "single_trade_loss", Passed: true}

	if order.StopPrice != nil && order.LimitPrice != nil {
		var maxLoss decimal.Decimal
		if order.Side == model.OrderBuy {
			maxLoss = order.LimitPrice.Sub(*order.StopPrice).Mul(decimal.NewFromInt(int64(order.Quantity)))
		} else {
			maxLoss = order.StopPrice.Sub(*order.LimitPrice).Mul(decimal.NewFromInt(int64(order.Quantity)))
		}
		if maxLoss.IsNegative() {
			maxLoss = maxLoss.Neg()
		}

		if maxLoss.GreaterThan(cfg.MaxSingleTradeLoss) {
			result.Passed = false
			result.Reason = fmt.Sprintf("If this trade hits your stop-loss, you'd lose $%s. Your single-trade limit is $%s.",
				maxLoss.StringFixed(2), cfg.MaxSingleTradeLoss.StringFixed(0))
			result.Details = map[string]interface{}{
				"max_loss": maxLoss.StringFixed(2),
				"limit":    cfg.MaxSingleTradeLoss.StringFixed(0),
			}
			return result
		}
	}

	result.Reason = "Single trade loss within limits"
	return result
}

func (rm *RiskManager) checkOrderRate(cfg config.RiskConfig) model.CheckResult {
	result := model.CheckResult{Check: "order_rate", Passed: true}

	rm.mu.Lock()
	cutoff := time.Now().Add(-60 * time.Second)
	var recent []recentOrder
	for _, o := range rm.recentOrders {
		if o.timestamp.After(cutoff) {
			recent = append(recent, o)
		}
	}
	rm.recentOrders = recent
	count := len(recent)
	rm.mu.Unlock()

	if count >= cfg.MaxOrdersPerMinute {
		result.Passed = false
		result.Reason = fmt.Sprintf("Order rate limit reached (%d orders in the last minute). This protects against runaway algorithms. Wait a moment or increase the limit in Settings.",
			cfg.MaxOrdersPerMinute)
		result.Details = map[string]interface{}{
			"count": count,
			"limit": cfg.MaxOrdersPerMinute,
		}
	} else {
		result.Reason = "Order rate within limits"
	}
	return result
}

func (rm *RiskManager) checkRestrictedSymbol(order *model.Order, cfg config.RiskConfig) model.CheckResult {
	result := model.CheckResult{Check: "restricted_symbol", Passed: true}

	if cfg.RestrictedSymbols[order.Symbol] {
		result.Passed = false
		result.Reason = fmt.Sprintf("%s is on your restricted symbols list. Remove it in Settings to trade it.", order.Symbol)
	} else {
		result.Reason = "Symbol is not restricted"
	}
	return result
}

func (rm *RiskManager) checkSectorConcentration(order *model.Order, portfolio *model.PortfolioSnapshot, cfg config.RiskConfig) model.CheckResult {
	result := model.CheckResult{Check: "sector_concentration", Passed: true}

	if portfolio == nil || portfolio.TotalEquity.IsZero() {
		result.Reason = "Sector concentration check passed (no portfolio data)"
		return result
	}

	orderSector := getSector(order.Symbol)
	if orderSector == "" {
		result.Reason = "Sector concentration check passed (unknown sector)"
		return result
	}

	sectorExposure := decimal.Zero
	for _, pos := range portfolio.Positions {
		if pos.Sector == orderSector {
			sectorExposure = sectorExposure.Add(pos.MarketValue.Abs())
		}
	}

	orderPrice := decimal.NewFromInt(100)
	if order.LimitPrice != nil {
		orderPrice = *order.LimitPrice
	}
	newExposure := sectorExposure.Add(orderPrice.Mul(decimal.NewFromInt(int64(order.Quantity))))
	newPct := newExposure.Div(portfolio.TotalEquity).Mul(decimal.NewFromInt(100))

	currentPct := sectorExposure.Div(portfolio.TotalEquity).Mul(decimal.NewFromInt(100))

	if newPct.GreaterThan(cfg.MaxSectorConcentration) {
		result.Passed = false
		result.Reason = fmt.Sprintf("You already have %s%% of your portfolio in %s. This trade would bring it to %s%%, exceeding your %s%% sector limit.",
			currentPct.StringFixed(0), orderSector, newPct.StringFixed(0), cfg.MaxSectorConcentration.StringFixed(0))
		result.Details = map[string]interface{}{
			"sector":      orderSector,
			"current_pct": currentPct.StringFixed(1),
			"new_pct":     newPct.StringFixed(1),
			"limit_pct":   cfg.MaxSectorConcentration.StringFixed(0),
		}
	} else {
		result.Reason = "Sector concentration within limits"
	}
	return result
}

func (rm *RiskManager) checkStopLossRequired(order *model.Order, portfolio *model.PortfolioSnapshot, cfg config.RiskConfig) model.CheckResult {
	result := model.CheckResult{Check: "stop_loss_required", Passed: true}

	if !cfg.RequireStopLoss {
		result.Reason = "Stop-loss requirement disabled"
		return result
	}

	// Only require stop-loss for new entry positions
	if order.Side == model.OrderBuy {
		isNewPosition := true
		if portfolio != nil {
			for _, pos := range portfolio.Positions {
				if pos.Symbol == order.Symbol && pos.Quantity > 0 {
					isNewPosition = false
					break
				}
			}
		}
		if isNewPosition && order.StopPrice == nil {
			result.Passed = false
			result.Reason = "Stop-loss is required for all new positions. Add a stop-loss price to this order or disable the requirement in Settings."
			return result
		}
	}

	result.Reason = "Stop-loss check passed"
	return result
}

func (rm *RiskManager) checkDeltaExposure(order *model.Order, portfolio *model.PortfolioSnapshot, cfg config.RiskConfig) model.CheckResult {
	result := model.CheckResult{Check: "delta_exposure", Passed: true}

	if portfolio == nil {
		result.Reason = "Delta exposure check passed (no portfolio data)"
		return result
	}

	currentDelta := portfolio.NetDelta
	orderDelta := decimal.NewFromInt(int64(order.Quantity))
	if order.Side == model.OrderSell {
		orderDelta = orderDelta.Neg()
	}
	newDelta := currentDelta.Add(orderDelta)

	if newDelta.Abs().GreaterThan(cfg.MaxDeltaExposure) {
		result.Passed = false
		result.Reason = fmt.Sprintf("Your net portfolio delta is %s. Adding this trade brings it to %s, which exceeds your limit of %s.",
			currentDelta.StringFixed(0), newDelta.StringFixed(0), cfg.MaxDeltaExposure.StringFixed(0))
		result.Details = map[string]interface{}{
			"current_delta": currentDelta.StringFixed(0),
			"new_delta":     newDelta.StringFixed(0),
			"limit":         cfg.MaxDeltaExposure.StringFixed(0),
		}
	} else {
		result.Reason = "Delta exposure within limits"
	}
	return result
}

func (rm *RiskManager) checkPDT(order *model.Order, portfolio *model.PortfolioSnapshot) model.CheckResult {
	result := model.CheckResult{Check: "pdt", Passed: true}

	if portfolio == nil {
		result.Reason = "PDT check passed (no portfolio data)"
		return result
	}

	// PDT only applies if equity < $25,000
	threshold := decimal.NewFromInt(25000)
	if portfolio.TotalEquity.GreaterThanOrEqual(threshold) {
		result.Reason = "PDT check passed (equity above $25,000)"
		return result
	}

	// Check if this would be a day trade (closing a position opened today)
	if order.Side == model.OrderSell {
		for _, pos := range portfolio.Positions {
			if pos.Symbol == order.Symbol && pos.IsLong() {
				if pos.OpenedAt.Format("2006-01-02") == time.Now().Format("2006-01-02") {
					// This would be a day trade - warn but allow (count check would need DB)
					result.Reason = "Warning: This may count as a day trade. With an account under $25,000, you are limited to 3 day trades per 5 business days."
					result.Details = map[string]interface{}{
						"equity":    portfolio.TotalEquity.StringFixed(2),
						"threshold": threshold.StringFixed(0),
					}
					return result
				}
			}
		}
	}

	result.Reason = "PDT check passed"
	return result
}

func (rm *RiskManager) checkMarketHours() model.CheckResult {
	result := model.CheckResult{Check: "market_hours", Passed: true}

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		loc = time.FixedZone("EST", -5*3600)
	}
	now := time.Now().In(loc)
	day := now.Weekday()
	minutes := now.Hour()*60 + now.Minute()

	isOpen := day >= time.Monday && day <= time.Friday && minutes >= 9*60+30 && minutes < 16*60
	isPreMarket := day >= time.Monday && day <= time.Friday && minutes >= 4*60 && minutes < 9*60+30

	if !isOpen && !isPreMarket {
		result.Passed = false
		nextOpen := "tomorrow"
		if day == time.Friday && minutes >= 16*60 || day == time.Saturday || day == time.Sunday {
			nextOpen = "Monday"
		}
		result.Reason = fmt.Sprintf("The market is currently closed. It opens %s at 9:30 AM ET. Your order will be queued and submitted at market open.", nextOpen)
	} else {
		result.Reason = "Market is open"
	}
	return result
}

func (rm *RiskManager) checkDuplicateOrder(order *model.Order) model.CheckResult {
	result := model.CheckResult{Check: "duplicate_order", Passed: true}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	cutoff := time.Now().Add(-5 * time.Second)
	for _, recent := range rm.recentOrders {
		if recent.symbol == order.Symbol && recent.side == order.Side &&
			recent.quantity == order.Quantity && recent.timestamp.After(cutoff) {
			result.Passed = false
			ago := time.Since(recent.timestamp)
			result.Reason = fmt.Sprintf("A very similar order was placed %d seconds ago. This looks like a duplicate and has been blocked.",
				int(ago.Seconds()))
			result.Details = map[string]interface{}{
				"symbol":   order.Symbol,
				"side":     string(order.Side),
				"quantity": order.Quantity,
				"seconds_ago": int(ago.Seconds()),
			}
			return result
		}
	}

	result.Reason = "No duplicate detected"
	return result
}

// getSector returns the GICS sector for a symbol (simplified lookup)
func getSector(symbol string) string {
	sectors := map[string]string{
		"AAPL": "Technology", "MSFT": "Technology", "NVDA": "Technology",
		"GOOG": "Technology", "GOOGL": "Technology", "META": "Technology",
		"AMZN": "Consumer Discretionary", "TSLA": "Consumer Discretionary",
		"JPM": "Financials", "BAC": "Financials", "GS": "Financials",
		"JNJ": "Healthcare", "UNH": "Healthcare", "PFE": "Healthcare",
		"XOM": "Energy", "CVX": "Energy",
		"PG": "Consumer Staples", "KO": "Consumer Staples",
		"SPY": "ETF", "QQQ": "ETF", "IWM": "ETF", "DIA": "ETF",
	}
	if s, ok := sectors[symbol]; ok {
		return s
	}
	return ""
}
