package normalizer

import (
	"strings"

	"trading-platform/market-data/internal/model"
)

// NormalizeTick ensures consistent formatting for a tick.
func NormalizeTick(tick model.MarketTick) model.MarketTick {
	tick.Symbol = strings.ToUpper(strings.TrimSpace(tick.Symbol))
	return tick
}

// NormalizeBar ensures consistent formatting for a bar.
func NormalizeBar(bar model.Bar) model.Bar {
	bar.Symbol = strings.ToUpper(strings.TrimSpace(bar.Symbol))
	return bar
}

// NormalizeTrade ensures consistent formatting for a trade.
func NormalizeTrade(trade model.Trade) model.Trade {
	trade.Symbol = strings.ToUpper(strings.TrimSpace(trade.Symbol))
	return trade
}
