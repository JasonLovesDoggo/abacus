package middleware

import (
	"strings"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"github.com/jasonlovesdoggo/abacus/utils"
	"go.uber.org/zap"
)

var logger, _ = zap.NewProduction()

func formatPath(path string) string {
	if len(path) == 0 {
		return ""
	}
	parts := strings.SplitN(path[1:], "/", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func Stats() gin.HandlerFunc {
	return func(c *gin.Context) {
		if utils.StatsManager == nil {
			logger.Fatal("StatsManager not initialized. Call InitializeStatsManager first")
		}

		path := formatPath(c.Request.URL.Path)
		if path == "" {
			c.Next()
			return
		}

		atomic.AddInt64(&utils.Total, 1)
		utils.StatsManager.RecordStat(path, 1)

		c.Next()
	}
}
