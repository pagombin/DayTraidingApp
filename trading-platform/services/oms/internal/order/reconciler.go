package order

import (
	"context"
	"time"

	"trading-platform/oms/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type Reconciler struct {
	pool *pgxpool.Pool
	log  *zap.SugaredLogger
}

func NewReconciler(pool *pgxpool.Pool, log *zap.SugaredLogger) *Reconciler {
	return &Reconciler{pool: pool, log: log}
}

func (r *Reconciler) ReconcileOnStartup(ctx context.Context, brokerOrders []*model.Order) error {
	r.log.Info("Starting order reconciliation with broker")

	dbCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Find all non-terminal orders in DB
	rows, err := r.pool.Query(dbCtx,
		`SELECT order_id, broker_order_id, status FROM orders
		 WHERE status NOT IN ('FILLED', 'CANCELLED', 'REJECTED', 'EXPIRED', 'FAILED')`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type dbOrder struct {
		OrderID       string
		BrokerOrderID string
		Status        string
	}
	var openDBOrders []dbOrder
	for rows.Next() {
		var o dbOrder
		if err := rows.Scan(&o.OrderID, &o.BrokerOrderID, &o.Status); err != nil {
			continue
		}
		openDBOrders = append(openDBOrders, o)
	}

	brokerMap := make(map[string]*model.Order)
	for _, bo := range brokerOrders {
		brokerMap[bo.BrokerOrderID] = bo
	}

	reconciled := 0
	for _, dbo := range openDBOrders {
		if dbo.BrokerOrderID == "" {
			continue
		}
		bo, exists := brokerMap[dbo.BrokerOrderID]
		if !exists {
			// Order not found at broker - mark as failed
			_, _ = r.pool.Exec(dbCtx,
				"UPDATE orders SET status = 'FAILED', updated_at = NOW() WHERE order_id = $1",
				dbo.OrderID)
			reconciled++
			continue
		}
		if string(bo.State) != dbo.Status {
			_, _ = r.pool.Exec(dbCtx,
				"UPDATE orders SET status=$1, filled_quantity=$2, avg_fill_price=$3, updated_at=NOW() WHERE order_id=$4",
				string(bo.State), bo.FilledQty, bo.AvgFillPrice, dbo.OrderID)
			reconciled++
		}
	}

	r.log.Infow("Order reconciliation complete",
		"open_db_orders", len(openDBOrders),
		"broker_orders", len(brokerOrders),
		"reconciled", reconciled)

	return nil
}
