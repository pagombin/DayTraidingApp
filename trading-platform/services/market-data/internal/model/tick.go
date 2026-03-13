package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// MarketTick represents a normalized Level 1 quote for a stock/ETF.
type MarketTick struct {
	Symbol    string          `json:"symbol"`
	Timestamp time.Time       `json:"timestamp"`
	Bid       decimal.Decimal `json:"bid"`
	Ask       decimal.Decimal `json:"ask"`
	Last      decimal.Decimal `json:"last"`
	Volume    int64           `json:"volume"`
	Source    string          `json:"source"`
}

// Spread returns the bid-ask spread.
func (t MarketTick) Spread() decimal.Decimal {
	return t.Ask.Sub(t.Bid)
}

// MidPrice returns the midpoint between bid and ask.
func (t MarketTick) MidPrice() decimal.Decimal {
	return t.Bid.Add(t.Ask).Div(decimal.NewFromInt(2))
}
