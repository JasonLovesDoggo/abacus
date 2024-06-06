package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/google/uuid"

	"github.com/jasonlovesdoggo/abacus/utils"

	"github.com/gin-gonic/gin"
)

var (
	subscriberMap = make(map[string]chan int) // Map to store channels for each counter
	mutex         sync.Mutex
)

func SetStream(dbKey string, value int) {
	mutex.Lock()
	ch, ok := subscriberMap[dbKey]
	mutex.Unlock()
	if ok {
		ch <- value
	}
}

func StreamValueView(c *gin.Context) { // todo: fix hanging when key does not exist
	namespace, key := utils.GetNamespaceKey(c)
	if namespace == "" || key == "" {
		return
	}
	dbKey := utils.CreateKey(c, namespace, key, false)
	if dbKey == "" {
		return
	}
	// Create a channel for this counter's updates if not already created
	mutex.Lock()
	ch, ok := subscriberMap[dbKey]
	if !ok {
		ch = make(chan int)
		subscriberMap[dbKey] = ch
	}
	mutex.Unlock()

	// Listen for client disconnect
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer func() {
		cancel()
		_ch, _ok := subscriberMap[dbKey]
		if _ok {
			mutex.Lock()
			close(_ch)
			delete(subscriberMap, dbKey)
			mutex.Unlock()
		}
	}()

	// Send initial count to a client
	val := Client.Get(context.Background(), dbKey).Val()
	if count, err := strconv.Atoi(val); err == nil {
		message := fmt.Sprintf("{\"value\": %d}\n", count)
		_, err := c.Writer.WriteString(message)
		if err != nil {
			return
		}
		c.Writer.Flush()
	}

	// Stream count updates to the client
	for {
		select {
		case <-ctx.Done():
			return
		case count, open := <-ch:
			if !open {
				return
			}
			message := fmt.Sprintf("{\"value\": %d}\n", count)
			_, err := c.Writer.WriteString(message)
			if err != nil {
				return
			}
			c.Writer.Flush()
		}
	}
}

func HitView(c *gin.Context) {
	namespace, key := utils.GetNamespaceKey(c)
	if namespace == "" || key == "" {
		return
	}
	dbKey := utils.CreateKey(c, namespace, key, false)
	if dbKey == "" { // error is handled in CreateKey
		return
	}
	// Get data from Redis
	val, err := Client.Incr(context.Background(), dbKey).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get data. Try again later."})
		return
	}
	go func() {
		SetStream(dbKey, int(val))
		Client.Expire(context.Background(), dbKey, utils.BaseTTLPeriod)
	}()
	if c.Query("callback") != "" {
		c.JSONP(http.StatusOK, gin.H{"value": val})

	} else {
		c.JSON(http.StatusOK, gin.H{"value": val})

	}
}

func GetView(c *gin.Context) {
	namespace, key := utils.GetNamespaceKey(c)
	if namespace == "" || key == "" {
		return
	}
	dbKey := utils.CreateKey(c, namespace, key, false)
	if dbKey == "" { // error is handled in CreateKey
		return
	}
	// Get data from Redis
	val, err := Client.Get(context.Background(), dbKey).Result()

	if errors.Is(err, redis.Nil) {
		c.JSON(http.StatusNotFound, gin.H{"error": "Key not found"})
		return
	} else if err != nil { // Other Redis errors
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get data. Try again later."})
		return
	}

	go func() {
		Client.Expire(context.Background(), dbKey, utils.BaseTTLPeriod)
	}()
	intval, _ := strconv.Atoi(val)
	if c.Query("callback") != "" {
		c.JSONP(http.StatusOK, gin.H{"value": intval})

	} else {
		c.JSON(http.StatusOK, gin.H{"value": intval})

	}
}

