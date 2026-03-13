package publisher

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"trading-platform/market-data/internal/model"
)

type pendingMessage struct {
	stream string
	values map[string]interface{}
}

type RedisPublisher struct {
	client    *redis.Client
	buffer    []pendingMessage
	bufferMu  sync.Mutex
	maxBuffer int
	logger    *zap.Logger
	done      chan struct{}
}

func NewRedisPublisher(client *redis.Client, logger *zap.Logger) *RedisPublisher {
	return &RedisPublisher{
		client:    client,
		maxBuffer: 10000,
		logger:    logger,
		done:      make(chan struct{}),
	}
}

// Start begins the pipeline flush loop.
func (p *RedisPublisher) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-p.done:
				return
			case <-ticker.C:
				p.flush(ctx)
			}
		}
	}()
}

func (p *RedisPublisher) Stop() {
	close(p.done)
}

// PublishTick publishes a MarketTick to Redis Stream and updates the latest cache.
func (p *RedisPublisher) PublishTick(ctx context.Context, tick model.MarketTick) error {
	tickJSON, err := json.Marshal(tick)
	if err != nil {
		return err
	}

	pipe := p.client.Pipeline()

	// XADD to stream with auto-trim
	streamKey := fmt.Sprintf("market:ticks:%s", tick.Symbol)
	pipe.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: 10000,
		Approx: true,
		Values: map[string]interface{}{
			"data": string(tickJSON),
		},
	})

	// SET latest quote cache
	latestKey := fmt.Sprintf("market:latest:%s", tick.Symbol)
	pipe.Set(ctx, latestKey, string(tickJSON), 5*time.Minute)

	// ZADD active symbols with timestamp score
	pipe.ZAdd(ctx, "market:active_symbols", redis.Z{
		Score:  float64(tick.Timestamp.Unix()),
		Member: tick.Symbol,
	})

	_, err = pipe.Exec(ctx)
	if err != nil {
		p.bufferMu.Lock()
		if len(p.buffer) < p.maxBuffer {
			p.buffer = append(p.buffer, pendingMessage{
				stream: streamKey,
				values: map[string]interface{}{"data": string(tickJSON)},
			})
		}
		p.bufferMu.Unlock()
		return fmt.Errorf("redis pipeline: %w", err)
	}

	return nil
}

// PublishBar publishes a completed Bar to the symbol's bar stream.
func (p *RedisPublisher) PublishBar(ctx context.Context, bar model.Bar) error {
	barJSON, err := json.Marshal(bar)
	if err != nil {
		return err
	}

	streamKey := fmt.Sprintf("market:bars:%s", bar.Symbol)
	return p.client.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		MaxLen: 1440, // one day of 1-min bars
		Approx: true,
		Values: map[string]interface{}{
			"data": string(barJSON),
		},
	}).Err()
}

// PublishStatus publishes a connection status change.
func (p *RedisPublisher) PublishStatus(ctx context.Context, status model.ServiceStatus) error {
	statusJSON, err := json.Marshal(status)
	if err != nil {
		return err
	}
	return p.client.Publish(ctx, "market:status", string(statusJSON)).Err()
}

func (p *RedisPublisher) flush(ctx context.Context) {
	p.bufferMu.Lock()
	if len(p.buffer) == 0 {
		p.bufferMu.Unlock()
		return
	}
	msgs := make([]pendingMessage, len(p.buffer))
	copy(msgs, p.buffer)
	p.buffer = p.buffer[:0]
	p.bufferMu.Unlock()

	pipe := p.client.Pipeline()
	for _, msg := range msgs {
		pipe.XAdd(ctx, &redis.XAddArgs{
			Stream: msg.stream,
			MaxLen: 10000,
			Approx: true,
			Values: msg.values,
		})
	}

	if _, err := pipe.Exec(ctx); err != nil {
		p.logger.Warn("Failed to flush buffered messages", zap.Error(err), zap.Int("count", len(msgs)))
		// Put them back
		p.bufferMu.Lock()
		p.buffer = append(msgs, p.buffer...)
		if len(p.buffer) > p.maxBuffer {
			dropped := len(p.buffer) - p.maxBuffer
			p.buffer = p.buffer[:p.maxBuffer]
			p.logger.Error("Dropped buffered messages", zap.Int("dropped", dropped))
		}
		p.bufferMu.Unlock()
	} else if len(msgs) > 0 {
		p.logger.Debug("Flushed buffered messages", zap.Int("count", len(msgs)))
	}
}
