package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	analytics "github.com/tom-draper/api-analytics/analytics/go/gin"

	"github.com/jasonlovesdoggo/abacus/utils"

	"github.com/gin-gonic/gin"
)

const DocsUrl string = "https://jasoncameron.dev/abacus/"

var startTime time.Time

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

	// Define routes
	r.NoRoute(func(c *gin.Context) {
		c.Redirect(http.StatusPermanentRedirect, DocsUrl)
	})
	// heath check
	r.StaticFile("/favicon.svg", "./assets/favicon.svg")
	r.StaticFile("/favicon.ico", "./assets/favicon.ico")
	r.GET("/healthcheck", func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{
			"status": "ok", "uptime": time.Since(startTime).String()})
	})

	r.GET("/hit/:namespace/*key", HitView)

	r.POST("/create/:namespace/*key", CreateView)
	r.GET("/create/:namespace/*key", CreateView)

	r.GET("/create/", CreateRandomView)
	r.POST("/create/", CreateRandomView)

	r.GET("/info/:namespace/*key", InfoView)

	authorized := r.Group("")
	authorized.Use(AuthMiddleware())

	authorized.POST("/delete/:namespace/*key", DeleteView)

	authorized.POST("/update/:namespace/*key", UpdateView)
	authorized.POST("/reset/:namespace/*key", ResetView)

	// Run the server
	_ = r.Run("0.0.0.0:" + os.Getenv("PORT"))
}