func CreateRandomView(c *gin.Context) {
	key, _ := utils.GenerateRandomString(16)
	namespace, err := utils.GenerateRandomString(16)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate random string. Try again later."})
		return
	}

	c.Params = gin.Params{gin.Param{Key: "namespace", Value: namespace}, gin.Param{Key: "key", Value: key}}
	CreateView(c)
}
func CreateView(c *gin.Context) {
	namespace, key := utils.GetNamespaceKey(c)
	if namespace == "" || key == "" {
		return
	}
	dbKey := utils.CreateKey(c, namespace, key, false)
	if dbKey == "" { // error is handled in CreateKey
		return
	}
	initialValue, err := strconv.Atoi(c.DefaultQuery("initializer", "0"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "initializer must be a number"})
		return
	}
	// Get data from Redis
	created := Client.SetNX(context.Background(), dbKey, initialValue, utils.BaseTTLPeriod)
	if created.Val() == false {
		c.JSON(http.StatusConflict, gin.H{"error": "Key already exists, please use a different key."})
		return
	}
	AdminKey := uuid.New().String()                                            // Create a new admin key used for deletion and control
	Client.Set(context.Background(), utils.CreateAdminKey(dbKey), AdminKey, 0) // todo: figure out how to handle admin keys (handle alongside admin orrrrrrr separately as in a routine once a month that deletes all admin keys with no corresponding key)
	c.JSON(http.StatusCreated, gin.H{"key": key, "namespace": namespace, "admin_key": AdminKey, "value": initialValue})
}

func InfoView(c *gin.Context) { // todo: write docs on what negative values mean (https://redis.io/commands/ttl/)
	namespace, key := utils.GetNamespaceKey(c)
	if namespace == "" || key == "" {
		return
	}
	dbKey := utils.CreateKey(c, namespace, key, true)
	if dbKey == "" { // error is handled in CreateKey
		return
	}
	dbValue := Client.Get(context.Background(), dbKey).Val()
	count, _ := strconv.Atoi(dbValue)

	isGenuine := Client.Exists(context.Background(), utils.CreateAdminKey(dbKey)).Val() == 0
	expiresAt := Client.TTL(context.Background(), dbKey).Val()
	exists := expiresAt != -2
	if !exists {
		count = -1
	}
	c.JSON(http.StatusOK, gin.H{"value": count, "full_key": dbKey, "is_genuine": isGenuine, "expires_in": expiresAt.Seconds(), "expires_str": expiresAt.String(), "exists": exists})
}

func DeleteView(c *gin.Context) {
	namespace, key := utils.GetNamespaceKey(c)
	if namespace == "" || key == "" {
		return
	}
	dbKey := utils.CreateKey(c, namespace, key, true)
	if dbKey == "" { // error is handled in CreateKey
		return
	}
	adminDBKey := utils.CreateAdminKey(dbKey)    // Create the admin key
	Client.Del(context.Background(), dbKey)      // Delete the normal key
	Client.Del(context.Background(), adminDBKey) // delete the admin key as it's now useless
	c.JSON(http.StatusOK, gin.H{"status": "ok", "message": "Deleted key: " + dbKey})
	// Close SSE stream
	mutex.Lock()
	ch, ok := subscriberMap[dbKey]
	mutex.Unlock()
	if ok {
		close(ch)
		delete(subscriberMap, dbKey)
	}
}

func SetView(c *gin.Context) {
	updatedValueRaw, _ := c.GetQuery("value")
	if updatedValueRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value is required, please provide a number in the fmt of ?value=NEW_VALUE"})
		return

	}
	updatedValue, err := strconv.Atoi(updatedValueRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value must be a number"})
		return
	}
	namespace, key := utils.GetNamespaceKey(c)
	if namespace == "" || key == "" {
		return
	}
	dbKey := utils.CreateKey(c, namespace, key, false)
	if dbKey == "" { // error is handled in CreateKey
		return
	}

	// Get data from Redis
	val, err := Client.SetXX(context.Background(), dbKey, updatedValue, utils.BaseTTLPeriod).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set data. Try again later."})
		return
	}
	if val == false {
		c.JSON(http.StatusConflict, gin.H{"error": "Key does not exist, please use a different key."})
	} else {
		go SetStream(dbKey, updatedValue)
		c.JSON(http.StatusOK, gin.H{"value": updatedValue})
	}
}

