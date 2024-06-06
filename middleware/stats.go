package middleware

import (
	"strings"

	"github.com/jasonlovesdoggo/abacus/utils"

	"github.com/gin-gonic/gin"
)

func formatPath(path string) string {
	path = path[1:]
	path = strings.Split(path, "/")[0]
	return path
}

//func shouldSkip(path string) bool {
//	switch path {
//
//	case "/favicon.ico", "/docs", "/", "favicon.svg":
//		return true
//	}
//	return false
//}

func Stats() gin.HandlerFunc {
	return func(c *gin.Context) {
		route := formatPath(c.Request.URL.Path)
		utils.WriterLock.Lock()
		utils.Total++
		utils.CommonStats[route]++
		utils.WriterLock.Unlock()
		c.Next()

	}
}
