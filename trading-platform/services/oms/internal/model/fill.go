package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Fill struct {
	FillID        string          `json:"fill_id"`
	OrderID       string          `json:"order_id"`
	BrokerOrderID string          `json:"broker_order_id"`
	Symbol        string          `json:"symbol"`
	Side          OrderSide       `json:"side"`
	FilledQty     int             `json:"filled_qty"`
	FilledPrice   decimal.Decimal `json:"filled_price"`
	Commission    decimal.Decimal `json:"commission"`
	Timestamp     time.Time       `json:"timestamp"`
}
