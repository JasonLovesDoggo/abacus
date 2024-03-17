package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	analytics "github.com/tom-draper/api-analytics/analytics/go/gin"
)

const DocsUrl string = "https://jasoncameron.dev/abacus/"

var startTime time.Time

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
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
	//r.GET("/get/:namespace/:key", getData)
	//r.GET("/info/:namespace/:key", getData)
	//r.GET("/info/:namespace", setData)

	if os.Getenv("API_ANALYTICS_ENABLED") == "true" {
		r.Use(analytics.Analytics(os.Getenv("API_ANALYTICS_KEY"))) // Add middleware
	}
	// Run the server
	_ = r.Run(os.Getenv("0.0.0.0:" + "PORT"))
}
