package signal

import (
	"context"
	"encoding/json"

	"trading-platform/oms/internal/model"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Receiver struct {
	rdb       *redis.Client
	log       *zap.SugaredLogger
	signalCh  chan *model.Signal
}

func NewReceiver(rdb *redis.Client, log *zap.SugaredLogger) *Receiver {
	return &Receiver{
		rdb:      rdb,
		log:      log,
		signalCh: make(chan *model.Signal, 100),
	}
}

func (r *Receiver) Signals() <-chan *model.Signal {
	return r.signalCh
}

func (r *Receiver) Start(ctx context.Context) {
	sub := r.rdb.Subscribe(ctx, "strategy:signals")
	ch := sub.Channel()

	go func() {
		defer sub.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-ch:
				var sig model.Signal
				if err := json.Unmarshal([]byte(msg.Payload), &sig); err != nil {
					r.log.Warnw("Failed to parse signal", "error", err)
					continue
				}
				r.log.Infow("Signal received", "signal_id", sig.SignalID, "symbol", sig.Symbol, "side", sig.Side)
				select {
				case r.signalCh <- &sig:
				default:
					r.log.Warn("Signal channel full, dropping signal")
				}
			}
		}
	}()
}
