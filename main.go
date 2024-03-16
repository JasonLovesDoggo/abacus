package main

import (
	"net/http"
	_ "strconv"

	"github.com/gin-gonic/gin"
)

const DocsUrl string = "https://github.com/JasonLovesDoggo/abacus/blob/master/docs/ROUTES.md"

func main() {
	// Initialize the Gin router
	r := gin.Default()

	// Define routes
	r.NoRoute(func(c *gin.Context) {
		c.Redirect(http.StatusPermanentRedirect, DocsUrl)
	})
	r.GET("/hit/:namespace/*key", HitView)
	//r.GET("/get/:namespace/:key", getData)
	//r.GET("/info/:namespace/:key", getData)
	//r.GET("/info/:namespace", setData)

	// Run the server
	_ = r.Run(":8080")
}
