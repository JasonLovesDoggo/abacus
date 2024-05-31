package main

import (
	"fmt"
	"github.com/redis/go-redis/v9"
	"net/http"
	"os"
	"strconv"
	"time"

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
	DbNum           int = 0
	startTime       time.Time
)

func init() {
	// Connect to Redis
	utils.LoadEnv()
	ADDR := os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT")
	fmt.Println("Listening to redis on: " + ADDR)
	DbNum, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
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
	// only run the following if .env is present
	utils.LoadEnv()
	startTime = time.Now()
	// Initialize the Gin router
	r := gin.Default()
	r.Use(cors.Default())
	if os.Getenv("API_ANALYTICS_ENABLED") == "true" {
		r.Use(analytics.Analytics(os.Getenv("API_ANALYTICS_KEY"))) // Add middleware
		fmt.Println("Analytics enabled")
	}
	route := r.Group("")
	route.Use(middleware.RateLimit(RateLimitClient))

	// Define routes
	r.NoRoute(func(c *gin.Context) {
		c.Redirect(http.StatusPermanentRedirect, DocsUrl)
	})
	// heath check
	r.StaticFile("/favicon.svg", "./assets/favicon.svg")
	r.StaticFile("/favicon.ico", "./assets/favicon.ico")
	route.GET("/healthcheck", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{
			"status": "ok", "uptime": time.Since(startTime).String()})
	})
	route.GET("/docs", func(context *gin.Context) {
		context.Redirect(http.StatusPermanentRedirect, DocsUrl)
	})
	route.GET("/stats", StatsView)

	route.GET("/get/:namespace/*key", GetView)

	route.GET("/hit/:namespace/*key", HitView)
	route.GET("/stream/:namespace/*key", middleware.SSEMiddleware(), StreamValueView)

	route.POST("/create/:namespace/*key", CreateView)
	route.GET("/create/:namespace/*key", CreateView)

	route.GET("/create/", CreateRandomView)
	route.POST("/create/", CreateRandomView)

	route.GET("/info/:namespace/*key", InfoView)

	authorized := route.Group("")
	authorized.Use(middleware.Auth(Client))

	authorized.POST("/delete/:namespace/*key", DeleteView)

	authorized.POST("/set/:namespace/*key", SetView)
	authorized.POST("/reset/:namespace/*key", ResetView)
	authorized.POST("/update/:namespace/*key", UpdateByView)

	// Run the server
	_ = r.Run("0.0.0.0:" + os.Getenv("PORT"))
}
