package utils

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

// Prom holds the metrics that the redis hook (and other call sites) update on
// hot paths. Gauges that read live state use GaugeFunc and need no updates.
var Prom = struct {
	registry   *prometheus.Registry
	registered bool

	redisCmdDuration *prometheus.HistogramVec
	redisCmdErrors   *prometheus.CounterVec
	sseClientDrops   prometheus.Counter
	sseMessageDrops  prometheus.Counter
	HTTPRequestDur   *prometheus.HistogramVec
}{
	HTTPRequestDur: prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "abacus_http_request_duration_seconds",
			Help: "End-to-end HTTP request latency per route, method, and status class.",
			// Same shape as the Redis histogram. App-level latency is dominated
			// by Redis time + a few hundred µs of gin/middleware overhead, so
			// the same 100µs-to-30s range covers both healthy and brownout cases.
			Buckets: prometheus.ExponentialBucketsRange(0.0001, 30.0, 20),
		},
		// route is the gin route TEMPLATE (e.g. "/hit/:namespace/:key"), not
		// the raw URL — otherwise label cardinality is unbounded across the
		// user-generated key space. status is the status class (2xx/4xx/5xx)
		// for the same reason. See middleware/prometheus.go for the
		// cardinality-safety contract.
		[]string{"method", "route", "status"},
	),
	redisCmdDuration: prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "abacus_redis_cmd_duration_seconds",
			Help:    "Redis command latency, by pool and command name.",
			// 20 exponential buckets from 100µs to 30s. The previous 2s ceiling
			// hid actual tail latency during Redis brownouts — the histogram
			// would report "2.00s" for every slow cmd because anything past 2s
			// landed in +Inf and histogram_quantile clamps to the top finite
			// bucket. 30s covers the worst plausible pool-timeout + retry case.
			Buckets: prometheus.ExponentialBucketsRange(0.0001, 30.0, 20),
		},
		[]string{"pool", "cmd"},
	),
	redisCmdErrors: prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "abacus_redis_cmd_errors_total",
			Help: "Redis command errors (excludes redis.Nil), by pool and command.",
		},
		[]string{"pool", "cmd"},
	),
	sseClientDrops: prometheus.NewCounter(prometheus.CounterOpts{
		Name: "abacus_sse_client_drops_total",
		Help: "SSE client channels evicted because they were full (slow consumer).",
	}),
	sseMessageDrops: prometheus.NewCounter(prometheus.CounterOpts{
		Name: "abacus_sse_message_drops_total",
		Help: "SSE broadcast messages dropped because the fan-out queue was full.",
	}),
}

