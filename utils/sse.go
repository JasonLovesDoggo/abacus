package utils

import (
	"log"
	"sync"
)

type ValueEvent struct {
	Message       chan KeyValue
	NewClients    chan KeyClientPair
	ClosedClients chan KeyClientPair
	TotalClients  map[string]map[chan int]bool
	mu            sync.RWMutex
}

type KeyValue struct {
	Key   string
	Value int
}

type KeyClientPair struct {
	Key    string
	Client chan int
}

func NewValueEventServer() *ValueEvent {
	event := &ValueEvent{
		Message:       make(chan KeyValue),
		NewClients:    make(chan KeyClientPair),
		ClosedClients: make(chan KeyClientPair),
		TotalClients:  make(map[string]map[chan int]bool),
	}
	go event.listen()
	return event
}

func (v *ValueEvent) listen() {
	for {
		select {
		case newClient := <-v.NewClients:
			v.mu.Lock()
			if _, exists := v.TotalClients[newClient.Key]; !exists {
				v.TotalClients[newClient.Key] = make(map[chan int]bool)
			}
			v.TotalClients[newClient.Key][newClient.Client] = true
			v.mu.Unlock()
			log.Printf("Client added for key %s. Total clients: %d", newClient.Key, len(v.TotalClients[newClient.Key]))

		case closedClient := <-v.ClosedClients:
			v.mu.Lock()
			delete(v.TotalClients[closedClient.Key], closedClient.Client)
			close(closedClient.Client)

			// Clean up key map if no more clients
			if len(v.TotalClients[closedClient.Key]) == 0 {
				delete(v.TotalClients, closedClient.Key)
			}
			v.mu.Unlock()
			log.Printf("Removed client for key %s", closedClient.Key)

		case keyValue := <-v.Message:
			v.mu.RLock()
			for clientChan := range v.TotalClients[keyValue.Key] {
				clientChan <- keyValue.Value
			}
			v.mu.RUnlock()
		}
	}
}

// Global event server
var ValueEventServer *ValueEvent

func init() {
	ValueEventServer = NewValueEventServer()
}

// When you want to update a value and notify clients for a specific key
func SetStream(dbKey string, newValue int) {
	// Broadcast the new value only to clients listening to this specific key
	ValueEventServer.Message <- KeyValue{
		Key:   dbKey,
		Value: newValue,
	}
}

func CloseStream(dbKey string) {
	// Close all client channels for this specific key
	ValueEventServer.mu.Lock()
	if clients, exists := ValueEventServer.TotalClients[dbKey]; exists {
		for clientChan := range clients {
			close(clientChan)
		}
		delete(ValueEventServer.TotalClients, dbKey)
	}
	ValueEventServer.mu.Unlock()
}
