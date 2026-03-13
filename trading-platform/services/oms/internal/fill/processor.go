package fill

import (
	"context"
	"encoding/json"
	"time"

	"trading-platform/oms/internal/broker"
	"trading-platform/oms/internal/model"
	"trading-platform/oms/internal/order"
	"trading-platform/oms/pkg/audit"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"
)

type Processor struct {
	pool         *pgxpool.Pool
	rdb          *redis.Client
	stateMachine *order.StateMachine
	persistence  *order.Persistence
	auditLogger  *audit.Logger
	log          *zap.SugaredLogger
}

func NewProcessor(pool *pgxpool.Pool, rdb *redis.Client, sm *order.StateMachine, p *order.Persistence, al *audit.Logger, log *zap.SugaredLogger) *Processor {
	return &Processor{
		pool:         pool,
		rdb:          rdb,
		stateMachine: sm,
		persistence:  p,
		auditLogger:  al,
		log:          log,
	}
}

func (fp *Processor) ProcessFill(ctx context.Context, update broker.OrderUpdate) error {
	fp.log.Infow("Processing fill",
		"broker_order_id", update.BrokerOrderID,
		"event", update.Event,
		"qty", update.FilledQty,
		"price", update.FilledPrice.StringFixed(2))

	// Find the order by broker ID
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var orderID uuid.UUID
	var symbol, side, strategyID string
	var totalQty int
	err := fp.pool.QueryRow(dbCtx,
		"SELECT order_id, symbol, side, strategy_id, quantity FROM orders WHERE broker_order_id = $1",
		update.BrokerOrderID).Scan(&orderID, &symbol, &side, &strategyID, &totalQty)
	if err != nil {
		fp.log.Warnw("Order not found for fill", "broker_order_id", update.BrokerOrderID, "error", err)
		return err
	}

	ord, err := fp.persistence.GetOrder(ctx, orderID)
	if err != nil {
		return err
	}

	// Update filled quantities
	ord.FilledQty += update.FilledQty
	ord.AvgFillPrice = &update.FilledPrice
	ord.Commission = ord.Commission.Add(update.Commission)

	// Determine new state
	var newState model.OrderState
	if ord.FilledQty >= totalQty {
		newState = model.OrderFilled
	} else {
		newState = model.OrderPartialFill
	}

	if err := fp.stateMachine.Transition(ctx, ord, newState, update.Event); err != nil {
		fp.log.Errorw("Failed to transition order on fill", "error", err, "order_id", orderID)
		return err
	}

	// Update position
	fp.updatePosition(ctx, symbol, model.OrderSide(side), strategyID, update.FilledQty, update.FilledPrice)

	// Publish position update
	data := map[string]interface{}{
		"type":        "position_update",
		"symbol":      symbol,
		"side":        side,
		"filled_qty":  update.FilledQty,
		"fill_price":  update.FilledPrice.StringFixed(2),
		"strategy_id": strategyID,
	}
	dataJSON, _ := json.Marshal(data)
	fp.rdb.Publish(ctx, "oms:position_updates", string(dataJSON))

	// Audit log
	fp.auditLogger.Log(ctx, audit.Entry{
		Action:     "order_filled",
		EntityType: "order",
		EntityID:   orderID.String(),
		Details: map[string]interface{}{
			"symbol":     symbol,
			"side":       side,
			"filled_qty": update.FilledQty,
			"fill_price": update.FilledPrice.StringFixed(2),
			"commission": update.Commission.StringFixed(4),
		},
	})

	return nil
}

func (fp *Processor) updatePosition(ctx context.Context, symbol string, side model.OrderSide, strategyID string, qty int, price decimal.Decimal) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if side == model.OrderBuy {
		// Check for existing position
		var existingQty int
		var existingCost decimal.Decimal
		var posID string
		err := fp.pool.QueryRow(dbCtx,
			"SELECT position_id, quantity, avg_cost FROM positions WHERE symbol = $1 AND account_id = 'default'",
			symbol).Scan(&posID, &existingQty, &existingCost)

		if err != nil {
			// New position
			_, _ = fp.pool.Exec(dbCtx,
				`INSERT INTO positions (account_id, strategy_id, symbol, quantity, avg_cost, current_price, market_value, unrealized_pnl)
				 VALUES ('default', $1, $2, $3, $4, $5, $6, 0)`,
				strategyID, symbol, qty, price, price,
				price.Mul(decimal.NewFromInt(int64(qty))))
		} else {
			// Update existing
			totalCost := existingCost.Mul(decimal.NewFromInt(int64(existingQty))).Add(price.Mul(decimal.NewFromInt(int64(qty))))
			newQty := existingQty + qty
			newAvgCost := totalCost.Div(decimal.NewFromInt(int64(newQty)))
			_, _ = fp.pool.Exec(dbCtx,
				"UPDATE positions SET quantity=$1, avg_cost=$2, updated_at=NOW() WHERE position_id=$3",
				newQty, newAvgCost, posID)
		}
	} else {
		// Selling - reduce or close position
		var existingQty int
		var existingCost decimal.Decimal
		var posID string
		err := fp.pool.QueryRow(dbCtx,
			"SELECT position_id, quantity, avg_cost FROM positions WHERE symbol = $1 AND account_id = 'default'",
			symbol).Scan(&posID, &existingQty, &existingCost)

		if err == nil {
			newQty := existingQty - qty
			if newQty <= 0 {
				// Close position, record realized PnL
				realizedPnL := price.Sub(existingCost).Mul(decimal.NewFromInt(int64(qty)))
				_, _ = fp.pool.Exec(dbCtx, "DELETE FROM positions WHERE position_id = $1", posID)

				// Record in trade journal
				_, _ = fp.pool.Exec(dbCtx,
					`INSERT INTO trade_journal (symbol, side, quantity, entry_price, exit_price, realized_pnl, strategy_id, opened_at, closed_at)
					 VALUES ($1, 'sell', $2, $3, $4, $5, $6, NOW(), NOW())`,
					symbol, qty, existingCost, price, realizedPnL, strategyID)
			} else {
				_, _ = fp.pool.Exec(dbCtx,
					"UPDATE positions SET quantity=$1, updated_at=NOW() WHERE position_id=$2",
					newQty, posID)
			}
		}
	}
}

func (fp *Processor) StartFillListener(ctx context.Context, updateCh <-chan broker.OrderUpdate) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case update := <-updateCh:
				if err := fp.ProcessFill(ctx, update); err != nil {
					fp.log.Errorw("Failed to process fill", "error", err, "broker_order_id", update.BrokerOrderID)
				}
			}
		}
	}()
}
