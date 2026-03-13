package paper

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"trading-platform/oms/internal/broker"
	"trading-platform/oms/internal/model"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type PaperSimulator struct {
	account    broker.AccountInfo
	positions  map[string]*model.Position
	openOrders map[string]*model.Order
	mu         sync.RWMutex
	rdb        *redis.Client
	updateCh   chan broker.OrderUpdate
	slippage   *SlippageModel
	log        *zap.SugaredLogger
}

func NewPaperSimulator(rdb *redis.Client, log *zap.SugaredLogger) *PaperSimulator {
	return &PaperSimulator{
		account: broker.AccountInfo{
			Equity:      decimal.NewFromInt(100000),
			Cash:        decimal.NewFromInt(100000),
			BuyingPower: decimal.NewFromInt(200000),
		},
		positions:  make(map[string]*model.Position),
		openOrders: make(map[string]*model.Order),
		rdb:        rdb,
		updateCh:   make(chan broker.OrderUpdate, 100),
		slippage:   NewSlippageModel(),
		log:        log,
	}
}

func (ps *PaperSimulator) Name() string { return "paper" }

func (ps *PaperSimulator) SubmitOrder(ctx context.Context, order *model.Order) (string, error) {
	brokerID := uuid.New().String()
	order.BrokerOrderID = brokerID

	ps.mu.Lock()
	ps.openOrders[brokerID] = order
	ps.mu.Unlock()

	// For market orders, fill immediately
	if order.OrderType == model.OrderTypeMarket {
		go ps.fillMarketOrder(ctx, order, brokerID)
	} else {
		// For limit/stop orders, monitor price
		go ps.monitorOrder(ctx, order, brokerID)
	}

	ps.log.Infow("Paper order submitted",
		"broker_id", brokerID, "symbol", order.Symbol,
		"side", order.Side, "qty", order.Quantity, "type", order.OrderType)

	return brokerID, nil
}

func (ps *PaperSimulator) fillMarketOrder(ctx context.Context, order *model.Order, brokerID string) {
	price := ps.getLatestPrice(ctx, order.Symbol)
	if price.IsZero() {
		price = decimal.NewFromInt(100) // fallback for paper
	}

	// Apply slippage
	slippage := ps.slippage.Calculate(price, order.Quantity)
	if order.Side == model.OrderBuy {
		price = price.Add(slippage)
	} else {
		price = price.Sub(slippage)
	}

	ps.processFill(order, brokerID, order.Quantity, price)
}

func (ps *PaperSimulator) monitorOrder(ctx context.Context, order *model.Order, brokerID string) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ps.mu.RLock()
			_, exists := ps.openOrders[brokerID]
			ps.mu.RUnlock()
			if !exists {
				return // order cancelled or filled
			}

			price := ps.getLatestPrice(ctx, order.Symbol)
			if price.IsZero() {
				continue
			}

			shouldFill := false
			switch order.OrderType {
			case model.OrderTypeLimit:
				if order.Side == model.OrderBuy && order.LimitPrice != nil && price.LessThanOrEqual(*order.LimitPrice) {
					shouldFill = true
				}
				if order.Side == model.OrderSell && order.LimitPrice != nil && price.GreaterThanOrEqual(*order.LimitPrice) {
					shouldFill = true
				}
			case model.OrderTypeStop:
				if order.Side == model.OrderBuy && order.StopPrice != nil && price.GreaterThanOrEqual(*order.StopPrice) {
					shouldFill = true
				}
				if order.Side == model.OrderSell && order.StopPrice != nil && price.LessThanOrEqual(*order.StopPrice) {
					shouldFill = true
				}
			}

			if shouldFill {
				slippage := ps.slippage.Calculate(price, order.Quantity)
				if order.Side == model.OrderBuy {
					price = price.Add(slippage)
				} else {
					price = price.Sub(slippage)
				}
				ps.processFill(order, brokerID, order.Quantity, price)
				return
			}
		}
	}
}

