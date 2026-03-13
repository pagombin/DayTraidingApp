package aggregator

import (
	"sync"
	"time"

	"github.com/shopspring/decimal"
	"trading-platform/market-data/internal/model"
)

type barBuilder struct {
	symbol     string
	open       decimal.Decimal
	high       decimal.Decimal
	low        decimal.Decimal
	close      decimal.Decimal
	volume     int64
	tradeCount int
	vwapNum    decimal.Decimal // sum(price * volume)
	vwapDen    int64           // sum(volume)
	startTime  time.Time
	hasData    bool
}

// BarAggregator aggregates ticks into 1-minute OHLCV bars.
type BarAggregator struct {
	bars     map[string]*barBuilder
	mu       sync.Mutex
	outputCh chan model.Bar
	done     chan struct{}
}

func NewBarAggregator() *BarAggregator {
	return &BarAggregator{
		bars:     make(map[string]*barBuilder),
		outputCh: make(chan model.Bar, 1000),
		done:     make(chan struct{}),
	}
}

func (a *BarAggregator) OutputChannel() <-chan model.Bar {
	return a.outputCh
}

// AddTick updates the in-progress bar for the tick's symbol.
func (a *BarAggregator) AddTick(tick model.MarketTick) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Align to the start of the current minute
	minuteStart := tick.Timestamp.Truncate(time.Minute)

	bb, ok := a.bars[tick.Symbol]
	if !ok || bb.startTime != minuteStart {
		// Emit previous bar if it exists
		if ok && bb.hasData {
			a.emitBar(bb)
		}
		// Start new bar
		bb = &barBuilder{
			symbol:    tick.Symbol,
			open:      tick.Last,
			high:      tick.Last,
			low:       tick.Last,
			startTime: minuteStart,
		}
		a.bars[tick.Symbol] = bb
	}

	bb.close = tick.Last
	bb.hasData = true
	bb.tradeCount++

	if tick.Last.GreaterThan(bb.high) {
		bb.high = tick.Last
	}
	if tick.Last.LessThan(bb.low) {
		bb.low = tick.Last
	}

	bb.volume += tick.Volume

	// VWAP calculation
	if tick.Volume > 0 {
		bb.vwapNum = bb.vwapNum.Add(tick.Last.Mul(decimal.NewFromInt(tick.Volume)))
		bb.vwapDen += tick.Volume
	}
}

// Start begins the background timer that checks for minute boundaries.
func (a *BarAggregator) Start() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-a.done:
				return
			case <-ticker.C:
				a.checkAndEmit()
			}
		}
	}()
}

// Stop stops the aggregator and emits any remaining bars.
func (a *BarAggregator) Stop() {
	close(a.done)
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, bb := range a.bars {
		if bb.hasData {
			a.emitBar(bb)
		}
	}
}

func (a *BarAggregator) checkAndEmit() {
	a.mu.Lock()
	defer a.mu.Unlock()

	now := time.Now().Truncate(time.Minute)
	for symbol, bb := range a.bars {
		if bb.hasData && bb.startTime.Before(now) {
			a.emitBar(bb)
			delete(a.bars, symbol)
		}
	}
}

func (a *BarAggregator) emitBar(bb *barBuilder) {
	vwap := bb.close // default to close if no volume
	if bb.vwapDen > 0 {
		vwap = bb.vwapNum.Div(decimal.NewFromInt(bb.vwapDen))
	}

	bar := model.Bar{
		Symbol:     bb.symbol,
		Timestamp:  bb.startTime,
		Open:       bb.open,
		High:       bb.high,
		Low:        bb.low,
		Close:      bb.close,
		Volume:     bb.volume,
		TradeCount: bb.tradeCount,
		VWAP:       vwap,
		Source:     "aggregator",
	}

	select {
	case a.outputCh <- bar:
	default:
	}
}
