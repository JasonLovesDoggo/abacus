package utils

import (
	"hash/fnv"
	"log"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

type ValueEvent struct {
	Message       chan KeyValue
	NewClients    chan KeyClientPair
	ClosedClients chan KeyClientPair
	TotalClients  map[string]map[chan int]bool
	Mu            sync.RWMutex

	// Internal optimized implementation
	shards          []*shard
	shardCount      int
	workers         int
	workerPool      chan workerTask
	maxConnections  int32
	activeConns     int32
	droppedMessages int64
	totalMessages   int64
	clientTimeout   time.Duration
	shutdown        chan struct{}
	wg              sync.WaitGroup
}

type KeyValue struct {
	Key   string
	Value int
}

type KeyClientPair struct {
	Key    string
	Client chan int
}

type shard struct {
	mu      sync.RWMutex
	clients map[string]map[chan int]bool
}

type workerTask struct {
	taskType string
	key      string
	value    int
	client   chan int
}

func NewValueEventServer() *ValueEvent {
	// Get configuration from environment with sensible defaults
	workers := getEnvInt("SSE_WORKER_COUNT", runtime.NumCPU())
	bufferSize := getEnvInt("SSE_BUFFER_SIZE", 1000)
	maxConns := getEnvInt("MAX_SSE_CONNECTIONS", 100000)
	clientTimeout := time.Duration(getEnvInt("SSE_CLIENT_TIMEOUT_MS", 1000)) * time.Millisecond
	shardCount := getEnvInt("SSE_SHARD_COUNT", 32)

	event := &ValueEvent{
		// Public channels for compatibility
		Message:       make(chan KeyValue, bufferSize),
		NewClients:    make(chan KeyClientPair, bufferSize),
		ClosedClients: make(chan KeyClientPair, bufferSize),
		TotalClients:  make(map[string]map[chan int]bool),

		// Optimized implementation
		shardCount:     shardCount,
		workers:        workers,
		workerPool:     make(chan workerTask, bufferSize*2),
		maxConnections: int32(maxConns),
		clientTimeout:  clientTimeout,
		shutdown:       make(chan struct{}),
	}

	// Initialize shards
	event.shards = make([]*shard, shardCount)
	for i := 0; i < shardCount; i++ {
		event.shards[i] = &shard{
			clients: make(map[string]map[chan int]bool),
		}
	}

	// Start worker pool
	for i := 0; i < workers; i++ {
		event.wg.Add(1)
		go event.worker(i)
	}

	// Start dispatcher
	event.wg.Add(1)
	go event.dispatcher()

	// Start compatibility sync
	go event.syncTotalClients()

	log.Printf("SSE server started with %d workers, %d shards, max %d connections",
		workers, shardCount, maxConns)

	return event
}

func (v *ValueEvent) getShard(key string) *shard {
	h := fnv.New32a()
	h.Write([]byte(key))
	return v.shards[h.Sum32()%uint32(v.shardCount)]
}

func (v *ValueEvent) dispatcher() {
	defer v.wg.Done()

	for {
		select {
		case <-v.shutdown:
			close(v.workerPool)
			return

		case newClient := <-v.NewClients:
			// Check connection limit
			currentConns := atomic.LoadInt32(&v.activeConns)
			if currentConns >= v.maxConnections {
				log.Printf("Connection limit reached (%d/%d), rejecting new client for key %s",
					currentConns, v.maxConnections, newClient.Key)
				close(newClient.Client)
				continue
			}

			atomic.AddInt32(&v.activeConns, 1)
			v.workerPool <- workerTask{
				taskType: "add",
				key:      newClient.Key,
				client:   newClient.Client,
			}

		case closedClient := <-v.ClosedClients:
			v.workerPool <- workerTask{
				taskType: "remove",
				key:      closedClient.Key,
				client:   closedClient.Client,
			}

		case keyValue := <-v.Message:
			atomic.AddInt64(&v.totalMessages, 1)
			v.broadcastMessage(keyValue)
		}
	}
}

func (v *ValueEvent) worker(id int) {
	defer v.wg.Done()

	for task := range v.workerPool {
		switch task.taskType {
		case "add":
			v.addClient(task.key, task.client)
		case "remove":
			v.removeClient(task.key, task.client)
		case "send":
			v.sendToClient(task.client, task.value)
		}
	}
}

func (v *ValueEvent) addClient(key string, client chan int) {
	shard := v.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if _, exists := shard.clients[key]; !exists {
		shard.clients[key] = make(map[chan int]bool)
	}
	shard.clients[key][client] = true
	log.Printf("Client added for key %s. Total clients: %d", key, len(shard.clients[key]))
}

func (v *ValueEvent) removeClient(key string, client chan int) {
	shard := v.getShard(key)
	shard.mu.Lock()
	defer shard.mu.Unlock()

	if clients, exists := shard.clients[key]; exists {
		if _, ok := clients[client]; ok {
			delete(clients, client)
			atomic.AddInt32(&v.activeConns, -1)

			// Don't close the channel here - let the route handler manage it
			// The channel will be closed when the HTTP handler exits

			log.Printf("Removed client for key %s", key)

			if len(clients) == 0 {
				delete(shard.clients, key)
				log.Printf("No more clients for key %s, removed key entry", key)
			}
		}
	}
}

func (v *ValueEvent) broadcastMessage(kv KeyValue) {
	shard := v.getShard(kv.Key)

	// Get clients snapshot
	shard.mu.RLock()
	clients, exists := shard.clients[kv.Key]
	if !exists || len(clients) == 0 {
		shard.mu.RUnlock()
		return
	}

	clientList := make([]chan int, 0, len(clients))
	for client := range clients {
		clientList = append(clientList, client)
	}
	shard.mu.RUnlock()

	// Parallel broadcast with batching
	var wg sync.WaitGroup
	batchSize := len(clientList) / v.workers
	if batchSize < 1 {
		batchSize = 1
	}

	for i := 0; i < len(clientList); i += batchSize {
		end := i + batchSize
		if end > len(clientList) {
			end = len(clientList)
		}

		wg.Add(1)
		go func(clients []chan int) {
			defer wg.Done()
			for _, client := range clients {
				v.sendToClient(client, kv.Value)
			}
		}(clientList[i:end])
	}

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All messages sent
	case <-time.After(100 * time.Millisecond):
		log.Printf("Broadcast timeout for key %s (sending to %d clients)", kv.Key, len(clientList))
	}
}

