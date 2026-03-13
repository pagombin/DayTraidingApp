package mock

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"trading-platform/market-data/internal/model"
)

// MockProvider generates fake market data for testing when no Alpaca key is configured.
type MockProvider struct {
	symbols  []string
	mu       sync.RWMutex
	state    model.ConnectionState
	tickCh   chan model.MarketTick
	barCh    chan model.Bar
	tradeCh  chan model.Trade
	statusCh chan model.ConnectionState
	cancel   context.CancelFunc
	wg       sync.WaitGroup
	logger   *zap.Logger
}

func NewMockProvider(logger *zap.Logger) *MockProvider {
	return &MockProvider{
		state:    model.StateDisconnected,
		tickCh:   make(chan model.MarketTick, 10000),
		barCh:    make(chan model.Bar, 1000),
		tradeCh:  make(chan model.Trade, 10000),
		statusCh: make(chan model.ConnectionState, 100),
		logger:   logger,
	}
}

func (m *MockProvider) Name() string { return "mock" }

func (m *MockProvider) Connect(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	m.state = model.StateConnected
	m.statusCh <- model.StateConnected
	m.logger.Info("Mock provider connected")

	m.wg.Add(1)
	go m.generateTicks(ctx)

	return nil
}

func (m *MockProvider) Subscribe(_ context.Context, symbols []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing := make(map[string]bool)
	for _, s := range m.symbols {
		existing[s] = true
	}
	for _, s := range symbols {
		if !existing[s] {
			m.symbols = append(m.symbols, s)
		}
	}
	m.logger.Info("Mock subscribed", zap.Strings("symbols", symbols))
	return nil
}

func (m *MockProvider) Unsubscribe(_ context.Context, symbols []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	remove := make(map[string]bool)
	for _, s := range symbols {
		remove[s] = true
	}
	var remaining []string
	for _, s := range m.symbols {
		if !remove[s] {
			remaining = append(remaining, s)
		}
	}
	m.symbols = remaining
	return nil
}

func (m *MockProvider) TickChannel() <-chan model.MarketTick       { return m.tickCh }
func (m *MockProvider) BarChannel() <-chan model.Bar                { return m.barCh }
func (m *MockProvider) TradeChannel() <-chan model.Trade            { return m.tradeCh }
func (m *MockProvider) StatusChannel() <-chan model.ConnectionState { return m.statusCh }

func (m *MockProvider) Disconnect() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.wg.Wait()
	m.state = model.StateDisconnected
	return nil
}

func (m *MockProvider) IsConnected() bool { return m.state == model.StateConnected }

func (m *MockProvider) FetchHistoricalBars(_ context.Context, symbol string, start, end time.Time, _ string) ([]model.Bar, error) {
	return []model.Bar{}, nil
}

func (m *MockProvider) FetchOptionChain(_ context.Context, _ string, _ *time.Time) ([]model.OptionQuote, error) {
	return []model.OptionQuote{}, nil
}

func (m *MockProvider) FetchLatestQuote(_ context.Context, symbol string) (*model.MarketTick, error) {
	price := 100.0 + rand.Float64()*100.0
	tick := &model.MarketTick{
		Symbol:    symbol,
		Timestamp: time.Now(),
		Bid:       decimal.NewFromFloat(price - 0.01),
		Ask:       decimal.NewFromFloat(price + 0.01),
		Last:      decimal.NewFromFloat(price),
		Source:    "mock",
	}
	return tick, nil
}

// basePrices for well-known symbols
var basePrices = map[string]float64{
	"AAPL": 178.50, "MSFT": 415.20, "GOOGL": 141.80, "AMZN": 185.60,
	"NVDA": 875.30, "META": 485.90, "TSLA": 245.70, "SPY": 505.40,
	"QQQ": 435.20, "IWM": 198.50,
}

func (m *MockProvider) generateTicks(ctx context.Context) {
	defer m.wg.Done()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	prices := make(map[string]float64)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.RLock()
			syms := make([]string, len(m.symbols))
			copy(syms, m.symbols)
			m.mu.RUnlock()

			for _, symbol := range syms {
				price, ok := prices[symbol]
				if !ok {
					if bp, found := basePrices[symbol]; found {
						price = bp
					} else {
						price = 50.0 + rand.Float64()*200.0
					}
				}

				// Random walk
				change := (rand.Float64() - 0.5) * 0.10
				price += change
				prices[symbol] = price

				spread := 0.01 + rand.Float64()*0.03
				tick := model.MarketTick{
					Symbol:    symbol,
					Timestamp: time.Now(),
					Bid:       decimal.NewFromFloat(price - spread/2),
					Ask:       decimal.NewFromFloat(price + spread/2),
					Last:      decimal.NewFromFloat(price),
					Volume:    int64(rand.Intn(1000) + 100),
					Source:    "mock",
				}

				select {
				case m.tickCh <- tick:
				default:
				}
			}
		}
	}
}
