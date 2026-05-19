package middleware

import (
	"time"

	"github.com/gin-gonic/gin"

	"pkg.jsn.cam/abacus/utils"
)

// Prometheus returns a gin middleware that records request latency into
// utils.Prom.HTTPRequestDur.
//
// Cardinality-safety contract — DO NOT BREAK THESE WHEN EDITING:
//
//   - The "route" label is c.FullPath(), the gin route TEMPLATE. The raw URL
//     path (c.Request.URL.Path) contains the namespace and key segments which
//     come from user input. Labeling by the raw path would create one time
//     series per (namespace, key) pair — unbounded — and OOM Prometheus.
//
//   - For requests that don't match any registered route, FullPath() returns
//     "". We bucket those as "unmatched" so they remain observable as a
//     single series without leaking the raw URL.
//
//   - The "status" label is the response code's CLASS (e.g. "2xx", "4xx",
//     "5xx"), not the exact code. Class-based labeling keeps cardinality
//     bounded at ~5 values forever; full status codes would slowly accumulate
//     every unusual response gin emits (304, 405, 429, 503, …) without
//     adding alerting signal.
//
//   - The "method" label is bounded by HTTP itself (~9 verbs).
//
// Worst-case cardinality: ~10 routes × ~5 methods × 5 status classes × 20
// histogram buckets ≈ 5000 series. Safe.
//
// Skipped routes (matched on c.FullPath(), the route template):
//
//   - /healthcheck: scraped by Fly every 30s, would dominate the count
//     without telling us anything about user-facing latency.
//
//   - /stream/:namespace/*key: SSE long-lived connections held open for
//     the lifetime of the subscriber. Recording these in the same histogram
//     as fast request/response endpoints would land every sample in the
//     +Inf / 30s buckets and poison the global p50/p95/p99 math.
//
// (/metrics isn't listed because it's served on a separate :9091 listener
// outside the gin router, so it never reaches this middleware.)
func Prometheus() gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.FullPath() {
		case "/healthcheck", "/stream/:namespace/*key":
			c.Next()
			return
		}

		start := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = "unmatched"
		}

		utils.Prom.HTTPRequestDur.
			WithLabelValues(c.Request.Method, route, statusClass(c.Writer.Status())).
			Observe(time.Since(start).Seconds())
	}
}

// statusClass collapses an HTTP status code into a small bounded class label.
func statusClass(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	case code >= 200:
		return "2xx"
	case code >= 100:
		return "1xx"
	default:
		// gin returns 0 if the handler never wrote a response (rare, but
		// possible during a panic before Recovery kicks in). Bucket it
		// distinctly so it's noticed.
		return "unknown"
	}
}
