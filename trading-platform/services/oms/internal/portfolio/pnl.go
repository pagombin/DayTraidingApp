package portfolio

import (
	"trading-platform/oms/internal/model"

	"github.com/shopspring/decimal"
)

func CalculatePositionPnL(pos *model.Position) (unrealized decimal.Decimal) {
	if pos.CurrentPrice.IsZero() {
		return decimal.Zero
	}
	costBasis := pos.AvgCost.Mul(decimal.NewFromInt(int64(pos.Quantity)))
	marketValue := pos.CurrentPrice.Mul(decimal.NewFromInt(int64(pos.Quantity)))
	return marketValue.Sub(costBasis)
}

func CalculateRealizedPnL(avgCost, exitPrice decimal.Decimal, quantity int) decimal.Decimal {
	return exitPrice.Sub(avgCost).Mul(decimal.NewFromInt(int64(quantity)))
}
