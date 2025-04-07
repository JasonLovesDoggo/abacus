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
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/alicebob/miniredis/v2"

	"github.com/anandvarma/namegen"

	"github.com/redis/go-redis/v9"

	"github.com/jasonlovesdoggo/abacus/middleware"

	"github.com/gin-contrib/cors"
	analytics "github.com/tom-draper/api-analytics/analytics/go/gin"

	"github.com/jasonlovesdoggo/abacus/utils"

	"github.com/gin-gonic/gin"
)

const (
	DocsUrl string = "https://jasoncameron.dev/abacus/"
	Version string = "1.4.0"
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

func CreateRouter() *gin.Engine {
	utils.InitializeStatsManager(Client)
	r := gin.Default()

	if gin.Mode() == gin.DebugMode {
		// Register pprof handlers
		pproRouter := r.Group("/debug/pprof")
		{
			pproRouter.GET("/", gin.WrapF(pprof.Index))
			pproRouter.GET("/cmdline", gin.WrapF(pprof.Cmdline))
			pproRouter.GET("/profile", gin.WrapF(pprof.Profile))
			pproRouter.GET("/symbol", gin.WrapF(pprof.Symbol))
			pproRouter.GET("/trace", gin.WrapF(pprof.Trace))
			pproRouter.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
			pproRouter.GET("/block", gin.WrapH(pprof.Handler("block")))
			pproRouter.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
			pproRouter.GET("/heap", gin.WrapH(pprof.Handler("heap")))
			pproRouter.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
			pproRouter.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
		}
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
