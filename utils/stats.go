package utils

import (
	"context"
	"fmt"
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

	fmt.Println("Saving stats...")
	totalCopy := atomic.SwapInt64(&Total, 0)

	fmt.Println("swapped totalCopy: ", totalCopy)

	client.IncrBy(context.Background(), "stats:Total", totalCopy) // Capitalized to avoid conflict with a potential key named "total"
	fmt.Println("Incremented totalCopy")
	CommonStats.Range(func(key, value interface{}) bool {
		fmt.Println("key: ", key)
		oldValue, _ := CommonStats.Swap(key, new(int64))
		oldValueNonPtr := *oldValue.(*int64)
		fmt.Println("swapped - oldValue: ", oldValueNonPtr)
		client.IncrBy(context.Background(), "stats:"+key.(string), oldValueNonPtr)
		return true

	})
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
