package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// Bar represents an OHLCV candlestick for a time period.
type Bar struct {
	Symbol     string          `json:"symbol"`
	Timestamp  time.Time       `json:"timestamp"`
	Open       decimal.Decimal `json:"open"`
	High       decimal.Decimal `json:"high"`
	Low        decimal.Decimal `json:"low"`
	Close      decimal.Decimal `json:"close"`
	Volume     int64           `json:"volume"`
	TradeCount int             `json:"trade_count"`
	VWAP       decimal.Decimal `json:"vwap"`
	Source     string          `json:"source"`
}
