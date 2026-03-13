package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// Trade represents an individual trade execution.
type Trade struct {
	Symbol    string          `json:"symbol"`
	Timestamp time.Time       `json:"timestamp"`
	Price     decimal.Decimal `json:"price"`
	Size      int64           `json:"size"`
	Exchange  string          `json:"exchange"`
	ID        string          `json:"id"`
	Source    string          `json:"source"`
}
