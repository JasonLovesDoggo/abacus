package utils

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

// SSE drop counters. Read by the Prometheus gauge funcs in prometheus.go.
var (
	SSEClientDrops  atomic.Int64
	SSEMessageDrops atomic.Int64
)

// RedisTimingHook records per-command latency into the Prometheus histogram
// registered in utils.Prom. Used for both clients (main + ratelimit pools)
// via Client.AddHook in main.go.
type RedisTimingHook struct{ Pool string }

func (h RedisTimingHook) DialHook(next redis.DialHook) redis.DialHook {
	return next
}

func (h RedisTimingHook) ProcessHook(next redis.ProcessHook) redis.ProcessHook {
	return func(ctx context.Context, cmd redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmd)
		recordCmd(h.Pool, cmd.Name(), start, err)
		return err
	}
}

func (h RedisTimingHook) ProcessPipelineHook(next redis.ProcessPipelineHook) redis.ProcessPipelineHook {
	return func(ctx context.Context, cmds []redis.Cmder) error {
		start := time.Now()
		err := next(ctx, cmds)
		recordCmd(h.Pool, "pipeline", start, err)
		return err
	}
}

func recordCmd(pool, name string, start time.Time, err error) {
	if !Prom.registered {
		return
	}
	dur := time.Since(start)
	Prom.redisCmdDuration.WithLabelValues(pool, name).Observe(dur.Seconds())
	if err != nil && !errors.Is(err, redis.Nil) {
		Prom.redisCmdErrors.WithLabelValues(pool, name).Inc()
	}
}
