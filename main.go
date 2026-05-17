package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/anandvarma/namegen"
	"github.com/redis/go-redis/v9"

	"pkg.jsn.cam/abacus/middleware"

	"github.com/getsentry/sentry-go"
	sentrygin "github.com/getsentry/sentry-go/gin"
	"github.com/gin-contrib/cors"
	analytics "github.com/tom-draper/api-analytics/analytics/go/gin"

	"pkg.jsn.cam/abacus/utils"

	"github.com/gin-gonic/gin"
)

const (
	DocsUrl string = "https://jasoncameron.dev/abacus/"
	Version string = "1.6.0"
)

var (
	Client          *redis.Client
	RateLimitClient *redis.Client
	DbNum           = 0 // 0-16
	StartTime       time.Time
	Shard           string
)

const is32Bit = uint64(^uintptr(0)) != ^uint64(0)

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if len(value) == 0 {
		return defaultValue
	}
	return value
}

func init() {
	utils.LoadEnv()

	if strings.ToLower(os.Getenv("DEBUG")) == "true" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Use miniredis for testing
	if strings.ToLower(os.Getenv("TESTING")) == "true" {
		setupMockRedis()
		return
	}

	// Production Redis setup
	Shard = namegen.New().Get()

	ADDR := os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT")
	log.Println("Listening to redis on: " + ADDR)
	var err error
	DbNum, err = strconv.Atoi(os.Getenv("REDIS_DB"))
	if err != nil {
		DbNum = 0 // Default to 0 if not set
	} else if DbNum < 0 || DbNum > 16 {
		log.Fatalf("Redis DB must be between 0-16: %v", DbNum)
	}

	Client = redis.NewClient(&redis.Options{
		Addr:     ADDR, // Redis server address
		Username: os.Getenv("REDIS_USERNAME"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       DbNum,
	})
	RateLimitClient = redis.NewClient(&redis.Options{
		Addr:     ADDR, // Redis server address
		Username: os.Getenv("REDIS_USERNAME"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       DbNum + 1,
	})
}

func setupMockRedis() {
	// Used for testing, "miniredis" is a mock Redis server that runs in-memory for testing purposes only (no persistence)
	mr, err := miniredis.Run()
	if err != nil {
		log.Fatalf("Failed to start miniredis: %v", err)
	}

	log.Println("Using miniredis for testing")

	// Connect clients to miniredis
	Client = redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	RateLimitClient = redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
}

func registerPprof(g *gin.RouterGroup) {
	g.GET("/", gin.WrapF(pprof.Index))
	g.GET("/cmdline", gin.WrapF(pprof.Cmdline))
	g.GET("/profile", gin.WrapF(pprof.Profile))
	g.GET("/symbol", gin.WrapF(pprof.Symbol))
	g.GET("/trace", gin.WrapF(pprof.Trace))
	g.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
	g.GET("/block", gin.WrapH(pprof.Handler("block")))
	g.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	g.GET("/heap", gin.WrapH(pprof.Handler("heap")))
	g.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
	g.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
}

// startPprofServer binds pprof to a private port (default 6060) so it never
// touches the public listener. Enable with PPROF_ENABLED=true and reach it via
// `fly proxy 6060 -a j-abacus`.
func startPprofServer(ctx context.Context) {
	if strings.ToLower(os.Getenv("PPROF_ENABLED")) != "true" {
		return
	}
	// Bind to all interfaces so `fly proxy` (which routes via the 6PN private
	// network, not loopback) can reach it. Stays private because fly.toml only
	// forwards 8080 publicly.
	addr := getEnv("PPROF_ADDR", ":6060")
	// Enable block/mutex profilers (off by default in Go). Cheap rates.
	runtime.SetBlockProfileRate(int(time.Millisecond))
	runtime.SetMutexProfileFraction(100)

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)

	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("pprof listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("pprof server error: %v", err)
		}
	}()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
}

func initSentry() {
	dsn := getEnv("SENTRY_DSN", "https://76e82ebd5a8b301511aa8a518f1baf36@o4505315141025792.ingest.us.sentry.io/4511403097194496")
	if strings.ToLower(os.Getenv("SENTRY_ENABLED")) == "false" || os.Getenv("TESTING") == "true" {
		return
	}

	sampleRate, err := strconv.ParseFloat(getEnv("SENTRY_TRACES_SAMPLE_RATE", "0.05"), 64)
	if err != nil {
		sampleRate = 0.05
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              dsn,
		EnableTracing:    true,
		TracesSampleRate: sampleRate,
		Release:          "abacus@" + Version,
		Environment:      getEnv("SENTRY_ENV", "production"),
		Debug:            strings.ToLower(os.Getenv("SENTRY_DEBUG")) == "true",
	}); err != nil {
		log.Printf("Sentry initialization failed: %v", err)
		return
	}

	// Smoke-test the wire: one tagged message + one transaction with a span.
	// If neither shows up in Sentry, the DSN/network is broken; if the message
	// lands but no transaction appears, it's a sampling/UI issue, not the SDK.
	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("smoke_test", "startup")
		scope.SetLevel(sentry.LevelInfo)
		sentry.CaptureMessage("abacus startup: sentry wire OK (release=" + Version + ")")
	})

	tx := sentry.StartTransaction(context.Background(), "smoke_test.startup",
		sentry.WithOpName("startup"),
		sentry.WithSpanSampled(sentry.SampledTrue),
	)
	span := tx.StartChild("smoke_test.span")
	time.Sleep(5 * time.Millisecond)
	span.Finish()
	tx.Finish()

	// Force the events out now instead of waiting for the batcher.
	sentry.Flush(3 * time.Second)
	log.Println("Sentry smoke test sent (message + transaction). Look for 'smoke_test' tag.")
}

