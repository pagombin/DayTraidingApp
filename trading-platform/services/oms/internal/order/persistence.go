package order

import (
	"context"
	"encoding/json"
	"time"

	"trading-platform/oms/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Persistence struct {
	pool *pgxpool.Pool
	log  *zap.SugaredLogger
}

func NewPersistence(pool *pgxpool.Pool, log *zap.SugaredLogger) *Persistence {
	return &Persistence{pool: pool, log: log}
}

func (p *Persistence) CreateOrder(ctx context.Context, order *model.Order) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	metadata, _ := json.Marshal(order.Metadata)

	_, err := p.pool.Exec(dbCtx,
		`INSERT INTO orders (order_id, strategy_id, account_id, symbol, side, direction, order_type, quantity,
		 limit_price, stop_price, trail_percent, time_in_force, status, signal_id, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		order.OrderID, order.StrategyID, order.AccountID, order.Symbol,
		string(order.Side), string(order.Side), string(order.OrderType), order.Quantity,
		order.LimitPrice, order.StopPrice, order.TrailPercent,
		string(order.TimeInForce), string(order.State), order.SignalID,
		metadata, order.CreatedAt, order.UpdatedAt)
	return err
}

func (p *Persistence) SaveOrderState(ctx context.Context, order *model.Order) error {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := p.pool.Exec(dbCtx,
		`UPDATE orders SET status=$1, filled_quantity=$2, avg_fill_price=$3, commission=$4,
		 broker_order_id=$5, rejection_reason=$6, submitted_at=$7, filled_at=$8, updated_at=$9
		 WHERE order_id=$10`,
		string(order.State), order.FilledQty, order.AvgFillPrice, order.Commission,
		order.BrokerOrderID, order.RejectionReason,
		order.SubmittedAt, order.FilledAt, order.UpdatedAt, order.OrderID)
	return err
}

func (p *Persistence) LogStateTransition(ctx context.Context, orderID uuid.UUID, oldState, newState model.OrderState, reason string) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	_, err := p.pool.Exec(dbCtx,
		"INSERT INTO order_state_log (order_id, old_state, new_state, reason) VALUES ($1, $2, $3, $4)",
		orderID, string(oldState), string(newState), reason)
	if err != nil {
		p.log.Warnw("Failed to log state transition", "error", err, "order_id", orderID)
	}
}

func (p *Persistence) GetOrder(ctx context.Context, orderID uuid.UUID) (*model.Order, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	order := &model.Order{}
	var side, orderType, state, tif string
	err := p.pool.QueryRow(dbCtx,
		`SELECT order_id, strategy_id, account_id, symbol, side, order_type, quantity,
		 limit_price, stop_price, status, broker_order_id, filled_quantity, avg_fill_price,
		 commission, signal_id, rejection_reason, time_in_force, created_at, updated_at,
		 submitted_at, filled_at
		 FROM orders WHERE order_id = $1`, orderID).
		Scan(&order.OrderID, &order.StrategyID, &order.AccountID, &order.Symbol,
			&side, &orderType, &order.Quantity,
			&order.LimitPrice, &order.StopPrice, &state, &order.BrokerOrderID,
			&order.FilledQty, &order.AvgFillPrice, &order.Commission,
			&order.SignalID, &order.RejectionReason, &tif,
			&order.CreatedAt, &order.UpdatedAt, &order.SubmittedAt, &order.FilledAt)

	if err != nil {
		return nil, err
	}

	order.Side = model.OrderSide(side)
	order.OrderType = model.OrderType(orderType)
	order.State = model.OrderState(state)
	order.TimeInForce = model.TimeInForce(tif)
	return order, nil
}

func (p *Persistence) ListOrders(ctx context.Context, status string, limit int) ([]*model.Order, error) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	query := `SELECT order_id, strategy_id, account_id, symbol, side, order_type, quantity,
		 limit_price, stop_price, status, broker_order_id, filled_quantity, avg_fill_price,
		 commission, signal_id, rejection_reason, time_in_force, created_at, updated_at,
		 submitted_at, filled_at FROM orders`
	args := []interface{}{}

	if status != "" {
		query += " WHERE status = $1"
		args = append(args, status)
	}
	query += " ORDER BY created_at DESC"
	if limit > 0 {
		if len(args) > 0 {
			query += " LIMIT $2"
		} else {
			query += " LIMIT $1"
		}
		args = append(args, limit)
	}

	rows, err := p.pool.Query(dbCtx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*model.Order
	for rows.Next() {
		order := &model.Order{}
		var side, orderType, state, tif string
		if err := rows.Scan(&order.OrderID, &order.StrategyID, &order.AccountID, &order.Symbol,
			&side, &orderType, &order.Quantity,
			&order.LimitPrice, &order.StopPrice, &state, &order.BrokerOrderID,
			&order.FilledQty, &order.AvgFillPrice, &order.Commission,
			&order.SignalID, &order.RejectionReason, &tif,
			&order.CreatedAt, &order.UpdatedAt, &order.SubmittedAt, &order.FilledAt); err != nil {
			continue
		}
		order.Side = model.OrderSide(side)
		order.OrderType = model.OrderType(orderType)
		order.State = model.OrderState(state)
		order.TimeInForce = model.TimeInForce(tif)
		orders = append(orders, order)
	}
	return orders, nil
}

func (p *Persistence) SaveRiskCheckResults(ctx context.Context, orderID uuid.UUID, results []model.CheckResult) {
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	for _, r := range results {
		detailsJSON, _ := json.Marshal(r.Details)
		_, err := p.pool.Exec(dbCtx,
			"INSERT INTO risk_check_results (order_id, check_name, passed, reason, details) VALUES ($1, $2, $3, $4, $5)",
			orderID, r.Check, r.Passed, r.Reason, detailsJSON)
		if err != nil {
			p.log.Warnw("Failed to save risk check result", "error", err, "order_id", orderID, "check", r.Check)
		}
	}
}

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}
