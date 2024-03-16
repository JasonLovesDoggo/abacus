package main

import (
	"net/http"
	_ "strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const DocsUrl string = "https://github.com/JasonLovesDoggo/abacus/blob/master/docs/ROUTES.md"

var startTime time.Time

func main() {
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

	// Run the server
	_ = r.Run(":8080")
}
