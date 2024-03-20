package main

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-redis/redis/v8"
	"github.com/jasonlovesdoggo/abacus/utils"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authToken := c.DefaultQuery("token", "")
		if authToken == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Token is required, please provide a token in the format of ?token=ADMIN_TOKEN"})
			c.Abort() // Abort further processing
			return
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
		} else { // token is valid.
			c.Next()
		}
	}
}
