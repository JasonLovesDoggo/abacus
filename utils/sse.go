package utils

import (
	"log"
	"sync"
	"time"
)

type ValueEvent struct {
	Message       chan KeyValue
	NewClients    chan KeyClientPair
	ClosedClients chan KeyClientPair
	TotalClients  map[string]map[chan int]bool
	Mu            sync.RWMutex
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
		// Use buffered channels to prevent blocking
		Message:       make(chan KeyValue, 20000),
		NewClients:    make(chan KeyClientPair, 5000),
		ClosedClients: make(chan KeyClientPair, 5000),
		TotalClients:  make(map[string]map[chan int]bool),
	}
	go event.listen()
	return event
}

func (v *ValueEvent) listen() {
	for {
		select {
		case newClient := <-v.NewClients:
			v.Mu.Lock()
			if _, exists := v.TotalClients[newClient.Key]; !exists {
				v.TotalClients[newClient.Key] = make(map[chan int]bool)
			}
			v.TotalClients[newClient.Key][newClient.Client] = true
			v.Mu.Unlock()
			log.Printf("Client added for key %s. Total clients: %d", newClient.Key, len(v.TotalClients[newClient.Key]))

		case closedClient := <-v.ClosedClients:
			v.Mu.Lock()
			if clients, exists := v.TotalClients[closedClient.Key]; exists {
				if _, ok := clients[closedClient.Client]; ok {
					delete(clients, closedClient.Client)

					// Close channel safely
					close(closedClient.Client)

					log.Printf("Removed client for key %s", closedClient.Key)

					// Clean up key map if no more clients
					if len(clients) == 0 {
						delete(v.TotalClients, closedClient.Key)
						log.Printf("No more clients for key %s, removed key entry", closedClient.Key)
					}
				}
			}
			v.Mu.Unlock()

		case keyValue := <-v.Message:
			// First, get a snapshot of clients under read lock
			v.Mu.RLock()
			clients, exists := v.TotalClients[keyValue.Key]
			if !exists || len(clients) == 0 {
				v.Mu.RUnlock()
				continue
			}

			// Create a safe copy of client channels
			clientChannels := make([]chan int, 0, len(clients))
			for clientChan := range clients {
				clientChannels = append(clientChannels, clientChan)
			}
			v.Mu.RUnlock()

			// Send messages without holding the lock
			// Use non-blocking sends for better performance
			var failedClients []chan int
			for _, clientChan := range clientChannels {
				select {
				case clientChan <- keyValue.Value:
					// Message sent successfully
				default:
					// Channel full, client is slow - mark for removal
					failedClients = append(failedClients, clientChan)
				}
			}

			// Schedule removal of failed clients
			for _, failedClient := range failedClients {
				select {
				case v.ClosedClients <- KeyClientPair{Key: keyValue.Key, Client: failedClient}:
					// Client scheduled for removal
				default:
					// If ClosedClients channel is full, try again later
					go func(key string, client chan int) {
						time.Sleep(200 * time.Millisecond)
						select {
						case v.ClosedClients <- KeyClientPair{Key: key, Client: client}:
							// Success on retry
						default:
							log.Printf("Failed to remove client for key %s even after retry", key)
						}
					}(keyValue.Key, failedClient)
				}
			}
		}
	}
}

func (v *ValueEvent) CountClientsForKey(key string) int {
	v.Mu.RLock()
	defer v.Mu.RUnlock()

	if clients, exists := v.TotalClients[key]; exists {
		return len(clients)
	}
	return 0
}

// Global event server
var ValueEventServer *ValueEvent

func init() {
	ValueEventServer = NewValueEventServer()
}

// When you want to update a value and notify clients for a specific key
func SetStream(dbKey string, newValue int) {
	// Use a non-blocking send with default case to prevent blocking
	select {
	case ValueEventServer.Message <- KeyValue{Key: dbKey, Value: newValue}:
		// Message sent successfully
	default:
		log.Printf("Warning: Message channel full, update for key %s dropped", dbKey)
	}
}

func CloseStream(dbKey string) {
	// First collect all channels to be closed while holding the lock
	var channelsToClose []chan int

	ValueEventServer.Mu.Lock()
	if clients, exists := ValueEventServer.TotalClients[dbKey]; exists {
		// Create a copy of all channels we need to close
		for clientChan := range clients {
			channelsToClose = append(channelsToClose, clientChan)
		}
		// Remove the entry from the map
		delete(ValueEventServer.TotalClients, dbKey)
	}
	ValueEventServer.Mu.Unlock()

	// Now close the channels after releasing the lock
	for _, ch := range channelsToClose {
		close(ch)
	}

	if len(channelsToClose) > 0 {
		log.Printf("Closed all streams for key %s (%d clients)", dbKey, len(channelsToClose))
	}
}
