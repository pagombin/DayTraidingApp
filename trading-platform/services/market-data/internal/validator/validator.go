package validator

import (
	"fmt"
	"time"

	"github.com/shopspring/decimal"
	"trading-platform/market-data/internal/model"
)

type ValidationError struct {
	Symbol string
	Reason string
	Tick   model.MarketTick
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("invalid tick for %s: %s", e.Symbol, e.Reason)
}

// ValidateTick checks a tick for obvious errors.
func ValidateTick(tick model.MarketTick, lastKnown *model.MarketTick) error {
	if tick.Last.LessThanOrEqual(decimal.Zero) {
		return ValidationError{tick.Symbol, "last price <= 0", tick}
	}

	if tick.Bid.LessThan(decimal.Zero) {
		return ValidationError{tick.Symbol, "bid price < 0", tick}
	}

	if tick.Ask.LessThan(decimal.Zero) {
		return ValidationError{tick.Symbol, "ask price < 0", tick}
	}

	// Bid must not exceed Ask (crossed market)
	if tick.Bid.GreaterThan(decimal.Zero) && tick.Ask.GreaterThan(decimal.Zero) {
		if tick.Bid.GreaterThan(tick.Ask) {
			return ValidationError{tick.Symbol, "bid > ask (crossed market)", tick}
		}
	}

	if tick.Volume < 0 {
		return ValidationError{tick.Symbol, "negative volume", tick}
	}

	// Timestamp must not be in the future (allow 5s clock skew)
	if tick.Timestamp.After(time.Now().Add(5 * time.Second)) {
		return ValidationError{tick.Symbol, "timestamp in the future", tick}
	}

	// Price deviation check (>50% move is suspicious)
	if lastKnown != nil && lastKnown.Last.GreaterThan(decimal.Zero) {
		changeRatio := tick.Last.Sub(lastKnown.Last).Abs().Div(lastKnown.Last)
		threshold := decimal.NewFromFloat(0.50)
		if changeRatio.GreaterThan(threshold) {
			return ValidationError{
				tick.Symbol,
				fmt.Sprintf("price moved %.1f%% from last known (%.2f -> %.2f)",
					changeRatio.Mul(decimal.NewFromInt(100)).InexactFloat64(),
					lastKnown.Last.InexactFloat64(),
					tick.Last.InexactFloat64()),
				tick,
			}
		}
	}

	return nil
}
