package metrics

import (
	"sync/atomic"
)

type Metrics struct {
	OrdersSubmitted  int64
	OrdersFilled     int64
	OrdersRejected   int64
	OrdersCancelled  int64
	RiskChecksRun    int64
	RiskChecksFailed int64
}

var Global = &Metrics{}

func (m *Metrics) IncrOrdersSubmitted()  { atomic.AddInt64(&m.OrdersSubmitted, 1) }
func (m *Metrics) IncrOrdersFilled()     { atomic.AddInt64(&m.OrdersFilled, 1) }
func (m *Metrics) IncrOrdersRejected()   { atomic.AddInt64(&m.OrdersRejected, 1) }
func (m *Metrics) IncrOrdersCancelled()  { atomic.AddInt64(&m.OrdersCancelled, 1) }
func (m *Metrics) IncrRiskChecksRun()    { atomic.AddInt64(&m.RiskChecksRun, 1) }
func (m *Metrics) IncrRiskChecksFailed() { atomic.AddInt64(&m.RiskChecksFailed, 1) }

func (m *Metrics) Snapshot() map[string]int64 {
	return map[string]int64{
		"orders_submitted":   atomic.LoadInt64(&m.OrdersSubmitted),
		"orders_filled":      atomic.LoadInt64(&m.OrdersFilled),
		"orders_rejected":    atomic.LoadInt64(&m.OrdersRejected),
		"orders_cancelled":   atomic.LoadInt64(&m.OrdersCancelled),
		"risk_checks_run":    atomic.LoadInt64(&m.RiskChecksRun),
		"risk_checks_failed": atomic.LoadInt64(&m.RiskChecksFailed),
	}
}