func (ps *PaperSimulator) processFill(order *model.Order, brokerID string, qty int, price decimal.Decimal) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	delete(ps.openOrders, brokerID)

	// Update positions
	orderValue := price.Mul(decimal.NewFromInt(int64(qty)))
	if order.Side == model.OrderBuy {
		pos, exists := ps.positions[order.Symbol]
		if exists {
			// Add to existing position
			totalCost := pos.AvgCost.Mul(decimal.NewFromInt(int64(pos.Quantity))).Add(orderValue)
			pos.Quantity += qty
			pos.AvgCost = totalCost.Div(decimal.NewFromInt(int64(pos.Quantity)))
		} else {
			ps.positions[order.Symbol] = &model.Position{
				PositionID: uuid.New().String(),
				AccountID:  "default",
				StrategyID: order.StrategyID,
				Symbol:     order.Symbol,
				Quantity:   qty,
				AvgCost:    price,
				OpenedAt:   time.Now(),
			}
		}
		ps.account.Cash = ps.account.Cash.Sub(orderValue)
	} else {
		pos, exists := ps.positions[order.Symbol]
		if exists {
			pos.Quantity -= qty
			if pos.Quantity <= 0 {
				delete(ps.positions, order.Symbol)
			}
		}
		ps.account.Cash = ps.account.Cash.Add(orderValue)
	}

	// Recalculate equity
	ps.recalculateEquity()

	// Send fill update
	ps.updateCh <- broker.OrderUpdate{
		BrokerOrderID: brokerID,
		Event:         "fill",
		FilledQty:     qty,
		FilledPrice:   price,
		Commission:    decimal.Zero,
		Symbol:        order.Symbol,
		Side:          order.Side,
	}

	ps.log.Infow("Paper order filled",
		"broker_id", brokerID, "symbol", order.Symbol,
		"side", order.Side, "qty", qty, "price", price.StringFixed(2))
}

func (ps *PaperSimulator) recalculateEquity() {
	equity := ps.account.Cash
	for _, pos := range ps.positions {
		if !pos.CurrentPrice.IsZero() {
			equity = equity.Add(pos.CurrentPrice.Mul(decimal.NewFromInt(int64(pos.Quantity))))
		} else {
			equity = equity.Add(pos.AvgCost.Mul(decimal.NewFromInt(int64(pos.Quantity))))
		}
	}
	ps.account.Equity = equity
	ps.account.BuyingPower = equity.Mul(decimal.NewFromInt(2))
}

func (ps *PaperSimulator) CancelOrder(ctx context.Context, brokerOrderID string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if _, exists := ps.openOrders[brokerOrderID]; !exists {
		return fmt.Errorf("order %s not found", brokerOrderID)
	}
	delete(ps.openOrders, brokerOrderID)
	ps.updateCh <- broker.OrderUpdate{
		BrokerOrderID: brokerOrderID,
		Event:         "cancelled",
	}
	return nil
}

func (ps *PaperSimulator) GetOrderStatus(ctx context.Context, brokerOrderID string) (*model.Order, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if order, exists := ps.openOrders[brokerOrderID]; exists {
		return order, nil
	}
	return nil, fmt.Errorf("order %s not found", brokerOrderID)
}

func (ps *PaperSimulator) GetAllOrders(ctx context.Context) ([]*model.Order, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var orders []*model.Order
	for _, o := range ps.openOrders {
		orders = append(orders, o)
	}
	return orders, nil
}

func (ps *PaperSimulator) GetPositions(ctx context.Context) ([]*model.Position, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	var positions []*model.Position
	for _, p := range ps.positions {
		pos := *p
		positions = append(positions, &pos)
	}
	return positions, nil
}

func (ps *PaperSimulator) GetAccount(ctx context.Context) (*broker.AccountInfo, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	acc := ps.account
	return &acc, nil
}

func (ps *PaperSimulator) StreamUpdates(ctx context.Context) (<-chan broker.OrderUpdate, error) {
	return ps.updateCh, nil
}

func (ps *PaperSimulator) IsConnected() bool { return true }

func (ps *PaperSimulator) getLatestPrice(ctx context.Context, symbol string) decimal.Decimal {
	key := fmt.Sprintf("market:latest:%s", symbol)
	val, err := ps.rdb.Get(ctx, key).Result()
	if err != nil {
		return decimal.Zero
	}
	var quote struct {
		Last string `json:"last"`
		Ask  string `json:"ask"`
		Bid  string `json:"bid"`
	}
	if err := json.Unmarshal([]byte(val), &quote); err != nil {
		return decimal.Zero
	}
	if quote.Last != "" {
		p, _ := decimal.NewFromString(quote.Last)
		return p
	}
	if quote.Ask != "" {
		p, _ := decimal.NewFromString(quote.Ask)
		return p
	}
	return decimal.Zero
}

func (ps *PaperSimulator) UpdatePrice(symbol string, price decimal.Decimal) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if pos, exists := ps.positions[symbol]; exists {
		pos.CurrentPrice = price
		pos.MarketValue = price.Mul(decimal.NewFromInt(int64(pos.Quantity)))
		pos.UnrealizedPnL = pos.MarketValue.Sub(pos.AvgCost.Mul(decimal.NewFromInt(int64(pos.Quantity))))
	}
	ps.recalculateEquity()
}
