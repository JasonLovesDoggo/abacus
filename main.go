package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

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
	Version string = "1.1.0"
)

var (
	Client          *redis.Client
	RateLimitClient *redis.Client
	DbNum           = 0 // 0-16
	StartTime       time.Time
	Shard           string
)

func init() {
	// Connect to Redis
	Shard = namegen.New().Get()
	utils.LoadEnv()

	if strings.ToLower(os.Getenv("DEBUG")) == "true" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	ADDR := os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT")
	log.Println("Listening to redis on: " + ADDR)
	DbNum, _ = strconv.Atoi(os.Getenv("REDIS_DB"))
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

func main() {
	//gin.SetMode(gin.ReleaseMode)
	// only run the following if .env is present
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	utils.LoadEnv()
	StartTime = time.Now()
	utils.InitializeStats(Client)
	// Initialize the Gin router
	r := gin.Default()
	// Cors
	corsConfig := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		AllowCredentials: false,
		AllowAllOrigins:  true,
		MaxAge:           12 * time.Hour,
	}
	r.Use(cors.New(corsConfig))
	if os.Getenv("API_ANALYTICS_ENABLED") == "true" {
		r.Use(analytics.Analytics(os.Getenv("API_ANALYTICS_KEY"))) // Add middleware
		log.Println("Analytics enabled")
	}
	route := r.Group("")
	route.Use(middleware.Stats())
	route.Use(middleware.RateLimit(RateLimitClient))
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
		route.GET("/get/:namespace/*key", GetView)

		route.GET("/hit/:namespace/*key", HitView)
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
	// Run the server

	srv := &http.Server{
		Addr:    ":" + os.Getenv("PORT"),
		Handler: r,
	}

	go func() {
		// service connections
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.
	quit := make(chan os.Signal, 1)
	// kill (no param) default send syscall.SIGTERM
	// kill -2 is syscall.SIGINT
	// kill -9 is syscall. SIGKILL but can"t be catch, so don't need add it
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	close(utils.ServerClose)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}
	select {
	case <-ctx.Done():
		log.Println("timeout of 5 seconds.")
	}
	log.Println("Server exiting")
}
