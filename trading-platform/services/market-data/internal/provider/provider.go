package provider

import (
	"context"
	"time"

	"trading-platform/market-data/internal/model"
)

// DataProvider is the interface that all market data providers must implement.
type DataProvider interface {
	Name() string
	Connect(ctx context.Context) error
	Subscribe(ctx context.Context, symbols []string) error
	Unsubscribe(ctx context.Context, symbols []string) error
	TickChannel() <-chan model.MarketTick
	BarChannel() <-chan model.Bar
	TradeChannel() <-chan model.Trade
	StatusChannel() <-chan model.ConnectionState
	FetchHistoricalBars(ctx context.Context, symbol string, start, end time.Time, timeframe string) ([]model.Bar, error)
	FetchOptionChain(ctx context.Context, underlying string, expiration *time.Time) ([]model.OptionQuote, error)
	FetchLatestQuote(ctx context.Context, symbol string) (*model.MarketTick, error)
	Disconnect() error
	IsConnected() bool
}
