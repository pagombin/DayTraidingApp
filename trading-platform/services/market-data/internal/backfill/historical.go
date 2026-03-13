package backfill

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"trading-platform/market-data/internal/model"
	"trading-platform/market-data/internal/provider"
	"trading-platform/market-data/internal/publisher"
)

type HistoricalBackfiller struct {
	provider    provider.DataProvider
	dbWriter    *publisher.DBWriter
	pool        *pgxpool.Pool
	rateLimiter *rate.Limiter
	logger      *zap.Logger
}

func NewHistoricalBackfiller(prov provider.DataProvider, dbWriter *publisher.DBWriter, pool *pgxpool.Pool, logger *zap.Logger) *HistoricalBackfiller {
	return &HistoricalBackfiller{
		provider:    prov,
		dbWriter:    dbWriter,
		pool:        pool,
		rateLimiter: rate.NewLimiter(rate.Every(300*time.Millisecond), 1), // ~200 req/min
		logger:      logger,
	}
}

// BackfillSymbol fetches and stores historical bars for a single symbol.
func (h *HistoricalBackfiller) BackfillSymbol(ctx context.Context, symbol string, start, end time.Time, timeframe string) (int, error) {
	if err := h.rateLimiter.Wait(ctx); err != nil {
		return 0, err
	}

	bars, err := h.provider.FetchHistoricalBars(ctx, symbol, start, end, timeframe)
	if err != nil {
		return 0, err
	}

	for _, bar := range bars {
		tick := model.MarketTick{
			Symbol:    bar.Symbol,
			Timestamp: bar.Timestamp,
			Bid:       bar.Close,
			Ask:       bar.Close,
			Last:      bar.Close,
			Volume:    bar.Volume,
			Source:    bar.Source + "-backfill",
		}
		h.dbWriter.AddTick(tick)
	}

	h.logger.Info("Backfilled symbol",
		zap.String("symbol", symbol),
		zap.Int("bars", len(bars)),
		zap.Time("start", start),
		zap.Time("end", end),
	)

	return len(bars), nil
}

// BackfillAll fetches historical data for all watchlist symbols.
func (h *HistoricalBackfiller) BackfillAll(ctx context.Context, symbols []string) error {
	for _, symbol := range symbols {
		var lastTS time.Time
		err := h.pool.QueryRow(ctx,
			"SELECT COALESCE(MAX(timestamp), '2000-01-01'::timestamptz) FROM market_ticks WHERE symbol = $1",
			symbol).Scan(&lastTS)
		if err != nil {
			h.logger.Warn("Failed to get last timestamp", zap.String("symbol", symbol), zap.Error(err))
			continue
		}

		if time.Since(lastTS) > time.Minute {
			count, err := h.BackfillSymbol(ctx, symbol, lastTS, time.Now(), "1Min")
			if err != nil {
				h.logger.Warn("Backfill failed", zap.String("symbol", symbol), zap.Error(err))
				continue
			}
			if count > 0 {
				h.logger.Info("Gap filled", zap.String("symbol", symbol), zap.Int("bars", count))
			}
		}
	}
	return nil
}
