package middleware

import (
	"strings"
	"sync/atomic"

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
		atomic.AddInt64(&utils.Total, 1)
		val, _ := (&utils.CommonStats).LoadOrStore(route, new(int64))
		ptr := val.(*int64)
		atomic.AddInt64(ptr, 1)
		c.Next()

	}
}