func (v *ValueEvent) sendToClient(client chan int, value int) {
	select {
	case client <- value:
		// Successfully sent
	case <-time.After(v.clientTimeout):
		atomic.AddInt64(&v.droppedMessages, 1)
	default:
		// Channel full, drop message
		atomic.AddInt64(&v.droppedMessages, 1)
	}
}

// Maintain compatibility with old code that reads TotalClients
func (v *ValueEvent) syncTotalClients() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-v.shutdown:
			return
		case <-ticker.C:
			v.updateTotalClients()
		}
	}
}

func (v *ValueEvent) updateTotalClients() {
	allClients := make(map[string]map[chan int]bool)

	for _, s := range v.shards {
		s.mu.RLock()
		for key, clients := range s.clients {
			if _, exists := allClients[key]; !exists {
				allClients[key] = make(map[chan int]bool)
			}
			for client := range clients {
				allClients[key][client] = true
			}
		}
		s.mu.RUnlock()
	}

	v.Mu.Lock()
	v.TotalClients = allClients
	v.Mu.Unlock()
}

func (v *ValueEvent) CountClientsForKey(key string) int {
	shard := v.getShard(key)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	if clients, exists := shard.clients[key]; exists {
		return len(clients)
	}
	return 0
}

func (v *ValueEvent) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"active_connections": atomic.LoadInt32(&v.activeConns),
		"max_connections":    v.maxConnections,
		"dropped_messages":   atomic.LoadInt64(&v.droppedMessages),
		"total_messages":     atomic.LoadInt64(&v.totalMessages),
		"workers":            v.workers,
		"shards":             v.shardCount,
	}
}

// ValueEventServer is the global event server instance
var ValueEventServer *ValueEvent

func init() {
	ValueEventServer = NewValueEventServer()
}

// SetStream sends a value update to all clients subscribed to the given key
func SetStream(dbKey string, newValue int) {
	// Use non-blocking send to prevent blocking
	select {
	case ValueEventServer.Message <- KeyValue{Key: dbKey, Value: newValue}:
		// Message sent successfully
	default:
		log.Printf("Warning: Message channel full, update for key %s dropped", dbKey)
	}
}

func CloseStream(dbKey string) {
	// Get all clients for this key across all shards
	shard := ValueEventServer.getShard(dbKey)

	clientCount := 0

	shard.mu.Lock()
	if clients, exists := shard.clients[dbKey]; exists {
		clientCount = len(clients)
		// Just remove from map, don't close channels
		// Channels are managed by the HTTP handlers
		delete(shard.clients, dbKey)
	}
	shard.mu.Unlock()

	if clientCount > 0 {
		log.Printf("Removed all stream clients for key %s (%d clients)", dbKey, clientCount)
	}
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
