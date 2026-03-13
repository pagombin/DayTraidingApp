package alpaca

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"trading-platform/market-data/internal/model"
	"trading-platform/market-data/pkg/reconnect"
)

type AlpacaProvider struct {
	apiKey    string
	apiSecret string
	wsURL     string
	restURL   string

	conn   *websocket.Conn
	mu     sync.RWMutex
	state  model.ConnectionState
	symbols []string

	tickCh   chan model.MarketTick
	barCh    chan model.Bar
	tradeCh  chan model.Trade
	statusCh chan model.ConnectionState

	symbolStatus map[string]*model.SymbolStatus
	statusMu     sync.RWMutex

	totalTicks  atomic.Int64
	tickRate    atomic.Int64
	connectedAt atomic.Value

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *zap.Logger
}

func NewAlpacaProvider(apiKey, apiSecret, wsURL, restURL string, logger *zap.Logger) *AlpacaProvider {
	ctx, cancel := context.WithCancel(context.Background())
	return &AlpacaProvider{
		apiKey:       apiKey,
		apiSecret:    apiSecret,
		wsURL:        wsURL,
		restURL:      restURL,
		state:        model.StateDisconnected,
		tickCh:       make(chan model.MarketTick, 10000),
		barCh:        make(chan model.Bar, 1000),
		tradeCh:      make(chan model.Trade, 10000),
		statusCh:     make(chan model.ConnectionState, 100),
		symbolStatus: make(map[string]*model.SymbolStatus),
		ctx:          ctx,
		cancel:       cancel,
		logger:       logger,
	}
}

func (p *AlpacaProvider) Name() string { return "alpaca" }

func (p *AlpacaProvider) Connect(ctx context.Context) error {
	p.setState(model.StateConnecting)

	cfg := reconnect.Config{
		InitialDelay: 0,
		MaxDelay:     60 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  0,
		AlertAfter:   5 * time.Minute,
		OnAlert: func(err error, dur time.Duration) {
			p.logger.Error("Alpaca connection down for extended period",
				zap.Error(err), zap.Duration("downtime", dur))
		},
	}

	return reconnect.Run(ctx, p.logger, cfg, func() error {
		dialer := websocket.Dialer{
			HandshakeTimeout: 10 * time.Second,
		}

		conn, _, err := dialer.DialContext(ctx, p.wsURL, nil)
		if err != nil {
			return fmt.Errorf("websocket dial: %w", err)
		}

		if err := authenticate(conn, p.apiKey, p.apiSecret); err != nil {
			conn.Close()
			return fmt.Errorf("auth: %w", err)
		}

		p.mu.Lock()
		p.conn = conn
		p.mu.Unlock()

		now := time.Now()
		p.connectedAt.Store(now)
		p.setState(model.StateConnected)
		p.logger.Info("Connected to Alpaca WebSocket", zap.String("url", p.wsURL))

		// Re-subscribe if we had symbols
		p.mu.RLock()
		syms := make([]string, len(p.symbols))
		copy(syms, p.symbols)
		p.mu.RUnlock()

		if len(syms) > 0 {
			if err := p.sendSubscribe(syms); err != nil {
				p.logger.Warn("Failed to re-subscribe after reconnect", zap.Error(err))
			}
		}

		// Start read loop in background
		p.wg.Add(1)
		go p.readLoop()

		return nil
	})
}

func (p *AlpacaProvider) Subscribe(ctx context.Context, symbols []string) error {
	p.mu.Lock()
	// Add new symbols (deduplicate)
	existing := make(map[string]bool)
	for _, s := range p.symbols {
		existing[s] = true
	}
	var newSyms []string
	for _, s := range symbols {
		if !existing[s] {
			p.symbols = append(p.symbols, s)
			newSyms = append(newSyms, s)
		}
	}
	p.mu.Unlock()

	if len(newSyms) == 0 {
		return nil
	}

	p.statusMu.Lock()
	for _, s := range newSyms {
		if _, ok := p.symbolStatus[s]; !ok {
			p.symbolStatus[s] = &model.SymbolStatus{Symbol: s}
		}
	}
	p.statusMu.Unlock()

	if !p.IsConnected() {
		return nil // will subscribe on connect
	}

	return p.sendSubscribe(newSyms)
}

func (p *AlpacaProvider) Unsubscribe(ctx context.Context, symbols []string) error {
	p.mu.Lock()
	remove := make(map[string]bool)
	for _, s := range symbols {
		remove[s] = true
	}
	var remaining []string
	for _, s := range p.symbols {
		if !remove[s] {
			remaining = append(remaining, s)
		}
	}
	p.symbols = remaining
	p.mu.Unlock()

	if !p.IsConnected() {
		return nil
	}

	return p.sendUnsubscribe(symbols)
}

