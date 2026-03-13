package broker

import (
	"context"

	"trading-platform/oms/internal/model"

	"github.com/shopspring/decimal"
)

type BrokerAdapter interface {
	Name() string
	SubmitOrder(ctx context.Context, order *model.Order) (brokerOrderID string, err error)
	CancelOrder(ctx context.Context, brokerOrderID string) error
	GetOrderStatus(ctx context.Context, brokerOrderID string) (*model.Order, error)
	GetAllOrders(ctx context.Context) ([]*model.Order, error)
	GetPositions(ctx context.Context) ([]*model.Position, error)
	GetAccount(ctx context.Context) (*AccountInfo, error)
	StreamUpdates(ctx context.Context) (<-chan OrderUpdate, error)
	IsConnected() bool
}

type AccountInfo struct {
	Equity        decimal.Decimal `json:"equity"`
	Cash          decimal.Decimal `json:"cash"`
	BuyingPower   decimal.Decimal `json:"buying_power"`
	DayTradeCount int             `json:"day_trade_count"`
	PDTFlagged    bool            `json:"pdt_flagged"`
}

type OrderUpdate struct {
	BrokerOrderID string
	Event         string
	FilledQty     int
	FilledPrice   decimal.Decimal
	Commission    decimal.Decimal
	Symbol        string
	Side          model.OrderSide
}
