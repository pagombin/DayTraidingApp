package publisher

import (
	"context"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"trading-platform/market-data/internal/model"
)

type DBWriter struct {
	pool          *pgxpool.Pool
	buffer        []model.MarketTick
	bufferMu      sync.Mutex
	maxBuffer     int
	flushSize     int
	flushInterval time.Duration
	logger        *zap.Logger
	done          chan struct{}
}

func NewDBWriter(pool *pgxpool.Pool, logger *zap.Logger) *DBWriter {
	return &DBWriter{
		pool:          pool,
		maxBuffer:     100000,
		flushSize:     5000,
		flushInterval: 5 * time.Second,
		logger:        logger,
		done:          make(chan struct{}),
	}
}

// AddTick adds a tick to the write buffer. Non-blocking.
func (w *DBWriter) AddTick(tick model.MarketTick) {
	w.bufferMu.Lock()
	defer w.bufferMu.Unlock()

	if len(w.buffer) >= w.maxBuffer {
		// Drop oldest ticks
		dropped := len(w.buffer) - w.maxBuffer + 1
		w.buffer = w.buffer[dropped:]
		w.logger.Error("DB write buffer full, dropped ticks", zap.Int("dropped", dropped))
	}

	w.buffer = append(w.buffer, tick)
}

// Start begins the background flush loop.
func (w *DBWriter) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.flushInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				w.flushAll(context.Background())
				return
			case <-w.done:
				w.flushAll(context.Background())
				return
			case <-ticker.C:
				w.bufferMu.Lock()
				size := len(w.buffer)
				w.bufferMu.Unlock()
				if size > 0 {
					if err := w.flush(ctx); err != nil {
						w.logger.Warn("DB flush failed", zap.Error(err), zap.Int("buffer_size", size))
					}
				}
			}
		}
	}()

	// Also flush when buffer hits flushSize
	go func() {
		check := time.NewTicker(100 * time.Millisecond)
		defer check.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.done:
				return
			case <-check.C:
				w.bufferMu.Lock()
				size := len(w.buffer)
				w.bufferMu.Unlock()
				if size >= w.flushSize {
					if err := w.flush(ctx); err != nil {
						w.logger.Warn("DB flush failed (size trigger)", zap.Error(err))
					}
				}
			}
		}
	}()
}

func (w *DBWriter) Stop() {
	close(w.done)
}

// flush writes the current buffer to TimescaleDB using COPY.
func (w *DBWriter) flush(ctx context.Context) error {
	w.bufferMu.Lock()
	if len(w.buffer) == 0 {
		w.bufferMu.Unlock()
		return nil
	}
	ticks := make([]model.MarketTick, len(w.buffer))
	copy(ticks, w.buffer)
	w.buffer = w.buffer[:0]
	w.bufferMu.Unlock()

	start := time.Now()

	rows := make([][]interface{}, 0, len(ticks))
	for _, t := range ticks {
		rows = append(rows, []interface{}{
			t.Symbol,
			t.Timestamp,
			t.Bid.String(),
			t.Ask.String(),
			t.Last.String(),
			t.Volume,
			t.Source,
		})
	}

	_, err := w.pool.CopyFrom(
		ctx,
		pgx.Identifier{"market_ticks"},
		[]string{"symbol", "timestamp", "bid", "ask", "last_price", "volume", "source"},
		pgx.CopyFromRows(rows),
	)

	if err != nil {
		// Put ticks back in buffer
		w.bufferMu.Lock()
		w.buffer = append(ticks, w.buffer...)
		if len(w.buffer) > w.maxBuffer {
			w.buffer = w.buffer[:w.maxBuffer]
		}
		w.bufferMu.Unlock()
		return err
	}

	w.logger.Debug("Flushed ticks to DB",
		zap.Int("count", len(ticks)),
		zap.Duration("duration", time.Since(start)),
	)

	return nil
}

func (w *DBWriter) flushAll(ctx context.Context) {
	if err := w.flush(ctx); err != nil {
		w.logger.Error("Final flush failed", zap.Error(err))
	}
}

// BufferSize returns the current buffer size.
func (w *DBWriter) BufferSize() int {
	w.bufferMu.Lock()
	defer w.bufferMu.Unlock()
	return len(w.buffer)
}
