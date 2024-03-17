package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
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
	dbKey, err := createKey(namespace, key, false)
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

func createKey(namespace, key string, skipValidation bool) (string, error) {
	if skipValidation == true {
		fmt.Println("skipValidation")
		if err := validate(namespace); err != nil {
			return "", err
		}
		if err := validate(key); err != nil {
			return "", err
		}
	}

	// Construct the Redis key
	fmt.Println("key:" + namespace + ":" + key)
	key = strings.Trim(key, "/")
	return "key:" + namespace + ":" + key, nil
}

// validate checks if the namespace/key meet the validation criteria.
func validate(input string) error {
	if len(input) <= 3 || len(input) >= 64 {
		return fmt.Errorf("length must be between 3 and 64 characters inclusive")
	}
	match, err := regexp.MatchString(`^[A-Za-z0-9_\-.]{3,64}$`, input)
	fmt.Println(match, err, input)
	if err != nil {
		return err
	}
	if !match {
		return fmt.Errorf("must match the pattern ^[A-Za-z0-9_\\-.]{3,64}$")
	}
	return nil
}
