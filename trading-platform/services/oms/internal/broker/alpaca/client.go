package alpaca

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"trading-platform/oms/internal/broker"
	"trading-platform/oms/internal/model"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type Client struct {
	apiKey    string
	apiSecret string
	baseURL   string
	client    *http.Client
	connected bool
	updateCh  chan broker.OrderUpdate
	mu        sync.RWMutex
	log       *zap.SugaredLogger
}

func NewClient(apiKey, apiSecret, baseURL string, log *zap.SugaredLogger) *Client {
	return &Client{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   baseURL,
		client:    &http.Client{Timeout: 30 * time.Second},
		updateCh:  make(chan broker.OrderUpdate, 100),
		log:       log,
	}
}

func (c *Client) Name() string { return "alpaca" }

func (c *Client) SubmitOrder(ctx context.Context, order *model.Order) (string, error) {
	body := map[string]interface{}{
		"symbol":        order.Symbol,
		"qty":           order.Quantity,
		"side":          string(order.Side),
		"type":          string(order.OrderType),
		"time_in_force": string(order.TimeInForce),
	}
	if order.LimitPrice != nil {
		body["limit_price"] = order.LimitPrice.String()
	}
	if order.StopPrice != nil {
		body["stop_price"] = order.StopPrice.String()
	}

	resp, err := c.doRequest(ctx, "POST", "/v2/orders", body)
	if err != nil {
		return "", err
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp, &result); err != nil {
		return "", err
	}

	c.log.Infow("Order submitted to Alpaca", "broker_order_id", result.ID, "symbol", order.Symbol)
	return result.ID, nil
}

func (c *Client) CancelOrder(ctx context.Context, brokerOrderID string) error {
	_, err := c.doRequest(ctx, "DELETE", "/v2/orders/"+brokerOrderID, nil)
	return err
}

func (c *Client) GetOrderStatus(ctx context.Context, brokerOrderID string) (*model.Order, error) {
	resp, err := c.doRequest(ctx, "GET", "/v2/orders/"+brokerOrderID, nil)
	if err != nil {
		return nil, err
	}
	return parseAlpacaOrder(resp)
}

func (c *Client) GetAllOrders(ctx context.Context) ([]*model.Order, error) {
	resp, err := c.doRequest(ctx, "GET", "/v2/orders?status=open", nil)
	if err != nil {
		return nil, err
	}
	var alpacaOrders []json.RawMessage
	if err := json.Unmarshal(resp, &alpacaOrders); err != nil {
		return nil, err
	}
	var orders []*model.Order
	for _, raw := range alpacaOrders {
		o, err := parseAlpacaOrder(raw)
		if err == nil {
			orders = append(orders, o)
		}
	}
	return orders, nil
}

func (c *Client) GetPositions(ctx context.Context) ([]*model.Position, error) {
	resp, err := c.doRequest(ctx, "GET", "/v2/positions", nil)
	if err != nil {
		return nil, err
	}
	var alpacaPositions []struct {
		Symbol       string `json:"symbol"`
		Qty          string `json:"qty"`
		AvgEntryPrice string `json:"avg_entry_price"`
		CurrentPrice string `json:"current_price"`
		MarketValue  string `json:"market_value"`
		UnrealizedPL string `json:"unrealized_pl"`
		Side         string `json:"side"`
	}
	if err := json.Unmarshal(resp, &alpacaPositions); err != nil {
		return nil, err
	}
	var positions []*model.Position
	for _, ap := range alpacaPositions {
		qty, _ := decimal.NewFromString(ap.Qty)
		avgCost, _ := decimal.NewFromString(ap.AvgEntryPrice)
		currentPrice, _ := decimal.NewFromString(ap.CurrentPrice)
		marketValue, _ := decimal.NewFromString(ap.MarketValue)
		unrealizedPnL, _ := decimal.NewFromString(ap.UnrealizedPL)

		q := int(qty.IntPart())
		if ap.Side == "short" {
			q = -q
		}
		positions = append(positions, &model.Position{
			Symbol:        ap.Symbol,
			Quantity:      q,
			AvgCost:       avgCost,
			CurrentPrice:  currentPrice,
			MarketValue:   marketValue,
			UnrealizedPnL: unrealizedPnL,
		})
	}
	return positions, nil
}

func (c *Client) GetAccount(ctx context.Context) (*broker.AccountInfo, error) {
	resp, err := c.doRequest(ctx, "GET", "/v2/account", nil)
	if err != nil {
		return nil, err
	}
	var acc struct {
		Equity        string `json:"equity"`
		Cash          string `json:"cash"`
		BuyingPower   string `json:"buying_power"`
		DaytradeCount int    `json:"daytrade_count"`
		PDTFlagged    bool   `json:"pattern_day_trader"`
	}
	if err := json.Unmarshal(resp, &acc); err != nil {
		return nil, err
	}
	equity, _ := decimal.NewFromString(acc.Equity)
	cash, _ := decimal.NewFromString(acc.Cash)
	bp, _ := decimal.NewFromString(acc.BuyingPower)

	return &broker.AccountInfo{
		Equity:        equity,
		Cash:          cash,
		BuyingPower:   bp,
		DayTradeCount: acc.DaytradeCount,
		PDTFlagged:    acc.PDTFlagged,
	}, nil
}

func (c *Client) StreamUpdates(ctx context.Context) (<-chan broker.OrderUpdate, error) {
	return c.updateCh, nil
}

func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", c.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", c.apiSecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("alpaca API error %d: %s", resp.StatusCode, string(data))
	}

	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	return data, nil
}

func parseAlpacaOrder(data []byte) (*model.Order, error) {
	var ao struct {
		ID            string `json:"id"`
		Symbol        string `json:"symbol"`
		Side          string `json:"side"`
		Type          string `json:"type"`
		Qty           string `json:"qty"`
		FilledQty     string `json:"filled_qty"`
		Status        string `json:"status"`
		LimitPrice    string `json:"limit_price"`
		StopPrice     string `json:"stop_price"`
		AvgFillPrice  string `json:"filled_avg_price"`
	}
	if err := json.Unmarshal(data, &ao); err != nil {
		return nil, err
	}

	qty, _ := decimal.NewFromString(ao.Qty)
	filledQty, _ := decimal.NewFromString(ao.FilledQty)

	order := &model.Order{
		BrokerOrderID: ao.ID,
		Symbol:        ao.Symbol,
		Side:          model.OrderSide(ao.Side),
		OrderType:     model.OrderType(ao.Type),
		Quantity:      int(qty.IntPart()),
		FilledQty:     int(filledQty.IntPart()),
	}

	if ao.AvgFillPrice != "" {
		p, _ := decimal.NewFromString(ao.AvgFillPrice)
		order.AvgFillPrice = &p
	}

	switch ao.Status {
	case "new", "accepted":
		order.State = model.OrderAccepted
	case "partially_filled":
		order.State = model.OrderPartialFill
	case "filled":
		order.State = model.OrderFilled
	case "canceled":
		order.State = model.OrderCancelled
	case "expired":
		order.State = model.OrderExpired
	case "rejected":
		order.State = model.OrderRejected
	default:
		order.State = model.OrderSubmitted
	}

	return order, nil
}