func (p *AlpacaProvider) TickChannel() <-chan model.MarketTick      { return p.tickCh }
func (p *AlpacaProvider) BarChannel() <-chan model.Bar               { return p.barCh }
func (p *AlpacaProvider) TradeChannel() <-chan model.Trade           { return p.tradeCh }
func (p *AlpacaProvider) StatusChannel() <-chan model.ConnectionState { return p.statusCh }

func (p *AlpacaProvider) Disconnect() error {
	p.cancel()
	p.mu.Lock()
	conn := p.conn
	p.conn = nil
	p.mu.Unlock()

	if conn != nil {
		conn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		conn.Close()
	}

	p.setState(model.StateDisconnected)
	p.wg.Wait()
	return nil
}

func (p *AlpacaProvider) IsConnected() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state == model.StateConnected && p.conn != nil
}

func (p *AlpacaProvider) setState(state model.ConnectionState) {
	p.mu.Lock()
	p.state = state
	p.mu.Unlock()

	select {
	case p.statusCh <- state:
	default:
	}
}

func (p *AlpacaProvider) sendSubscribe(symbols []string) error {
	p.mu.RLock()
	conn := p.conn
	p.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := map[string]interface{}{
		"action": "subscribe",
		"trades": symbols,
		"quotes": symbols,
		"bars":   symbols,
	}
	return conn.WriteJSON(msg)
}

func (p *AlpacaProvider) sendUnsubscribe(symbols []string) error {
	p.mu.RLock()
	conn := p.conn
	p.mu.RUnlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}

	msg := map[string]interface{}{
		"action": "unsubscribe",
		"trades": symbols,
		"quotes": symbols,
		"bars":   symbols,
	}
	return conn.WriteJSON(msg)
}

func (p *AlpacaProvider) readLoop() {
	defer p.wg.Done()

	for {
		p.mu.RLock()
		conn := p.conn
		p.mu.RUnlock()

		if conn == nil {
			return
		}

		_, msg, err := conn.ReadMessage()
		if err != nil {
			if p.ctx.Err() != nil {
				return // context cancelled, shutting down
			}
			p.logger.Warn("WebSocket read error, will reconnect", zap.Error(err))
			p.setState(model.StateReconnecting)
			// Reconnect in a new goroutine
			go func() {
				if err := p.Connect(p.ctx); err != nil {
					p.logger.Error("Reconnection failed", zap.Error(err))
				}
			}()
			return
		}

		p.processMessage(msg)
	}
}

func (p *AlpacaProvider) processMessage(data []byte) {
	var messages []json.RawMessage
	if err := json.Unmarshal(data, &messages); err != nil {
		p.logger.Warn("Failed to parse WebSocket message", zap.Error(err))
		return
	}

	for _, raw := range messages {
		var header struct {
			T string `json:"T"`
		}
		if err := json.Unmarshal(raw, &header); err != nil {
			continue
		}

		switch header.T {
		case "q":
			var q rawQuote
			if err := json.Unmarshal(raw, &q); err != nil {
				p.logger.Warn("Failed to parse quote", zap.Error(err))
				continue
			}
			tick := parseQuoteToTick(q)
			p.updateSymbolStatus(tick.Symbol, tick.Timestamp)
			p.totalTicks.Add(1)
			select {
			case p.tickCh <- tick:
			default:
				p.logger.Warn("Tick channel full, dropping tick", zap.String("symbol", tick.Symbol))
			}

		case "t":
			var t rawTrade
			if err := json.Unmarshal(raw, &t); err != nil {
				p.logger.Warn("Failed to parse trade", zap.Error(err))
				continue
			}
			trade := parseTradeToTrade(t)
			tick := parseTradeToTick(t)
			p.updateSymbolStatus(trade.Symbol, trade.Timestamp)
			p.totalTicks.Add(1)

			select {
			case p.tradeCh <- trade:
			default:
			}
			select {
			case p.tickCh <- tick:
			default:
			}

		case "b":
			var b rawBar
			if err := json.Unmarshal(raw, &b); err != nil {
				p.logger.Warn("Failed to parse bar", zap.Error(err))
				continue
			}
			bar := parseBarToBar(b)
			select {
			case p.barCh <- bar:
			default:
				p.logger.Warn("Bar channel full, dropping bar", zap.String("symbol", bar.Symbol))
			}

		case "success", "subscription":
			// Control messages — log and continue
			p.logger.Debug("Control message", zap.ByteString("msg", raw))

		case "error":
			p.logger.Error("Alpaca error message", zap.ByteString("msg", raw))
		}
	}
}

