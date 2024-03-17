package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/google/uuid"

	"github.com/jasonlovesdoggo/abacus/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
)

var Client *redis.Client

func init() {
	// Connect to Redis
	utils.LoadEnv()
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
	namespace, key := utils.GetNamespaceKey(c)
	if namespace == "" || key == "" {
		return
	}
	//fmt.Println("namespace:"+namespace, "key:"+key)
	dbKey, err := utils.CreateKey(namespace, key, false)
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

func CreateView(c *gin.Context) {
	namespace, key := utils.GetNamespaceKey(c)
	if namespace == "" || key == "" {
		return
	}
	dbKey, err := utils.CreateKey(namespace, key, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	initialValue := c.DefaultQuery("initializer", "")
	_, err = strconv.Atoi(initialValue)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "initializer must be a number"})
		return
	}
	// Get data from Redis
	created := Client.SetNX(context.Background(), dbKey, initialValue, 0)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set data. Try again later."})
		return
	}
	if created.Val() == false {
		c.JSON(http.StatusConflict, gin.H{"error": "Key already exists, please use a different key."})
		return
	}
	AdminKey := uuid.New().String() // Create a new admin key used for deletion and control
	Client.Set(context.Background(), utils.CreateAdminKey(dbKey), AdminKey, 0)
	c.JSON(http.StatusCreated, gin.H{"key": dbKey, "admin_key": AdminKey, "value": initialValue})
}
