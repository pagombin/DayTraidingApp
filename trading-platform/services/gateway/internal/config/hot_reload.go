package config

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Manager struct {
	redis *goredis.Client
	log   *zap.SugaredLogger
}

func NewManager(rdb *goredis.Client, log *zap.SugaredLogger) *Manager {
	return &Manager{redis: rdb, log: log}
}

func (m *Manager) SubscribeChanges(ctx context.Context) {
	sub := m.redis.Subscribe(ctx, "config:changed")
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-ch:
			m.log.Infow("Config changed via hot-reload",
				"channel", msg.Channel,
				"payload", msg.Payload,
			)
		}
	}
}

func (m *Manager) PublishChange(ctx context.Context, key string, value string) error {
	payload := `{"key":"` + key + `","value":` + value + `}`
	return m.redis.Publish(ctx, "config:changed", payload).Err()
}
