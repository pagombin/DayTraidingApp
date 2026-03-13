package sizing

import (
	"trading-platform/oms/internal/config"
	"trading-platform/oms/internal/model"

	"github.com/shopspring/decimal"
)

type PositionSizer struct {
	cfg *config.Config
}

func NewPositionSizer(cfg *config.Config) *PositionSizer {
	return &PositionSizer{cfg: cfg}
}

func (ps *PositionSizer) CalculateSize(order *model.Order, portfolio *model.PortfolioSnapshot) int {
	if portfolio == nil || portfolio.TotalEquity.IsZero() {
		return order.Quantity
	}

	riskCfg := ps.cfg.GetRisk()

	maxPositionValue := portfolio.TotalEquity.Mul(riskCfg.MaxPositionSizePct).Div(decimal.NewFromInt(100))

	orderPrice := decimal.NewFromInt(100)
	if order.LimitPrice != nil {
		orderPrice = *order.LimitPrice
	}

	if orderPrice.IsZero() {
		return order.Quantity
	}

	maxShares := maxPositionValue.Div(orderPrice).IntPart()
	if int(maxShares) < order.Quantity {
		return int(maxShares)
	}
	return order.Quantity
}
