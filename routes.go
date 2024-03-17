package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

var Client *redis.Client

func init() {
	// Connect to Redis
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}
	ADDR := os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT")
	fmt.Println("Listening to redis on: " + ADDR)
	PASS, _ := strconv.Atoi(os.Getenv("REDIS_DB"))
	Client = redis.NewClient(&redis.Options{
		Addr:     ADDR, // Redis server address
		Username: os.Getenv("REDIS_USERNAME"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       PASS,
	})
}

func InfoView(c *gin.Context) {
	var namespace, key string
	containsKey := c.Params.ByName("key")
	if containsKey == "" {
		namespace = "default"
		key = c.Param("key")
	} else {
		namespace = c.Param("namespace")
		key = c.Param("key")
	}

	dbKey := namespace + ":" + key
	// Get data from Redis
	val, err := Client.Get(context.Background(), dbKey).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get data. Try again later."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"value": val})
}

func HitView(c *gin.Context) {
	var namespace, key string
	key = strings.Trim(c.Param("key"), "/")
	if !(len(key) > 0) {
		namespace = "default"
		key = c.Param("namespace")
	} else {
		namespace = c.Param("namespace")
	}
	fmt.Println("namespace:"+namespace, "key:"+key)
	dbKey, err := CreateKey(namespace, key, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// Get data from Redis
	val, err := Client.Incr(context.Background(), dbKey).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get data. Try again later."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"count": val})
}
