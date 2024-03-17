package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"

	"github.com/jasonlovesdoggo/abacus/utils"

	"github.com/gin-gonic/gin"
	analytics "github.com/tom-draper/api-analytics/analytics/go/gin"
)

const DocsUrl string = "https://jasoncameron.dev/abacus/"

var startTime time.Time

func main() {
	// only run the following if .env is present
	utils.LoadEnv()
	startTime = time.Now()
	// Initialize the Gin router
	r := gin.Default()

	// Define routes
	r.NoRoute(func(c *gin.Context) {
		c.Redirect(http.StatusPermanentRedirect, DocsUrl)
	})
	// heath check
	r.GET("/healthcheck", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{
			"status": "ok", "uptime": time.Since(startTime).String()})
	})

	r.GET("/hit/:namespace/*key", HitView)
	r.POST("/hit/:namespace/*key", HitView)
	r.POST("/create/:namespace/*key", CreateView)
	r.GET("/create/:namespace/*key", CreateView)
	//r.GET("/info/:namespace/:key", getData)
	//r.GET("/info/:namespace", setData)

	if os.Getenv("API_ANALYTICS_ENABLED") == "true" {
		r.Use(analytics.Analytics(os.Getenv("API_ANALYTICS_KEY"))) // Add middleware
		fmt.Println("Analytics enabled")
	}
	// Run the server
	r.Use(cors.Default())
	_ = r.Run("0.0.0.0:" + os.Getenv("PORT"))
}
