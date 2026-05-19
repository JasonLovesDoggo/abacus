package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/require"

	"pkg.jsn.cam/abacus/utils"
)

// metricLabelCombos returns the number of unique label combinations currently
// stored for the abacus_http_request_duration_seconds metric. One combination
// of (method, route, status) = one time series in Prometheus.
func metricLabelCombos(t *testing.T) (int, []string) {
	t.Helper()
	ch := make(chan prometheus.Metric, 4096)
	go func() {
		utils.Prom.HTTPRequestDur.Collect(ch)
		close(ch)
	}()
	seen := map[string]bool{}
	var combos []string
	for m := range ch {
		var dtoMetric dto.Metric
		require.NoError(t, m.Write(&dtoMetric))
		var parts []string
		for _, lp := range dtoMetric.Label {
			parts = append(parts, lp.GetName()+"="+lp.GetValue())
		}
		key := strings.Join(parts, ",")
		if !seen[key] {
			seen[key] = true
			combos = append(combos, key)
		}
	}
	return len(seen), combos
}

func newTestRouterWithRoutes() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Prometheus())
	// Mirror the real routes shape so c.FullPath() returns parameterized templates.
	r.GET("/hit/:namespace/:key", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/get/:namespace/:key", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.POST("/create/:namespace/*key", func(c *gin.Context) { c.JSON(http.StatusCreated, gin.H{"ok": true}) })
	r.GET("/info/:namespace/*key", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	r.GET("/healthcheck", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	// SSE endpoint — same route shape as the real /stream/:namespace/*key.
	// Handler just writes immediately; the test only cares that the histogram
	// records (or doesn't record) it, not that it actually streams.
	r.GET("/stream/:namespace/*key", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
	return r
}

func resetMetric() {
	utils.Prom.HTTPRequestDur.Reset()
}

// THE invariant: spamming 1,000 distinct (namespace, key) pairs against the
// same route template must produce exactly ONE time series, not 1,000.
// If this ever fails, someone replaced c.FullPath() with c.Request.URL.Path.
func TestPrometheus_UserGeneratedKeysDoNotExplodeCardinality(t *testing.T) {
	resetMetric()
	r := newTestRouterWithRoutes()

	for i := 0; i < 1000; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/hit/ns/key"+strconv.Itoa(i), nil)
		r.ServeHTTP(w, req)
	}

	count, combos := metricLabelCombos(t)
	require.Equal(t, 1, count,
		"1000 distinct keys against one route template must produce ONE series, got %d: %v",
		count, combos)
	require.Contains(t, combos[0], "route=/hit/:namespace/:key",
		"route label must be the template, not a concrete path")
}

// 404s for unknown URLs must collapse into a single "unmatched" series.
// Without this, attackers probing random URLs would inflate cardinality.
func TestPrometheus_UnmatchedRoutesBucketedTogether(t *testing.T) {
	resetMetric()
	r := newTestRouterWithRoutes()

	for i := 0; i < 50; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/no/such/route/"+strconv.Itoa(i), nil)
		r.ServeHTTP(w, req)
	}

	count, combos := metricLabelCombos(t)
	require.Equal(t, 1, count, "50 unmatched URLs must collapse to one series, got: %v", combos)
	require.Contains(t, combos[0], "route=unmatched")
}

// Status codes must collapse into classes. 200, 201, 204 → "2xx" (one series),
// not three. Same for 400/401/404 → "4xx".
func TestPrometheus_StatusCodesCollapseIntoClasses(t *testing.T) {
	resetMetric()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Prometheus())
	r.GET("/x", func(c *gin.Context) {
		code, _ := strconv.Atoi(c.Query("code"))
		c.JSON(code, gin.H{"ok": true})
	})

	for _, code := range []int{200, 201, 204, 400, 401, 404, 500, 502, 503} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/x?code="+strconv.Itoa(code), nil)
		r.ServeHTTP(w, req)
	}

	count, combos := metricLabelCombos(t)
	// Expected: 3 series (2xx, 4xx, 5xx), all method=GET, route=/x.
	require.Equal(t, 3, count, "9 status codes across 3 classes must produce 3 series, got: %v", combos)
}

// /healthcheck must be excluded entirely — Fly scrapes it on a fixed timer
// and it would dominate the bucket counts without telling us anything about
// user-facing latency.
func TestPrometheus_HealthcheckNotRecorded(t *testing.T) {
	resetMetric()
	r := newTestRouterWithRoutes()

	for i := 0; i < 100; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/healthcheck", nil))
	}

	// Hit a real route once so the metric isn't entirely empty.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/get/ns/key", nil))

	count, combos := metricLabelCombos(t)
	require.Equal(t, 1, count, "healthcheck must be excluded; only /get should remain. got: %v", combos)
	require.NotContains(t, combos[0], "/healthcheck")
}

// SSE (/stream/:namespace/*key) must be excluded entirely. Connections are
// held open for the lifetime of the subscriber. Recording them in the same
// histogram as request/response endpoints would land every sample in the
// top buckets (15s/30s/+Inf) and poison the global percentile math on every
// other panel.
func TestPrometheus_SSENotRecorded(t *testing.T) {
	resetMetric()
	r := newTestRouterWithRoutes()

	// Hit the SSE route a bunch.
	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/stream/ns/key", nil))
	}

	// Hit a real route once so the metric isn't entirely empty.
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/get/ns/key", nil))

	count, combos := metricLabelCombos(t)
	require.Equal(t, 1, count, "SSE must be excluded; only /get should remain. got: %v", combos)
	for _, c := range combos {
		require.NotContains(t, c, "/stream", "SSE route must not appear in histogram labels")
	}
}

// Realistic upper bound check: simulate everything the real app might emit
// and assert total label combinations stay under a sane budget. If a future
// change accidentally adds a high-cardinality label, this fails loudly.
func TestPrometheus_TotalCardinalityStaysUnderBudget(t *testing.T) {
	resetMetric()
	r := newTestRouterWithRoutes()

	// Exercise every (route, method) pair plus a few status classes.
	requests := []struct {
		method, path string
	}{
		{"GET", "/hit/a/b"}, {"GET", "/hit/c/d"},
		{"GET", "/get/a/b"}, {"GET", "/get/c/d"},
		{"POST", "/create/a/b"}, {"POST", "/create/c/d"},
		{"GET", "/info/a/b"}, {"GET", "/info/c/d"},
		{"GET", "/bogus"}, // unmatched 404
		{"DELETE", "/hit/a/b"}, // wrong method = 404
	}
	for _, r2 := range requests {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(r2.method, r2.path, nil)
		r.ServeHTTP(w, req)
	}

	count, combos := metricLabelCombos(t)
	// The real app has ~10 routes, ~4 methods, 5 status classes.
	// Even at maximum hypothetical use that's ~200 combinations. Anything
	// above 50 in this test would mean a regression in label discipline.
	require.LessOrEqual(t, count, 50,
		"label cardinality must stay bounded — got %d combos: %v", count, combos)
}
