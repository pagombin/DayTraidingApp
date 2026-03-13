package killswitch

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"trading-platform/oms/internal/broker"
	"trading-platform/oms/internal/model"
	"trading-platform/oms/pkg/audit"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type KillSwitch struct {
	brokerAdapter broker.BrokerAdapter
	rdb           *redis.Client
	auditLogger   *audit.Logger
	log           *zap.SugaredLogger
	active        atomic.Bool
}

func NewKillSwitch(ba broker.BrokerAdapter, rdb *redis.Client, al *audit.Logger, log *zap.SugaredLogger) *KillSwitch {
	ks := &KillSwitch{
		brokerAdapter: ba,
		rdb:           rdb,
		auditLogger:   al,
		log:           log,
	}

	// Check if kill switch was active before restart
	val, err := rdb.Get(context.Background(), "killswitch:active").Result()
	if err == nil && val == "true" {
		ks.active.Store(true)
		log.Warn("Kill switch was active before restart, remaining active")
	}

	return ks
}

func (ks *KillSwitch) Activate(ctx context.Context, reason string) error {
	if ks.active.Load() {
		return fmt.Errorf("kill switch is already active")
	}

	ks.log.Warnw("KILL SWITCH ACTIVATING", "reason", reason)

	// Step 1: Set Redis flag
	ks.rdb.Set(ctx, "killswitch:active", "true", 0)
	ks.active.Store(true)

	// Step 2: Cancel all open orders
	orders, err := ks.brokerAdapter.GetAllOrders(ctx)
	if err != nil {
		ks.log.Errorw("Failed to get open orders during kill switch", "error", err)
	} else {
		for _, order := range orders {
			if order.BrokerOrderID != "" {
				if err := ks.brokerAdapter.CancelOrder(ctx, order.BrokerOrderID); err != nil {
					ks.log.Errorw("Failed to cancel order during kill switch",
						"broker_order_id", order.BrokerOrderID, "error", err)
				}
			}
		}
		ks.log.Infow("Cancelled open orders", "count", len(orders))
	}

	// Step 3: Flatten all positions
	positions, err := ks.brokerAdapter.GetPositions(ctx)
	if err != nil {
		ks.log.Errorw("Failed to get positions during kill switch", "error", err)
	} else {
		for _, pos := range positions {
			if pos.Quantity == 0 {
				continue
			}
			closeOrder := &model.Order{
				Symbol:      pos.Symbol,
				OrderType:   model.OrderTypeMarket,
				TimeInForce: model.TIFDay,
				StrategyID:  "killswitch",
				AccountID:   "default",
			}
			if pos.Quantity > 0 {
				closeOrder.Side = model.OrderSell
				closeOrder.Quantity = pos.Quantity
			} else {
				closeOrder.Side = model.OrderBuy
				closeOrder.Quantity = -pos.Quantity
			}
			if _, err := ks.brokerAdapter.SubmitOrder(ctx, closeOrder); err != nil {
				ks.log.Errorw("Failed to flatten position",
					"symbol", pos.Symbol, "quantity", pos.Quantity, "error", err)
			} else {
				ks.log.Infow("Flattening position", "symbol", pos.Symbol, "quantity", pos.Quantity)
			}
		}
		ks.log.Infow("Flatten orders submitted", "count", len(positions))
	}

	// Step 4: Publish event
	data, _ := json.Marshal(map[string]interface{}{
		"type":      "killswitch",
		"active":    true,
		"reason":    reason,
		"timestamp": time.Now().Format(time.RFC3339),
	})
	ks.rdb.Publish(ctx, "killswitch:activated", string(data))
	ks.rdb.Publish(ctx, "oms:killswitch", string(data))

	// Step 5: Audit log
	ks.auditLogger.Log(ctx, audit.Entry{
		Action:     "killswitch_activated",
		EntityType: "system",
		EntityID:   "killswitch",
		Details: map[string]interface{}{
			"reason": reason,
		},
	})

	ks.log.Warn("KILL SWITCH ACTIVATED — All trading halted")
	return nil
}

func (ks *KillSwitch) Deactivate(ctx context.Context, confirmation string) error {
	if confirmation != "CONFIRM" {
		return fmt.Errorf("deactivation requires confirmation string 'CONFIRM'")
	}
	if !ks.active.Load() {
		return fmt.Errorf("kill switch is not active")
	}

	ks.rdb.Del(ctx, "killswitch:active")
	ks.active.Store(false)

	data, _ := json.Marshal(map[string]interface{}{
		"type":      "killswitch",
		"active":    false,
		"timestamp": time.Now().Format(time.RFC3339),
	})
	ks.rdb.Publish(ctx, "killswitch:deactivated", string(data))
	ks.rdb.Publish(ctx, "oms:killswitch", string(data))

	ks.auditLogger.Log(ctx, audit.Entry{
		Action:     "killswitch_deactivated",
		EntityType: "system",
		EntityID:   "killswitch",
		Details:    map[string]interface{}{},
	})

	ks.log.Info("Kill switch deactivated — Trading resumed")
	return nil
}

func (ks *KillSwitch) IsActive() bool {
	return ks.active.Load()
}

