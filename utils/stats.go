package utils

import (
	"context"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
)

var Total int64 = 0

var CommonStats = map[string]int64{}

var ServerClose = make(chan struct{})

func saveStats(client *redis.Client) {
	newTotal := Total
	Total = 0 // reset the total

	newStats := CommonStats
	CommonStats = map[string]int64{} // reset the map

	client.IncrBy(context.Background(), "stats:Total", newTotal) // Capitalized to avoid conflict with a potential key named "total"
	for key, value := range newStats {
		client.IncrBy(context.Background(), "stats:"+key, value)
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
