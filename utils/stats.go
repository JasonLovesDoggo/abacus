package utils

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/goccy/go-json"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// How many paths to track before panicking
	maxPaths = 45
	// Threshold for total count to trigger save
	saveThreshold = 100
)

var (
	Total        int64 = 0
	ServerClose        = make(chan bool, 1)
	StatsManager *StatManager
	logger, _           = zap.NewProduction()
)

// StatManager handles collecting and saving statistics
type StatManager struct {
	stats     *sync.Map     // Thread-safe map for path stats
	pathCount atomic.Int64  // Number of unique paths being tracked
	client    *redis.Client // Redis client for persistence
	saveMutex sync.Mutex    // Mutex for thread-safe saves
}

// NewStatsManager creates a new stats manager
func NewStatsManager(client *redis.Client) *StatManager {
	sm := &StatManager{
		stats:  &sync.Map{},
		client: client,
	}

	// Start background save timer
	go sm.periodicSave()
	// Start health check timer
	go sm.periodicHealthCheck()

	return sm
}

// RecordStat records a stat for a path
func (sm *StatManager) RecordStat(path string, count int64) {
	// Update total counter atomically
	atomic.AddInt64(&Total, count)

	// Get or create counter for this path
	val, loaded := sm.stats.Load(path)
	if !loaded {
		// Check if we've hit the path limit
		if sm.pathCount.Load() >= maxPaths {
			// Panic instead of using overflow bucket
			panic(fmt.Sprintf("Stats path limit exceeded: %d paths is the maximum allowed... if you see this, "+
				"please make a issue @ https://github.com/JasonLovesDoggo/abacus or raise stats.maxPaths",
				maxPaths))
		}

		// Create new counter
		val = new(int64)
		sm.stats.Store(path, val)
		sm.pathCount.Add(1)
	}

	// Update path counter atomically
	atomic.AddInt64(val.(*int64), count)

	// Save if total exceeds threshold
	if atomic.LoadInt64(&Total) > saveThreshold {
		go sm.saveStats(false)
	}
}

// periodicSave saves stats every 30 seconds
func (sm *StatManager) periodicSave() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			sm.saveStats(false)
		case _, ok := <-ServerClose:
			if !ok {
				// Channel was closed
				sm.saveStats(true)
				return
			}
			// Received shutdown signal
			sm.saveStats(true)
			// Signal completion
			ServerClose <- true
			return
		}
	}
}

// periodicHealthCheck logs stats every minute
func (sm *StatManager) periodicHealthCheck() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.logStats()
	}
}

// saveStats saves current stats to Redis
func (sm *StatManager) saveStats(force bool) {
	// Skip if total count is low and not forced
	totalCount := atomic.LoadInt64(&Total)
	if !force && totalCount < saveThreshold {
		return
	}

	// Prevent concurrent saves
	sm.saveMutex.Lock()
	defer sm.saveMutex.Unlock()

	// Get current total and reset to 0
	totalCopy := atomic.SwapInt64(&Total, 0)
	if totalCopy == 0 {
		return // Nothing to save
	}

	ctx := context.Background()
	pipe := sm.client.Pipeline()
	pipe.IncrBy(ctx, "stats:Total", totalCopy)

	// Collect all path stats atomically
	pathStats := make(map[string]int64)
	sm.stats.Range(func(key, value interface{}) bool {
		path := key.(string)
		// Swap to get current value and reset to 0
		count := atomic.SwapInt64(value.(*int64), 0)
		if count > 0 {
			pathStats[path] = count
			pipe.IncrBy(ctx, "stats:"+path, count)
		}
		return true
	})

	// Execute Redis pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		logger.Error("Error saving stats", zap.Error(err))
		atomic.AddInt64(&Total, totalCopy)

		for path, count := range pathStats {
			if val, ok := sm.stats.Load(path); ok {
				atomic.AddInt64(val.(*int64), count)
			}
		}
	} else {
		logger.Info("Stats saved", zap.Int64("total", totalCopy), zap.Int("paths", len(pathStats)))
	}
}

// logStats outputs current stats for monitoring
func (sm *StatManager) logStats() {
	snapshot := &struct {
		Timestamp time.Time        `json:"timestamp"`
		Total     int64            `json:"total"`
		PathCount int64            `json:"path_count"`
		PathStats map[string]int64 `json:"path_stats"`
	}{
		Timestamp: time.Now(),
		Total:     atomic.LoadInt64(&Total),
		PathCount: sm.pathCount.Load(),
		PathStats: make(map[string]int64),
	}

	sm.stats.Range(func(key, value interface{}) bool {
		path := key.(string)
		count := atomic.LoadInt64(value.(*int64))
		snapshot.PathStats[path] = count
		return true
	})

	stats, _ := json.MarshalIndent(snapshot, "", "  ")
	logger.Info("Stats Health Check", zap.String("snapshot", string(stats)))

	if snapshot.Total >= saveThreshold {
		logger.Info("High total count, triggering save", zap.Int64("total", snapshot.Total), zap.Int("threshold", saveThreshold))
		go sm.saveStats(true)
	}
}

// InitializeStatsManager creates global stats manager
func InitializeStatsManager(client *redis.Client) *StatManager {
	StatsManager = NewStatsManager(client)
	return StatsManager
}