func (p *AlpacaProvider) updateSymbolStatus(symbol string, ts time.Time) {
	p.statusMu.Lock()
	defer p.statusMu.Unlock()
	ss, ok := p.symbolStatus[symbol]
	if !ok {
		ss = &model.SymbolStatus{Symbol: symbol}
		p.symbolStatus[symbol] = ss
	}
	ss.LastTickTime = ts
	ss.TickCount++
	ss.IsStale = false
}

// GetStatus returns the current service status snapshot.
func (p *AlpacaProvider) GetStatus() model.ServiceStatus {
	p.mu.RLock()
	state := p.state
	symCount := len(p.symbols)
	p.mu.RUnlock()

	status := model.ServiceStatus{
		State:           state,
		Provider:        "alpaca",
		SymbolCount:     symCount,
		TotalTicksToday: p.totalTicks.Load(),
		TicksPerSecond:  float64(p.tickRate.Load()),
	}

	if v := p.connectedAt.Load(); v != nil {
		if ct, ok := v.(time.Time); ok {
			status.ConnectedSince = &ct
			status.Uptime = time.Since(ct).Truncate(time.Second).String()
		}
	}

	// Check stale symbols
	p.statusMu.RLock()
	for _, ss := range p.symbolStatus {
		if time.Since(ss.LastTickTime) > 60*time.Second && ss.TickCount > 0 {
			ss.IsStale = true
			status.StaleSymbols = append(status.StaleSymbols, ss.Symbol)
		}
	}
	p.statusMu.RUnlock()

	return status
}

// StartTickRateTracker starts a goroutine that calculates ticks/second.
func (p *AlpacaProvider) StartTickRateTracker(ctx context.Context) {
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		var lastCount int64
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				current := p.totalTicks.Load()
				p.tickRate.Store(current - lastCount)
				lastCount = current
			}
		}
	}()
}

// SnapshotResult holds the full snapshot data for a symbol including prev close.
type SnapshotResult struct {
	Tick      model.MarketTick
	PrevClose float64
}

// FetchMultiSnapshots fetches snapshots for multiple symbols in one request.
// Returns tick data with volume/last trade and previous daily close for change calculation.
func (p *AlpacaProvider) FetchMultiSnapshots(ctx context.Context, symbols []string) ([]SnapshotResult, error) {
	if len(symbols) == 0 {
		return nil, nil
	}

	// Build comma-separated symbols list
	symList := ""
	for i, s := range symbols {
		if i > 0 {
			symList += ","
		}
		symList += s
	}

	url := fmt.Sprintf("%s/stocks/snapshots?symbols=%s&feed=iex", p.restURL, symList)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", p.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", p.apiSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("alpaca snapshots API error %d: %s", resp.StatusCode, string(body))
	}

	var snapshots map[string]struct {
		LatestTrade struct {
			T string  `json:"t"`
			P float64 `json:"p"`
		} `json:"latestTrade"`
		LatestQuote struct {
			T  string  `json:"t"`
			Bp float64 `json:"bp"`
			Ap float64 `json:"ap"`
		} `json:"latestQuote"`
		DailyBar struct {
			V int64   `json:"v"`
			C float64 `json:"c"`
		} `json:"dailyBar"`
		PrevDailyBar struct {
			C float64 `json:"c"`
		} `json:"prevDailyBar"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&snapshots); err != nil {
		return nil, fmt.Errorf("decode snapshots: %w", err)
	}

	var results []SnapshotResult
	for sym, snap := range snapshots {
		tsStr := snap.LatestTrade.T
		if tsStr == "" {
			tsStr = snap.LatestQuote.T
		}
		ts, _ := time.Parse(time.RFC3339Nano, tsStr)

		lastPrice := snap.LatestTrade.P
		if lastPrice == 0 && snap.LatestQuote.Bp > 0 {
			lastPrice = (snap.LatestQuote.Bp + snap.LatestQuote.Ap) / 2
		}

		results = append(results, SnapshotResult{
			Tick: model.MarketTick{
				Symbol:    sym,
				Timestamp: ts,
				Bid:       decimal.NewFromFloat(snap.LatestQuote.Bp),
				Ask:       decimal.NewFromFloat(snap.LatestQuote.Ap),
				Last:      decimal.NewFromFloat(lastPrice),
				Volume:    snap.DailyBar.V,
				Source:    "alpaca",
			},
			PrevClose: snap.PrevDailyBar.C,
		})
	}

	return results, nil
}

// --- REST API Methods ---

func (p *AlpacaProvider) FetchHistoricalBars(ctx context.Context, symbol string, start, end time.Time, timeframe string) ([]model.Bar, error) {
	url := fmt.Sprintf("%s/stocks/%s/bars?timeframe=%s&start=%s&end=%s&limit=10000&adjustment=raw&feed=iex",
		p.restURL, symbol, timeframe,
		start.Format(time.RFC3339), end.Format(time.RFC3339))

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", p.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", p.apiSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch bars: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("alpaca API error %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Bars []struct {
			T  string  `json:"t"`
			O  float64 `json:"o"`
			H  float64 `json:"h"`
			L  float64 `json:"l"`
			C  float64 `json:"c"`
			V  int64   `json:"v"`
			N  int     `json:"n"`
			VW float64 `json:"vw"`
		} `json:"bars"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode bars: %w", err)
	}

	bars := make([]model.Bar, 0, len(result.Bars))
	for _, b := range result.Bars {
		ts, _ := time.Parse(time.RFC3339Nano, b.T)
		bars = append(bars, model.Bar{
			Symbol:     symbol,
			Timestamp:  ts,
			Open:       decimal.NewFromFloat(b.O),
			High:       decimal.NewFromFloat(b.H),
			Low:        decimal.NewFromFloat(b.L),
			Close:      decimal.NewFromFloat(b.C),
			Volume:     b.V,
			TradeCount: b.N,
			VWAP:       decimal.NewFromFloat(b.VW),
			Source:     "alpaca",
		})
	}

	return bars, nil
}

