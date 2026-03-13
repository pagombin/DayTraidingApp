package alpaca

import (
	"time"

	"github.com/shopspring/decimal"
	"trading-platform/market-data/internal/model"
)

// Raw Alpaca message types received via WebSocket.
type rawQuote struct {
	T  string  `json:"T"`
	S  string  `json:"S"`
	Bx string  `json:"bx"`
	Bp float64 `json:"bp"`
	Bs int     `json:"bs"`
	Ax string  `json:"ax"`
	Ap float64 `json:"ap"`
	As int     `json:"as"`
	Z  string  `json:"z"`
	Ts string  `json:"t"`
}

type rawTrade struct {
	T  string  `json:"T"`
	S  string  `json:"S"`
	I  int64   `json:"i"`
	X  string  `json:"x"`
	P  float64 `json:"p"`
	Sz int64   `json:"s"`
	Z  string  `json:"z"`
	Ts string  `json:"t"`
}

type rawBar struct {
	T  string  `json:"T"`
	S  string  `json:"S"`
	O  float64 `json:"o"`
	H  float64 `json:"h"`
	L  float64 `json:"l"`
	C  float64 `json:"c"`
	V  int64   `json:"v"`
	Ts string  `json:"t"`
	N  int     `json:"n"`
	VW float64 `json:"vw"`
}

func parseQuoteToTick(q rawQuote) model.MarketTick {
	ts, _ := time.Parse(time.RFC3339Nano, q.Ts)
	return model.MarketTick{
		Symbol:    q.S,
		Timestamp: ts,
		Bid:       decimal.NewFromFloat(q.Bp),
		Ask:       decimal.NewFromFloat(q.Ap),
		Last:      decimal.NewFromFloat(q.Bp).Add(decimal.NewFromFloat(q.Ap)).Div(decimal.NewFromInt(2)),
		Volume:    0, // quotes don't carry cumulative volume
		Source:    "alpaca",
	}
}

func parseTradeToTrade(t rawTrade) model.Trade {
	ts, _ := time.Parse(time.RFC3339Nano, t.Ts)
	return model.Trade{
		Symbol:    t.S,
		Timestamp: ts,
		Price:     decimal.NewFromFloat(t.P),
		Size:      t.Sz,
		Exchange:  t.X,
		ID:        "",
		Source:    "alpaca",
	}
}

func parseTradeToTick(t rawTrade) model.MarketTick {
	ts, _ := time.Parse(time.RFC3339Nano, t.Ts)
	return model.MarketTick{
		Symbol:    t.S,
		Timestamp: ts,
		Last:      decimal.NewFromFloat(t.P),
		Volume:    t.Sz,
		Source:    "alpaca",
	}
}

func parseBarToBar(b rawBar) model.Bar {
	ts, _ := time.Parse(time.RFC3339Nano, b.Ts)
	return model.Bar{
		Symbol:     b.S,
		Timestamp:  ts,
		Open:       decimal.NewFromFloat(b.O),
		High:       decimal.NewFromFloat(b.H),
		Low:        decimal.NewFromFloat(b.L),
		Close:      decimal.NewFromFloat(b.C),
		Volume:     b.V,
		TradeCount: b.N,
		VWAP:       decimal.NewFromFloat(b.VW),
		Source:     "alpaca",
	}
}
