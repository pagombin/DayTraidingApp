package signal

import (
	"trading-platform/oms/internal/model"

	"github.com/shopspring/decimal"
)

type ManualOrderRequest struct {
	Symbol      string  `json:"symbol"`
	Side        string  `json:"side"`
	OrderType   string  `json:"order_type"`
	Quantity    int     `json:"quantity"`
	LimitPrice  *string `json:"limit_price,omitempty"`
	StopPrice   *string `json:"stop_price,omitempty"`
	TimeInForce string  `json:"time_in_force,omitempty"`
}

func (r *ManualOrderRequest) ToSignal() *model.Signal {
	sig := &model.Signal{
		SignalID:   "manual",
		StrategyID: "manual",
		Symbol:     r.Symbol,
		Side:       model.OrderSide(r.Side),
		OrderType:  model.OrderType(r.OrderType),
		Quantity:   r.Quantity,
	}
	if r.LimitPrice != nil {
		p, _ := decimal.NewFromString(*r.LimitPrice)
		sig.LimitPrice = &p
	}
	if r.StopPrice != nil {
		p, _ := decimal.NewFromString(*r.StopPrice)
		sig.StopPrice = &p
	}
	return sig
}
