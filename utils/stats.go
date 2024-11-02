package utils

import (
	"context"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	Total       int64 = 0
	CommonStats       = sync.Map{}

	ServerClose = make(chan struct{})
)

func saveStats(client *redis.Client) {
	totalCopy := atomic.SwapInt64(&Total, 0)
	pipe := client.Pipeline()
	pipe.IncrBy(context.Background(), "stats:Total", totalCopy) // Capitalized to avoid conflict with a potential key named "total"
	CommonStats.Range(func(key, value interface{}) bool {
		oldValue, _ := CommonStats.Swap(key, new(int64))
		oldValueNonPtr := *oldValue.(*int64)
		pipe.IncrBy(context.Background(), "stats:"+key.(string), oldValueNonPtr)
		return true
	})
	_, err := pipe.Exec(context.Background())
	if err != nil {
		log.Printf("Error saving stats: %v", err)
	}
}

func InitializeStats(client *redis.Client) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				saveStats(client)

			case <-ServerClose:
				ticker.Stop()
				log.Println("Saving stats... Closing stats goroutine. Goodbye!")
				saveStats(client) // save the stats one last time before closing

				return
			}
		}
	}()
}
