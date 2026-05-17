package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/gin-gonic/gin"
	"pkg.jsn.cam/abacus/utils"
)

func Auth(Client *redis.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		authToken := ""
		authTokenHeader := c.Request.Header.Get("Authorization")
		if !strings.HasPrefix(authTokenHeader, "Bearer ") {
			authTokenQuery := c.DefaultQuery("token", "")
			if authTokenQuery == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required, " +
					"please provide a token in the format of a Bearer token header or ?token=ADMIN_TOKEN"})
				c.Abort() // Abort further processing
				return
			}
			authToken = authTokenQuery
		} else {
			authToken = strings.TrimPrefix(authTokenHeader, "Bearer ")
		}

		adminDBKey := utils.CreateRawAdminKey(c)
		adminKey, err := Client.Get(context.Background(), adminDBKey).Result()
		switch {
		case errors.Is(err, redis.Nil):
			c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": "This entry is genuine and does not have an admin key. You cannot delete it. If you wanted to delete it, you should have created it with the /create endpoint."})
		case err != nil:
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify token. Try again later."})
		case adminKey != authToken:
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "token is invalid"})
		default:
			c.Next()
		}
	}
}