func (p *AlpacaProvider) FetchOptionChain(ctx context.Context, underlying string, expiration *time.Time) ([]model.OptionQuote, error) {
	url := fmt.Sprintf("%s/../v1beta1/options/snapshots/%s", p.restURL, underlying)
	if expiration != nil {
		url += "?expiration_date=" + expiration.Format("2006-01-02")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", p.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", p.apiSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("alpaca options API error %d: %s", resp.StatusCode, string(body))
	}

	// Alpaca options snapshot response is complex; return empty for now
	// Full parsing will be implemented when options trading is enabled
	return []model.OptionQuote{}, nil
}

func (p *AlpacaProvider) FetchLatestQuote(ctx context.Context, symbol string) (*model.MarketTick, error) {
	// Use the snapshot endpoint which returns last trade, quote, minute bar, daily bar, and prev daily bar
	url := fmt.Sprintf("%s/stocks/%s/snapshot?feed=iex", p.restURL, symbol)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("APCA-API-KEY-ID", p.apiKey)
	req.Header.Set("APCA-API-SECRET-KEY", p.apiSecret)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("alpaca API error %d: %s", resp.StatusCode, string(body))
	}

	var snap struct {
		LatestTrade struct {
			T string  `json:"t"`
			P float64 `json:"p"`
			S int64   `json:"s"`
		} `json:"latestTrade"`
		LatestQuote struct {
			T  string  `json:"t"`
			Bp float64 `json:"bp"`
			Ap float64 `json:"ap"`
			Bs int     `json:"bs"`
			As int     `json:"as"`
		} `json:"latestQuote"`
		DailyBar struct {
			T string  `json:"t"`
			V int64   `json:"v"`
			C float64 `json:"c"`
		} `json:"dailyBar"`
		PrevDailyBar struct {
			C float64 `json:"c"`
			V int64   `json:"v"`
		} `json:"prevDailyBar"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&snap); err != nil {
		return nil, err
	}

	// Use latest trade timestamp, fall back to quote timestamp
	tsStr := snap.LatestTrade.T
	if tsStr == "" {
		tsStr = snap.LatestQuote.T
	}
	ts, _ := time.Parse(time.RFC3339Nano, tsStr)

	// Last price from latest trade; fall back to quote midpoint
	lastPrice := snap.LatestTrade.P
	if lastPrice == 0 {
		lastPrice = (snap.LatestQuote.Bp + snap.LatestQuote.Ap) / 2
	}

	// Volume from today's daily bar
	volume := snap.DailyBar.V

	tick := &model.MarketTick{
		Symbol:    symbol,
		Timestamp: ts,
		Bid:       decimal.NewFromFloat(snap.LatestQuote.Bp),
		Ask:       decimal.NewFromFloat(snap.LatestQuote.Ap),
		Last:      decimal.NewFromFloat(lastPrice),
		Volume:    volume,
		Source:    "alpaca",
	}
	return tick, nil
}
