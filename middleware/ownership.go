package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/gin-gonic/gin"
	"github.com/jasonlovesdoggo/abacus/utils"
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
		//if adminDBKey == "" {
		//	c.JSON(http.StatusInternalServerError, gin.H{"error": "There is no"})
		//	c.Abort() // Abort further processing
		//	return
		//}
		adminKey, err := Client.Get(context.Background(), adminDBKey).Result()
		if errors.Is(err, redis.Nil) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "This entry is genuine and does not have an admin key. You cannot delete it. If you wanted to delete it, you should have created it with the /create endpoint."})

		} else if adminKey != authToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token is invalid"})
			c.Abort() // Abort further processing
		} else { // token is valid.
			c.Next()
		}
	}
}
