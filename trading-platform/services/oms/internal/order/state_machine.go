package order

import (
	"context"
	"fmt"
	"time"

	"trading-platform/oms/internal/model"
	"trading-platform/oms/pkg/audit"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

var ValidTransitions = map[model.OrderState][]model.OrderState{
	model.OrderPending:          {model.OrderApproved, model.OrderAwaitingApproval, model.OrderRejected},
	model.OrderAwaitingApproval: {model.OrderApproved, model.OrderCancelled, model.OrderExpired},
	model.OrderApproved:         {model.OrderSubmitted, model.OrderFailed},
	model.OrderSubmitted:        {model.OrderAccepted, model.OrderRejected, model.OrderFailed, model.OrderCancelled},
	model.OrderAccepted:         {model.OrderPartialFill, model.OrderFilled, model.OrderCancelled, model.OrderExpired},
	model.OrderPartialFill:      {model.OrderPartialFill, model.OrderFilled, model.OrderCancelled},
}

type StateMachine struct {
	persistence *Persistence
	rdb         *redis.Client
	auditLogger *audit.Logger
	log         *zap.SugaredLogger
}

func NewStateMachine(p *Persistence, rdb *redis.Client, al *audit.Logger, log *zap.SugaredLogger) *StateMachine {
	return &StateMachine{persistence: p, rdb: rdb, auditLogger: al, log: log}
}

func (sm *StateMachine) Transition(ctx context.Context, order *model.Order, newState model.OrderState, reason string) error {
	allowed := ValidTransitions[order.State]
	valid := false
	for _, s := range allowed {
		if s == newState {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("invalid state transition: %s -> %s for order %s", order.State, newState, order.OrderID)
	}

	oldState := order.State
	order.State = newState
	order.UpdatedAt = time.Now().UTC()

	if newState == model.OrderSubmitted {
		now := time.Now().UTC()
		order.SubmittedAt = &now
	}
	if newState == model.OrderFilled {
		now := time.Now().UTC()
		order.FilledAt = &now
	}

	if err := sm.persistence.SaveOrderState(ctx, order); err != nil {
		order.State = oldState
		return fmt.Errorf("CRITICAL: failed to persist order state %s->%s for %s: %w",
			oldState, newState, order.OrderID, err)
	}

	sm.persistence.LogStateTransition(ctx, order.OrderID, oldState, newState, reason)

	sm.publishOrderUpdate(ctx, order, oldState, newState, reason)

	sm.auditLogger.Log(ctx, audit.Entry{
		Action:     "order_state_transition",
		EntityType: "order",
		EntityID:   order.OrderID.String(),
		Details: map[string]interface{}{
			"old_state": string(oldState),
			"new_state": string(newState),
			"reason":    reason,
			"symbol":    order.Symbol,
			"side":      string(order.Side),
			"quantity":  order.Quantity,
			"strategy":  order.StrategyID,
		},
	})

	sm.log.Infow("Order state transition",
		"order_id", order.OrderID,
		"symbol", order.Symbol,
		"old_state", oldState,
		"new_state", newState,
		"reason", reason)

	return nil
}

func (sm *StateMachine) publishOrderUpdate(ctx context.Context, order *model.Order, oldState, newState model.OrderState, reason string) {
	data := map[string]interface{}{
		"type":      "order_update",
		"order_id":  order.OrderID.String(),
		"symbol":    order.Symbol,
		"side":      string(order.Side),
		"quantity":  order.Quantity,
		"old_state": string(oldState),
		"new_state": string(newState),
		"reason":    reason,
	}
	if order.AvgFillPrice != nil {
		data["avg_fill_price"] = order.AvgFillPrice.String()
	}

	sm.rdb.Publish(ctx, "oms:order_updates", mustJSON(data))
}