func ResetView(c *gin.Context) {
	namespace, key := utils.GetNamespaceKey(c)
	if namespace == "" || key == "" {
		return
	}
	dbKey := utils.CreateKey(c, namespace, key, false)
	if dbKey == "" { // error is handled in CreateKey
		return
	}

	// Get data from Redis
	val, err := Client.SetXX(context.Background(), dbKey, 0, utils.BaseTTLPeriod).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set data. Try again later."})
		return
	}
	if val == false {
		c.JSON(http.StatusConflict, gin.H{"error": "Key does not exist, please use a different key."})
	} else {
		c.JSON(http.StatusOK, gin.H{"value": 0})
		go SetStream(dbKey, 0)
	}
}

func UpdateByView(c *gin.Context) {
	updatedValueRaw, _ := c.GetQuery("value")
	if updatedValueRaw == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value is required, please provide a number in the fmt of ?value=NEW_VALUE"})
		return

	}
	incrByValue, err := strconv.Atoi(updatedValueRaw)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "value must be a number, this means no floats."})
		return
	}
	if incrByValue == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "changing value by 0 does nothing, please provide a non-zero value in the fmt of ?value=NEW_VALUE"})
		return
	}
	namespace, key := utils.GetNamespaceKey(c)
	if namespace == "" || key == "" {
		return
	}
	dbKey := utils.CreateKey(c, namespace, key, false)
	if dbKey == "" { // error is handled in CreateKey
		return
	}

	exists := Client.Exists(context.Background(), dbKey).Val() == 0
	if exists {
		c.JSON(http.StatusConflict, gin.H{"error": "Key does not exist, please first create it using /create."})
		return
	}

	// Get data from Redis
	val, err := Client.IncrByFloat(context.Background(), dbKey, float64(incrByValue)).Result()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set data. Try again later."})
		return
	}

	c.JSON(http.StatusOK, gin.H{"value": int64(val)})
	go SetStream(dbKey, int(val))
}

func StatsView(c *gin.Context) {
	// get average ttl using INFO

	ctx := context.Background()
	infoStr, err := Client.Info(ctx).Result()
	if err != nil {
		panic(err)
	}

	infoDict := make(map[string]map[string]string)
	sections := strings.Split(infoStr, "\r\n\r\n")

	for _, section := range sections {
		lines := strings.Split(section, "\r\n")
		sectionName := lines[0][2:] // Remove "# " prefix

		infoDict[sectionName] = make(map[string]string)
		for _, line := range lines[1:] {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				infoDict[sectionName][key] = value
			}
		}
	}

	total, _ := strconv.Atoi(Client.Get(ctx, "stats:Total").Val())

	hits, _ := strconv.Atoi(Client.Get(ctx, "stats:hit").Val())
	gets, _ := strconv.Atoi(Client.Get(ctx, "stats:get").Val())

	create, _ := strconv.Atoi(Client.Get(ctx, "stats:create").Val())

	totalKeys := create + (hits / 8)

	c.JSON(http.StatusOK, gin.H{
		"version":                     Version,
		"uptime":                      time.Since(StartTime).String(),
		"db_uptime":                   infoDict["Server"]["uptime_in_seconds"],
		"db_version":                  infoDict["Server"]["redis_version"],
		"expired_keys__since_restart": infoDict["Stats"]["expired_keys"],
		"key_misses__since_restart":   infoDict["Stats"]["keyspace_misses"],
		"commands": gin.H{
			"total":  total,
			"get":    gets,
			"hit":    hits,
			"create": create,
		},
		"total_keys": totalKeys,
		"shard":      Shard,
	})
}
