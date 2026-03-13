package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics tracks operational metrics for the market data service.
// Prometheus integration can be added later; for now we expose metrics via the status API.
type Metrics struct {
	TotalTicks          atomic.Int64
	TicksPerSecond      atomic.Int64
	ValidationRejected  atomic.Int64
	RedisPublishErrors  atomic.Int64
	DBWriteErrors       atomic.Int64
	ReconnectionCount   atomic.Int64
	ActiveSymbols       atomic.Int64
	StaleSymbols        atomic.Int64
	DBBufferSize        atomic.Int64

	lastTickCount int64
	mu            sync.Mutex
	done          chan struct{}
}

func New() *Metrics {
	return &Metrics{
		done: make(chan struct{}),
	}
}

// StartRateTracker updates ticks-per-second every second.
func (m *Metrics) StartRateTracker() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-m.done:
				return
			case <-ticker.C:
				current := m.TotalTicks.Load()
				m.TicksPerSecond.Store(current - m.lastTickCount)
				m.lastTickCount = current
			}
		}
	}()
}

func (m *Metrics) Stop() {
	close(m.done)
}

// Snapshot returns current metrics as a map for API responses.
func (m *Metrics) Snapshot() map[string]interface{} {
	return map[string]interface{}{
		"total_ticks":          m.TotalTicks.Load(),
		"ticks_per_second":     m.TicksPerSecond.Load(),
		"validation_rejected":  m.ValidationRejected.Load(),
		"redis_publish_errors": m.RedisPublishErrors.Load(),
		"db_write_errors":      m.DBWriteErrors.Load(),
		"reconnection_count":   m.ReconnectionCount.Load(),
		"active_symbols":       m.ActiveSymbols.Load(),
		"stale_symbols":        m.StaleSymbols.Load(),
		"db_buffer_size":       m.DBBufferSize.Load(),
	}
}
