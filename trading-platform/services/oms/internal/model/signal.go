package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Signal struct {
	SignalID   string           `json:"signal_id"`
	StrategyID string          `json:"strategy_id"`
	Symbol     string           `json:"symbol"`
	Side       OrderSide        `json:"side"`
	OrderType  OrderType        `json:"order_type"`
	Quantity   int              `json:"quantity"`
	LimitPrice *decimal.Decimal `json:"limit_price,omitempty"`
	StopPrice  *decimal.Decimal `json:"stop_price,omitempty"`
	Urgency    string           `json:"urgency,omitempty"`
	Reason     string           `json:"reason,omitempty"`
	Timestamp  time.Time        `json:"timestamp"`
}
