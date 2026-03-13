package autonomy

import (
	"fmt"
	"sync"
	"time"

	"trading-platform/oms/internal/config"
	"trading-platform/oms/internal/model"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type AutonomyGate struct {
	cfg              *config.Config
	pendingApprovals map[uuid.UUID]*pendingOrder
	mu               sync.RWMutex
	log              *zap.SugaredLogger
}

type pendingOrder struct {
	Order     *model.Order
	ExpiresAt time.Time
}

func NewAutonomyGate(cfg *config.Config, log *zap.SugaredLogger) *AutonomyGate {
	g := &AutonomyGate{
		cfg:              cfg,
		pendingApprovals: make(map[uuid.UUID]*pendingOrder),
		log:              log,
	}
	go g.expireLoop()
	return g
}

func (g *AutonomyGate) Evaluate(signal *model.Signal) (proceed bool, state model.OrderState, reason string) {
	level := g.cfg.GetAutonomyLevel()

	switch level {
	case 0:
		return false, model.OrderRejected, "Autonomy Level 0 (Observer): signals are logged but no orders are created"
	case 1:
		return true, model.OrderAwaitingApproval, "Autonomy Level 1 (Advisor): order requires your approval"
	case 2:
		// Semi-auto: allow approved symbols only
		return true, model.OrderApproved, "Autonomy Level 2 (Semi-Auto): order approved for known symbol"
	case 3, 4:
		return true, model.OrderApproved, fmt.Sprintf("Autonomy Level %d: order auto-approved within risk limits", level)
	default:
		return false, model.OrderRejected, "Unknown autonomy level"
	}
}

func (g *AutonomyGate) AddPendingApproval(order *model.Order) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.pendingApprovals[order.OrderID] = &pendingOrder{
		Order:     order,
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}
}

func (g *AutonomyGate) GetPendingApprovals() []*model.Order {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var orders []*model.Order
	for _, po := range g.pendingApprovals {
		orders = append(orders, po.Order)
	}
	return orders
}

func (g *AutonomyGate) ApproveOrder(orderID uuid.UUID) (*model.Order, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	po, exists := g.pendingApprovals[orderID]
	if !exists {
		return nil, fmt.Errorf("order %s not found in pending approvals", orderID)
	}
	order := po.Order
	delete(g.pendingApprovals, orderID)
	return order, nil
}

func (g *AutonomyGate) RejectOrder(orderID uuid.UUID) (*model.Order, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	po, exists := g.pendingApprovals[orderID]
	if !exists {
		return nil, fmt.Errorf("order %s not found in pending approvals", orderID)
	}
	order := po.Order
	delete(g.pendingApprovals, orderID)
	return order, nil
}

func (g *AutonomyGate) expireLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		g.mu.Lock()
		now := time.Now()
		for id, po := range g.pendingApprovals {
			if now.After(po.ExpiresAt) {
				g.log.Infow("Pending approval expired", "order_id", id, "symbol", po.Order.Symbol)
				delete(g.pendingApprovals, id)
			}
		}
		g.mu.Unlock()
	}
}
