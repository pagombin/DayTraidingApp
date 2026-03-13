package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type OrderState string

const (
	OrderPending          OrderState = "PENDING"
	OrderApproved         OrderState = "APPROVED"
	OrderAwaitingApproval OrderState = "AWAITING_APPROVAL"
	OrderSubmitted        OrderState = "SUBMITTED"
	OrderAccepted         OrderState = "ACCEPTED"
	OrderPartialFill      OrderState = "PARTIAL_FILL"
	OrderFilled           OrderState = "FILLED"
	OrderCancelled        OrderState = "CANCELLED"
	OrderRejected         OrderState = "REJECTED"
	OrderExpired          OrderState = "EXPIRED"
	OrderFailed           OrderState = "FAILED"
)

func (s OrderState) IsTerminal() bool {
	return s == OrderFilled || s == OrderCancelled || s == OrderRejected ||
		s == OrderExpired || s == OrderFailed
}

type OrderType string

const (
	OrderTypeMarket       OrderType = "market"
	OrderTypeLimit        OrderType = "limit"
	OrderTypeStop         OrderType = "stop"
	OrderTypeStopLimit    OrderType = "stop_limit"
	OrderTypeTrailingStop OrderType = "trailing_stop"
)

type OrderSide string

const (
	OrderBuy  OrderSide = "buy"
	OrderSell OrderSide = "sell"
)

type TimeInForce string

const (
	TIFDay TimeInForce = "day"
	TIFGTC TimeInForce = "gtc"
	TIFIOC TimeInForce = "ioc"
	TIFFOK TimeInForce = "fok"
)

type Order struct {
	OrderID       uuid.UUID        `json:"order_id" db:"order_id"`
	BrokerOrderID string           `json:"broker_order_id,omitempty" db:"broker_order_id"`
	StrategyID    string           `json:"strategy_id" db:"strategy_id"`
	AccountID     string           `json:"account_id" db:"account_id"`
	Symbol        string           `json:"symbol" db:"symbol"`
	Side          OrderSide        `json:"side" db:"side"`
	OrderType     OrderType        `json:"order_type" db:"order_type"`
	Quantity      int              `json:"quantity" db:"quantity"`
	LimitPrice    *decimal.Decimal `json:"limit_price,omitempty" db:"limit_price"`
	StopPrice     *decimal.Decimal `json:"stop_price,omitempty" db:"stop_price"`
	TrailPercent  *decimal.Decimal `json:"trail_percent,omitempty" db:"trail_percent"`
	TimeInForce   TimeInForce      `json:"time_in_force" db:"time_in_force"`
	Legs          []OptionLeg      `json:"legs,omitempty"`
	State         OrderState       `json:"state" db:"status"`
	FilledQty     int              `json:"filled_qty" db:"filled_quantity"`
	AvgFillPrice  *decimal.Decimal `json:"avg_fill_price,omitempty" db:"avg_fill_price"`
	Commission    decimal.Decimal  `json:"commission" db:"commission"`
	SignalID      string           `json:"signal_id,omitempty" db:"signal_id"`
	RejectionReason string         `json:"rejection_reason,omitempty" db:"rejection_reason"`
	Metadata      map[string]interface{} `json:"metadata,omitempty" db:"metadata"`
	CreatedAt     time.Time        `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at" db:"updated_at"`
	SubmittedAt   *time.Time       `json:"submitted_at,omitempty" db:"submitted_at"`
	FilledAt      *time.Time       `json:"filled_at,omitempty" db:"filled_at"`
}

type OptionLeg struct {
	ContractSymbol string           `json:"contract_symbol"`
	Side           OrderSide        `json:"side"`
	Quantity       int              `json:"quantity"`
	LimitPrice     *decimal.Decimal `json:"limit_price,omitempty"`
}