func CreateRouter() *gin.Engine {
	utils.InitializeStatsManager(Client)

	// Async stdout for gin's access log so per-request writes never block on a
	// stdout flush. Gin's Logger reads this writer when Default() runs below.
	asyncOut := utils.DefaultAsyncStdout()
	gin.DefaultWriter = asyncOut
	gin.DefaultErrorWriter = asyncOut

	r := gin.Default()
	r.Use(sentrygin.New(sentrygin.Options{Repanic: true}))

	if gin.Mode() == gin.DebugMode {
		// In debug, expose pprof on the main router for convenience.
		registerPprof(r.Group("/debug/pprof"))
	}

	// Cors
	corsConfig := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		AllowCredentials: false,
		AllowAllOrigins:  true,
		MaxAge:           12 * time.Hour,
	}

	r.Use(cors.New(corsConfig))
	r.Use(gin.Recovery()) // recover from panics and returns a 500 error
	if os.Getenv("API_ANALYTICS_ENABLED") == "true" {
		r.Use(analytics.Analytics(os.Getenv("API_ANALYTICS_KEY"))) // Add middleware
		log.Println("Analytics enabled")
	}
	route := r.Group("")
	route.Use(middleware.Stats())
	if os.Getenv("RATE_LIMIT_ENABLED") == "true" {
		route.Use(middleware.RateLimit(RateLimitClient))
		log.Println("Rate limiting enabled")
	}
	// Define routes
	r.NoRoute(func(c *gin.Context) {
		c.Redirect(http.StatusPermanentRedirect, DocsUrl)
	})
	// heath check
	r.StaticFile("/favicon.svg", "./assets/favicon.svg")
	r.StaticFile("/favicon.ico", "./assets/favicon.ico")

	{ // Stats Routes
		route.GET("/healthcheck", func(context *gin.Context) {
			context.JSON(http.StatusOK, gin.H{
				"status": "ok", "uptime": time.Since(StartTime).String()})
		})

		route.GET("/docs", func(context *gin.Context) {
			context.Redirect(http.StatusPermanentRedirect, DocsUrl)
		})

		route.GET("/stats", StatsView)
	}
	{ // Public Routes
		route.GET("/get/:namespace/:key", GetView)
		route.GET("/get/:namespace/:key/shield", GetShieldView)

		route.GET("/hit/:namespace/:key/shield", HitShieldView)
		route.GET("/hit/:namespace/:key", HitView)
		route.GET("/stream/:namespace/*key", middleware.SSEMiddleware(), StreamValueView)

		route.POST("/create/:namespace/*key", CreateView)
		route.GET("/create/:namespace/*key", CreateView)

		route.GET("/create/", CreateRandomView)
		route.POST("/create/", CreateRandomView)

		route.GET("/info/:namespace/*key", InfoView)
	}
	authorized := route.Group("")
	authorized.Use(middleware.Auth(Client))
	{ // Authorized Routes
		authorized.POST("/delete/:namespace/*key", DeleteView)

		authorized.POST("/set/:namespace/*key", SetView)
		authorized.POST("/reset/:namespace/*key", ResetView)
		authorized.POST("/update/:namespace/*key", UpdateByView)
	}
	return r
}

func main() {
	if is32Bit {
		log.Fatal("This program is not supported on 32-bit systems, " +
			"please run on a 64-bit system. If you wish for 32-bit support, " +
			"please open an issue on the GitHub repository.\nexiting...")
	}

	utils.LoadEnv()
	StartTime = time.Now()

	initSentry()
	defer sentry.Flush(2 * time.Second)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if Client != nil {
		Client.AddHook(utils.RedisTimingHook{Pool: "main"})
	}
	if RateLimitClient != nil {
		RateLimitClient.AddHook(utils.RedisTimingHook{Pool: "ratelimit"})
	}
	utils.InitMetrics(ctx, Client, RateLimitClient)
	startPprofServer(ctx)

	r := CreateRouter()
	srv := &http.Server{
		Addr:    ":" + getEnv("PORT", "8080"),
		Handler: r,
	}
	fmt.Println("Listening on port " + getEnv("PORT", "8080"))

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal
	<-ctx.Done()
	log.Println("Shutdown signal received")

	// Signal StatsManager to save and wait for completion
	log.Println("Signaling stats manager to save data...")
	utils.ServerClose <- true

	// Wait for the response on the same channel
	log.Println("Waiting for stats manager to complete...")
	<-utils.ServerClose
	log.Println("Stats saving confirmed complete")

	// Now close Redis connections
	log.Println("Closing Redis connections...")
	if Client != nil {
		if err := Client.Close(); err != nil {
			log.Printf("Error closing Redis client: %v", err)
		}
	}
	if RateLimitClient != nil {
		if err := RateLimitClient.Close(); err != nil {
			log.Printf("Error closing Redis rate limit client: %v", err)
		}
	}

	// Shut down the HTTP server
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}

	<-shutdownCtx.Done()
	if errors.Is(shutdownCtx.Err(), context.DeadlineExceeded) {
		log.Println("Shutdown timeout of 5 seconds exceeded")
	}
	log.Println("Server exiting")
}
