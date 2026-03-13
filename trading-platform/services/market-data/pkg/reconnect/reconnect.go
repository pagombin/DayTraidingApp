package reconnect

import (
	"context"
	"fmt"
	"math"
	"time"

	"go.uber.org/zap"
)

type Config struct {
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Multiplier   float64
	MaxAttempts  int
	AlertAfter   time.Duration
	OnAlert      func(err error, duration time.Duration)
}

func DefaultConfig() Config {
	return Config{
		InitialDelay: 0,
		MaxDelay:     60 * time.Second,
		Multiplier:   2.0,
		MaxAttempts:  0,
		AlertAfter:   5 * time.Minute,
	}
}

// Run executes the connect function with exponential backoff.
// It blocks until success, context cancellation, or MaxAttempts is reached.
func Run(ctx context.Context, logger *zap.Logger, cfg Config, connectFn func() error) error {
	attempt := 0
	startTime := time.Now()
	alerted := false

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := connectFn()
		if err == nil {
			return nil
		}

		attempt++
		if cfg.MaxAttempts > 0 && attempt >= cfg.MaxAttempts {
			return fmt.Errorf("max reconnection attempts (%d) reached: %w", cfg.MaxAttempts, err)
		}

		delay := time.Duration(float64(cfg.InitialDelay) * math.Pow(cfg.Multiplier, float64(attempt-1)))
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
		if attempt == 1 && cfg.InitialDelay == 0 {
			delay = time.Second
		}

		logger.Warn("connection failed, retrying",
			zap.Error(err),
			zap.Int("attempt", attempt),
			zap.Duration("next_retry_in", delay),
			zap.Duration("total_downtime", time.Since(startTime)),
		)

		if !alerted && cfg.AlertAfter > 0 && time.Since(startTime) > cfg.AlertAfter {
			alerted = true
			if cfg.OnAlert != nil {
				cfg.OnAlert(err, time.Since(startTime))
			}
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
}
