package paper

import (
	"math/rand"

	"github.com/shopspring/decimal"
)

type SlippageModel struct {
	basePct float64 // 0.0001 = 0.01%
}

func NewSlippageModel() *SlippageModel {
	return &SlippageModel{basePct: 0.0001}
}

func (s *SlippageModel) Calculate(price decimal.Decimal, quantity int) decimal.Decimal {
	base := price.Mul(decimal.NewFromFloat(s.basePct))

	// Volume-based adjustment for larger orders
	if quantity > 100 {
		extraPct := float64(quantity-100) / 10000.0 * 0.00005
		base = base.Add(price.Mul(decimal.NewFromFloat(extraPct)))
	}

	// Random variation ±20%
	variation := 0.8 + rand.Float64()*0.4
	return base.Mul(decimal.NewFromFloat(variation))
}
