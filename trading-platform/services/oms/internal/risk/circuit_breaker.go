package risk

import (
	"context"
	"sync/atomic"

	"trading-platform/oms/internal/config"
	"trading-platform/oms/internal/model"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type CircuitBreakerState string

const (
	CBNormal    CircuitBreakerState = "normal"
	CBWarning   CircuitBreakerState = "warning"
	CBThrottle  CircuitBreakerState = "throttle"
	CBHalt      CircuitBreakerState = "halt"
	CBEmergency CircuitBreakerState = "emergency"
)

type CircuitBreaker struct {
	cfg   *config.Config
	state atomic.Value
	rdb   *redis.Client
	log   *zap.SugaredLogger
}

func NewCircuitBreaker(cfg *config.Config, rdb *redis.Client, log *zap.SugaredLogger) *CircuitBreaker {
	cb := &CircuitBreaker{cfg: cfg, rdb: rdb, log: log}
	cb.state.Store(CBNormal)
	return cb
}

func (cb *CircuitBreaker) GetState() CircuitBreakerState {
	return cb.state.Load().(CircuitBreakerState)
}

func (cb *CircuitBreaker) Evaluate(portfolio *model.PortfolioSnapshot) CircuitBreakerState {
	if portfolio == nil {
		return CBNormal
	}

	riskCfg := cb.cfg.GetRisk()
	if !riskCfg.CircuitBreakerEnabled {
		return CBNormal
	}

	dailyLoss := portfolio.DailyPnL.Neg()
	if dailyLoss.LessThanOrEqual(decimal.Zero) {
		cb.setState(CBNormal)
		return CBNormal
	}

	maxLoss := riskCfg.MaxDailyLoss
	pct := dailyLoss.Div(maxLoss).Mul(decimal.NewFromInt(100))

	oldState := cb.GetState()
	var newState CircuitBreakerState

	switch {
	case pct.GreaterThanOrEqual(decimal.NewFromInt(150)):
		newState = CBEmergency
	case pct.GreaterThanOrEqual(decimal.NewFromInt(100)):
		newState = CBHalt
	case pct.GreaterThanOrEqual(decimal.NewFromInt(75)):
		newState = CBThrottle
	case pct.GreaterThanOrEqual(decimal.NewFromInt(50)):
		newState = CBWarning
	default:
		newState = CBNormal
	}

	if newState != oldState {
		cb.setState(newState)
		cb.publishStateChange(context.Background(), oldState, newState, dailyLoss, maxLoss)
	}

	return newState
}

func (cb *CircuitBreaker) setState(state CircuitBreakerState) {
	cb.state.Store(state)
}

func (cb *CircuitBreaker) publishStateChange(ctx context.Context, oldState, newState CircuitBreakerState, loss, limit decimal.Decimal) {
	var message string
	var level string

	switch newState {
	case CBWarning:
		level = "warning"
		message = "You've lost $" + loss.StringFixed(2) + " today (50% of your $" + limit.StringFixed(0) + " daily limit). Strategies are still running but on alert."
	case CBThrottle:
		level = "warning"
		message = "You've lost $" + loss.StringFixed(2) + " today. Position sizes have been automatically reduced by 50% to protect your account."
	case CBHalt:
		level = "critical"
		message = "Daily loss limit of $" + limit.StringFixed(0) + " reached. All trading has been paused. Your existing positions are unchanged."
	case CBEmergency:
		level = "critical"
		message = "EMERGENCY: Losses have exceeded $" + loss.StringFixed(2) + ". All positions have been closed to protect your account."
	case CBNormal:
		level = "info"
		message = "Circuit breaker returned to normal."
	}

	cb.log.Warnw("Circuit breaker state change",
		"old_state", string(oldState),
		"new_state", string(newState),
		"daily_loss", loss.StringFixed(2),
		"limit", limit.StringFixed(0))

	data := map[string]interface{}{
		"type":      "risk_alert",
		"level":     level,
		"state":     string(newState),
		"message":   message,
		"daily_loss": loss.StringFixed(2),
		"limit":      limit.StringFixed(0),
	}
	cb.rdb.Publish(ctx, "oms:risk_alerts", mustJSON(data))
}

func (cb *CircuitBreaker) IsHalted() bool {
	state := cb.GetState()
	return state == CBHalt || state == CBEmergency
}
