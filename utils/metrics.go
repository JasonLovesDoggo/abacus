package utils

import (
	"context"
	"errors"
	"sync/atomic"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/getsentry/sentry-go/attribute"
	"github.com/redis/go-redis/v9"
)

var (
	meter           sentry.Meter
	SSEClientDrops  atomic.Int64
	SSEMessageDrops atomic.Int64
)

// InitMetrics wires the Sentry meter and starts the gauge ticker.
// Safe to call when Sentry is disabled (meter is a noop).
func InitMetrics(ctx context.Context, main, rl *redis.Client) {
	meter = sentry.NewMeter(ctx)
	go gaugeLoop(ctx, main, rl)
}

func gaugeLoop(ctx context.Context, main, rl *redis.Client) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			emitGauges(main, rl)
		}
	}
}

func emitGauges(main, rl *redis.Client) {
	// SSE fan-out state
	ValueEventServer.Mu.RLock()
	keys := len(ValueEventServer.TotalClients)
	clients := 0
	for _, m := range ValueEventServer.TotalClients {
		clients += len(m)
	}
	ValueEventServer.Mu.RUnlock()

	meter.Gauge("sse.clients.total", float64(clients))
	meter.Gauge("sse.keys.tracked", float64(keys))
	meter.Gauge("sse.message_queue.depth", float64(len(ValueEventServer.Message)))

	// Stats path saturation (panics at maxPaths)
	if StatsManager != nil {
		meter.Gauge("stats.paths.tracked", float64(StatsManager.pathCount.Load()))
	}

	// Drop counters (emit as gauges of the running total — Sentry handles deltas)
	meter.Gauge("sse.client.dropped", float64(SSEClientDrops.Load()))
	meter.Gauge("sse.message.dropped", float64(SSEMessageDrops.Load()))

	emitPoolStats(main, "main")
	emitPoolStats(rl, "ratelimit")
}

func emitPoolStats(c *redis.Client, pool string) {
	if c == nil {
		return
	}
	s := c.PoolStats()
	tag := sentry.WithAttributes(attribute.String("pool", pool))
	meter.Gauge("redis.pool.total_conns", float64(s.TotalConns), tag)
	meter.Gauge("redis.pool.idle_conns", float64(s.IdleConns), tag)
	meter.Gauge("redis.pool.stale_conns", float64(s.StaleConns), tag)
	meter.Gauge("redis.pool.hits", float64(s.Hits), tag)
	meter.Gauge("redis.pool.misses", float64(s.Misses), tag)
	meter.Gauge("redis.pool.timeouts", float64(s.Timeouts), tag)
}

// RedisTimingHook records per-command latency as a Sentry distribution.
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
	if meter == nil {
		return
	}
	status := "ok"
	if err != nil && !errors.Is(err, redis.Nil) {
		status = "error"
	}
	meter.Distribution(
		"redis.cmd.duration",
		float64(time.Since(start).Microseconds())/1000.0,
		sentry.WithUnit(sentry.UnitMillisecond),
		sentry.WithAttributes(
			attribute.String("pool", pool),
			attribute.String("cmd", name),
			attribute.String("status", status),
		),
	)
}
