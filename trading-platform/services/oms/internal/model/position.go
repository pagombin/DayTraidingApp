package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Position struct {
	PositionID    string          `json:"position_id" db:"position_id"`
	AccountID     string          `json:"account_id" db:"account_id"`
	StrategyID    string          `json:"strategy_id" db:"strategy_id"`
	Symbol        string          `json:"symbol" db:"symbol"`
	Quantity      int             `json:"quantity" db:"quantity"`
	AvgCost       decimal.Decimal `json:"avg_cost" db:"avg_cost"`
	CurrentPrice  decimal.Decimal `json:"current_price"`
	MarketValue   decimal.Decimal `json:"market_value"`
	UnrealizedPnL decimal.Decimal `json:"unrealized_pnl"`
	WeightPct     decimal.Decimal `json:"weight_pct"`
	Sector        string          `json:"sector,omitempty"`
	IsOption      bool            `json:"is_option" db:"is_option"`
	Expiration    *time.Time      `json:"expiration,omitempty" db:"expiration"`
	Strike        *decimal.Decimal `json:"strike,omitempty" db:"strike"`
	OptionType    string          `json:"option_type,omitempty" db:"option_type"`
	Delta         *decimal.Decimal `json:"delta,omitempty"`
	Gamma         *decimal.Decimal `json:"gamma,omitempty"`
	Theta         *decimal.Decimal `json:"theta,omitempty"`
	Vega          *decimal.Decimal `json:"vega,omitempty"`
	OpenedAt      time.Time       `json:"opened_at" db:"opened_at"`
}

func (p Position) IsLong() bool  { return p.Quantity > 0 }
func (p Position) IsShort() bool { return p.Quantity < 0 }