// InitPrometheus registers all collectors and starts the scrape server on addr.
// Fly auto-scrapes 0.0.0.0:9091/metrics every 15s when [metrics] is set in fly.toml.
func InitPrometheus(ctx context.Context, addr string, main, rl *redis.Client) {
	if Prom.registered {
		return
	}
	Prom.registry = prometheus.NewRegistry()

	// Go runtime + process collectors (memory, GC, goroutines, fds).
	Prom.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		Prom.redisCmdDuration,
		Prom.redisCmdErrors,
		Prom.sseClientDrops,
		Prom.sseMessageDrops,
		Prom.HTTPRequestDur,
	)

	// Live-read gauges. GaugeFunc evaluates on every scrape, so we don't need
	// the periodic ticker just to keep values fresh.
	Prom.registry.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{Name: "abacus_sse_clients_total", Help: "Connected SSE clients across all keys."},
		func() float64 {
			ValueEventServer.Mu.RLock()
			defer ValueEventServer.Mu.RUnlock()
			n := 0
			for _, m := range ValueEventServer.TotalClients {
				n += len(m)
			}
			return float64(n)
		},
	))
	Prom.registry.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{Name: "abacus_sse_keys_tracked", Help: "Distinct keys with at least one SSE subscriber."},
		func() float64 {
			ValueEventServer.Mu.RLock()
			defer ValueEventServer.Mu.RUnlock()
			return float64(len(ValueEventServer.TotalClients))
		},
	))
	Prom.registry.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{Name: "abacus_sse_message_queue_depth", Help: "Pending items in the SSE broadcast channel."},
		func() float64 { return float64(len(ValueEventServer.Message)) },
	))
	Prom.registry.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{Name: "abacus_stats_paths_tracked", Help: "Unique request paths the stats manager is tracking."},
		func() float64 {
			if StatsManager == nil {
				return 0
			}
			return float64(StatsManager.pathCount.Load())
		},
	))

	// EXPIRE coalescing visibility. The ratio Skipped / (Skipped+Refreshed)
	// tells us how much Redis traffic the gate is saving. These are running
	// totals so they're exposed as Counters (matching the _total suffix) —
	// rate() and increase() will work correctly across scrapes.
	Prom.registry.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{Name: "abacus_expire_refreshed_total", Help: "EXPIRE calls actually issued (cumulative)."},
		func() float64 {
			if ExpireGate == nil {
				return 0
			}
			return float64(ExpireGate.Refreshed.Load())
		},
	))
	Prom.registry.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{Name: "abacus_expire_skipped_total", Help: "EXPIRE calls suppressed by the coalescer (cumulative)."},
		func() float64 {
			if ExpireGate == nil {
				return 0
			}
			return float64(ExpireGate.Skipped.Load())
		},
	))
	// GetCache visibility. Hits/(Hits+Misses) is the cache-hit rate;
	// rate(misses_total) is the Redis-side load this cache is generating.
	Prom.registry.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{Name: "abacus_get_cache_hits_total", Help: "GetCache hits, including cached not-found (cumulative)."},
		func() float64 {
			if GetCacheV == nil {
				return 0
			}
			return float64(GetCacheV.Hits.Load())
		},
	))
	Prom.registry.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{Name: "abacus_get_cache_misses_total", Help: "GetCache misses (cache fills) actually sent to Redis (cumulative)."},
		func() float64 {
			if GetCacheV == nil {
				return 0
			}
			return float64(GetCacheV.Misses.Load())
		},
	))
	Prom.registry.MustRegister(prometheus.NewCounterFunc(
		prometheus.CounterOpts{Name: "abacus_get_cache_evicted_total", Help: "GetCache entries evicted by the sweeper (cumulative)."},
		func() float64 {
			if GetCacheV == nil {
				return 0
			}
			return float64(GetCacheV.Evicted.Load())
		},
	))
	Prom.registry.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{Name: "abacus_get_cache_size", Help: "GetCache current entry count."},
		func() float64 {
			if GetCacheV == nil {
				return 0
			}
			return float64(GetCacheV.Size())
		},
	))

	Prom.registry.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{Name: "abacus_expire_cache_size", Help: "Number of keys currently tracked by the EXPIRE coalescer."},
		func() float64 {
			if ExpireGate == nil {
				return 0
			}
			return float64(ExpireGate.Size())
		},
	))

	registerPoolGauges("main", main)
	registerPoolGauges("ratelimit", rl)

	Prom.registered = true

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.HandlerFor(Prom.registry, promhttp.HandlerOpts{}))
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("Prometheus metrics on %s/metrics", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("prometheus server error: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
}

func registerPoolGauges(pool string, c *redis.Client) {
	if c == nil {
		return
	}
	labels := prometheus.Labels{"pool": pool}
	g := func(name, help string, f func(*redis.PoolStats) float64) {
		Prom.registry.MustRegister(prometheus.NewGaugeFunc(
			prometheus.GaugeOpts{Name: name, Help: help, ConstLabels: labels},
			func() float64 { return f(c.PoolStats()) },
		))
	}
	g("abacus_redis_pool_total_conns", "Total connections in the pool.",
		func(s *redis.PoolStats) float64 { return float64(s.TotalConns) })
	g("abacus_redis_pool_idle_conns", "Idle connections waiting for work.",
		func(s *redis.PoolStats) float64 { return float64(s.IdleConns) })
	g("abacus_redis_pool_stale_conns", "Stale connections reaped.",
		func(s *redis.PoolStats) float64 { return float64(s.StaleConns) })
	g("abacus_redis_pool_hits", "Connection pool hits (cumulative).",
		func(s *redis.PoolStats) float64 { return float64(s.Hits) })
	g("abacus_redis_pool_misses", "Connection pool misses (cumulative).",
		func(s *redis.PoolStats) float64 { return float64(s.Misses) })
	g("abacus_redis_pool_timeouts", "Pool wait timeouts (cumulative). Should stay at 0.",
		func(s *redis.PoolStats) float64 { return float64(s.Timeouts) })
}
