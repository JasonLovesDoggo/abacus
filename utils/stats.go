package utils

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goccy/go-json"

	"github.com/redis/go-redis/v9"
)

const (
	batchSize              uint16 = 150
	maxPaths               uint64 = 120
	bufferWarningThreshold uint16 = uint16(float32(batchSize) * 0.5)
	// 50% of buffer capacity
	totalWarningThreshold uint16 = uint16(0.8 * float32(maxPaths)) // 80% of max paths

)

var (
	Total        int64 = 0
	ServerClose        = make(chan bool, 1)
	StatsManager *StatManager
)

type StatManager struct {
	stats     *sync.Map
	buffer    chan statsEntry
	pathCount atomic.Int64
	client    *redis.Client
	saveMutex sync.Mutex
}

type statsEntry struct {
	path      string
	count     int64
	timestamp time.Time
}

type StatsSnapshot struct {
	Timestamp  time.Time        `json:"timestamp"`
	Total      int64            `json:"total"`
	PathCount  int64            `json:"path_count"`
	BufferSize int              `json:"buffer_size"`
	PathStats  map[string]int64 `json:"path_stats"`
}

func NewStatsManager(client *redis.Client) *StatManager {
	sm := &StatManager{
		stats:  &sync.Map{},
		buffer: make(chan statsEntry, batchSize),
		client: client,
	}

	go sm.processBuffer()
	go sm.monitorHealth()

	return sm
}

func (sm *StatManager) getStatsSnapshot() *StatsSnapshot {
	snapshot := &StatsSnapshot{
		Timestamp:  time.Now(),
		Total:      atomic.LoadInt64(&Total),
		PathCount:  sm.pathCount.Load(),
		BufferSize: len(sm.buffer),
		PathStats:  make(map[string]int64),
	}

	sm.stats.Range(func(key, value interface{}) bool {
		snapshot.PathStats[key.(string)] = atomic.LoadInt64(value.(*int64))
		return true
	})

	return snapshot
}

func (sm *StatManager) saveStatsToRedis(force bool) {
	sm.saveMutex.Lock()
	defer sm.saveMutex.Unlock()

	if !force && len(sm.buffer) < int(bufferWarningThreshold) {
		return
	}

	totalCopy := atomic.SwapInt64(&Total, 0)
	ctx := context.Background()

	pipe := sm.client.Pipeline()

	if totalCopy > 0 {
		pipe.IncrBy(ctx, "stats:Total", totalCopy)
	}

	statsSnapshot := make(map[string]int64)

	sm.stats.Range(func(key, value interface{}) bool {
		oldValue := atomic.SwapInt64(value.(*int64), 0)
		if oldValue > 0 {
			statsSnapshot[key.(string)] = oldValue
			pipe.IncrBy(ctx, "stats:"+key.(string), oldValue)
		}
		return true
	})

	log.Printf("Saving stats to Redis (forced: %v, buffer size: %d/%d):\n%+v",
		force, len(sm.buffer), batchSize, statsSnapshot)

	_, err := pipe.Exec(ctx)
	if err != nil {
		errorMsg := fmt.Sprintf("Error saving stats to Redis: %v\nFailed stats dump:\n", err)
		errorMsg += fmt.Sprintf("Total: %d\n", totalCopy)
		for path, count := range statsSnapshot {
			errorMsg += fmt.Sprintf("%s: %d\n", path, count)
		}
		log.Printf(errorMsg)

		atomic.AddInt64(&Total, totalCopy)
		for path, count := range statsSnapshot {
			if val, ok := sm.stats.Load(path); ok {
				atomic.AddInt64(val.(*int64), count)
			}
		}
	}
}

func (sm *StatManager) monitorHealth() {
	ticker := time.NewTicker(1 * time.Minute)
	for {
		<-ticker.C
		snapshot := sm.getStatsSnapshot()
		stats, _ := json.MarshalIndent(snapshot, "", "  ")
		log.Printf("Stats Health Check:\n%s", string(stats))

		// Only trigger save if there's actually something in the buffer
		// or if the total is high enough to warrant a save
		bufferNearCapacity := len(sm.buffer) > int(bufferWarningThreshold)
		totalHighButBufferNotEmpty := snapshot.Total >= int64(totalWarningThreshold) && len(sm.buffer) > 0

		if bufferNearCapacity || totalHighButBufferNotEmpty {
			log.Printf("Buffer (%d/%d) or total count (%d/%d) reaching capacity with data to save. Triggering save operation.",
				len(sm.buffer), batchSize, snapshot.Total, totalWarningThreshold)
			sm.saveStatsToRedis(true)
		} else if snapshot.Total >= int64(totalWarningThreshold) {
			// Log that we're skipping save despite high total because buffer is empty
			log.Printf("Total count high (%d/%d) but buffer is empty. Skipping unnecessary save operation.",
				snapshot.Total, totalWarningThreshold)
		} else {
			log.Printf("Buffer (%d/%d) and total count (%d/%d) are within acceptable limits. Skipping save operation.",
				len(sm.buffer), batchSize, snapshot.Total, totalWarningThreshold)
		}
	}
}
func (sm *StatManager) processBuffer() {
	for entry := range sm.buffer {
		if len(sm.buffer) > int(bufferWarningThreshold) {
			go sm.saveStatsToRedis(false)
		}

		val, loaded := sm.stats.Load(entry.path)
		if !loaded {
			if sm.pathCount.Load() >= int64(maxPaths) {
				if sm.pathCount.Load()%100 == 0 {
					snapshot := sm.getStatsSnapshot()
					stats, _ := json.MarshalIndent(snapshot, "", "  ")
					log.Printf("WARNING: Path limit exceeded. Current stats:\n%s", string(stats))
				}
				val, _ = sm.stats.LoadOrStore("overflow", new(int64))
			} else {
				val = new(int64)
				sm.stats.Store(entry.path, val)
				sm.pathCount.Add(1)
				log.Printf("New path added: %s (Total paths: %d)", entry.path, sm.pathCount.Load())
			}
		}
		atomic.AddInt64(val.(*int64), entry.count)
	}
}

func (sm *StatManager) RecordStat(path string, count int64) {
	entry := statsEntry{
		path:      path,
		count:     count,
		timestamp: time.Now(),
	}

	if len(sm.buffer) > int(bufferWarningThreshold) {
		go sm.saveStatsToRedis(false)
	}

	sm.buffer <- entry
}

func InitializeStatsManager(client *redis.Client) *StatManager {
	sm := NewStatsManager(client)
	StatsManager = sm

	ticker := time.NewTicker(30 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				sm.saveStatsToRedis(false)
			case _, ok := <-ServerClose:
				ticker.Stop()
				if !ok {
					// Channel was closed, use old behavior
					log.Println("Saving final stats... Channel was closed.")
					sm.saveStatsToRedis(true)
					return
				}

				// New behavior - received sync signal
				log.Println("Saving final stats before shutdown...")
				sm.saveStatsToRedis(true)
				close(sm.buffer)

				// Signal back on the same channel that we're done
				log.Println("Stats saving completed, sending completion signal")
				ServerClose <- true
				return
			}
		}
	}()

	return sm
}
