package middleware

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	ratelimit "github.com/JGLTechnologies/gin-rate-limit"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

var rateLimitPolicy string

const rate = 3
const limit = 30

func keyFunc(c *gin.Context) string {
	return "R:" + c.ClientIP() // rate limit key in REDIS (add R: to the beginning to distinguish from other keys)
	// return c.ClientIP() + c.FullPath()

}
func errorHandler(c *gin.Context, info ratelimit.Info) {
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error": "Too many requests. Try again in " + time.Until(info.ResetTime).String(),
	})
	c.Header("Retry-After", time.Until(info.ResetTime).String())
	c.Header("RateLimit-Reset", fmt.Sprintf("%d", info.ResetTime.Unix()))
	c.Header("RateLimit-Remaining", "0")
	c.Header("RateLimit-Policy", rateLimitPolicy)

}
func beforeResponse(c *gin.Context, info ratelimit.Info) {
	c.Header("RateLimit-Remaining", fmt.Sprintf("%v", info.RemainingHits))
	c.Header("RateLimit-Reset", fmt.Sprintf("%d", info.ResetTime.Unix()))
	c.Header("RateLimit-Policy", rateLimitPolicy)
}

func RateLimit(client *redis.Client) gin.HandlerFunc {
	// This makes it so each ip can only make 5 requests per second
	rateLimitPolicy = strconv.Itoa(limit) + ";w=" + strconv.Itoa(rate) // paragraph 2.1 of the IETF Draft
	store := ratelimit.RedisStore(&ratelimit.RedisOptions{
		RedisClient: client,
		Rate:        time.Second * rate,
		Limit:       limit,
	})
	mw := RateLimiter(store, &ratelimit.Options{
		ErrorHandler:   errorHandler,
		KeyFunc:        keyFunc,
		BeforeResponse: beforeResponse,
	})
	return mw
}
func RateLimiter(s ratelimit.Store, options *ratelimit.Options) gin.HandlerFunc {
	if options == nil {
		options = &ratelimit.Options{}
	}
	return func(c *gin.Context) {
		key := options.KeyFunc(c)
		info := s.Limit(key, c)
		options.BeforeResponse(c, info)
		if c.IsAborted() {
			return
		}
		if info.RateLimited {
			options.ErrorHandler(c, info)
			c.Abort()
		} else {
			c.Next()
		}
	}
}
