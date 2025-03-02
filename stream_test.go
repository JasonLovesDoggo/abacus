package main

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jasonlovesdoggo/abacus/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCloseNotifier implements http.CloseNotifier for testing
type MockResponseRecorder struct {
	*httptest.ResponseRecorder
	closeNotify chan bool
}

// NewMockResponseRecorder creates a new response recorder with CloseNotify support
func NewMockResponseRecorder() *MockResponseRecorder {
	return &MockResponseRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		closeNotify:      make(chan bool, 1),
	}
}

// CloseNotify implements http.CloseNotifier
func (m *MockResponseRecorder) CloseNotify() <-chan bool {
	return m.closeNotify
}

// Close simulates a client disconnection
func (m *MockResponseRecorder) Close() {
	select {
	case m.closeNotify <- true:
		// Signal sent
	default:
		// Channel already has a value or is closed
	}
}

// TestStreamBasicFunctionality tests that the stream endpoint correctly
// sends events when values are updated
func TestStreamBasicFunctionality(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupTestRouter()

	// Create a counter first
	createResp := httptest.NewRecorder()
	createReq, _ := http.NewRequest("POST", "/create/test/stream-test", nil)
	router.ServeHTTP(createResp, createReq)
	assert.Equal(t, http.StatusCreated, createResp.Code)

	// For streaming tests, we need a real HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	// Use a real HTTP client to connect to the server
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequest("GET", server.URL+"/stream/test/stream-test", nil)
	require.NoError(t, err)

	req.Header.Set("Accept", "text/event-stream")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Channel to collect received events
	events := make(chan string, 10)
	done := make(chan struct{})

	// Process the SSE stream
	go func() {
		defer close(done)
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				select {
				case events <- line:
					// Event sent
				case <-time.After(100 * time.Millisecond):
					// Buffer full, drop event
					t.Logf("Event buffer full, dropped: %s", line)
				}
			}
		}
		if err := scanner.Err(); err != nil {
			t.Logf("Scanner error: %v", err)
		}
	}()

	// Wait for initial value
	select {
	case event := <-events:
		assert.True(t, strings.HasPrefix(event, "data: {\"value\":"))
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for initial event")
	}

	// Hit the counter to increment its value
	hitResp, err := client.Get(server.URL + "/hit/test/stream-test")
	require.NoError(t, err)
	hitResp.Body.Close()
	assert.Equal(t, http.StatusOK, hitResp.StatusCode)

	// Check that we got an update event
	select {
	case event := <-events:
		assert.True(t, strings.HasPrefix(event, "data: {\"value\":"))

		// Extract the value
		value := extractValueFromEvent(event)
		assert.Equal(t, 1, value)
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for update event")
	}

	// Close connection
	resp.Body.Close()

	// Give some time for cleanup
	time.Sleep(500 * time.Millisecond)

	// Verify proper cleanup
	clientCount := countClientsForKey("K:test:stream-test")
	assert.Equal(t, 0, clientCount, "Client wasn't properly cleaned up after disconnection")
}

// TestMultipleClients tests multiple clients connecting to the same stream
func TestMultipleClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := setupTestRouter()

	// Create a counter
	createResp := httptest.NewRecorder()
	createReq, _ := http.NewRequest("POST", "/create/test/multi-client", nil)
	router.ServeHTTP(createResp, createReq)
	assert.Equal(t, http.StatusCreated, createResp.Code)

	// Start a real HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	// Number of clients to test
	numClients := 3 // Reduced from 5 for faster testing

	// Set up client trackers
	type clientState struct {
		resp       *http.Response
		events     chan string
		done       chan struct{}
		lastValue  int
		eventCount int
	}

	clients := make([]*clientState, numClients)

	// Start all clients
	for i := 0; i < numClients; i++ {
		// Create client state
		clients[i] = &clientState{
			events: make(chan string, 10),
			done:   make(chan struct{}),
		}

		// Create request
		req, err := http.NewRequest("GET", server.URL+"/stream/test/multi-client", nil)
		require.NoError(t, err)
		req.Header.Set("Accept", "text/event-stream")

		// Connect client
		client := &http.Client{
			Timeout: 5 * time.Second,
		}
		resp, err := client.Do(req)
		require.NoError(t, err)
		clients[i].resp = resp

		// Process events
		go func(idx int) {
			defer close(clients[idx].done)
			scanner := bufio.NewScanner(clients[idx].resp.Body)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "data: ") {
					select {
					case clients[idx].events <- line:
						// Event sent
					default:
						// Buffer full, drop event
					}
				}
			}
		}(i)
	}

	// Give time for all clients to connect
	time.Sleep(300 * time.Millisecond)

	// Verify all clients receive initial value
	for i := 0; i < numClients; i++ {
		select {
		case event := <-clients[i].events:
			clients[i].lastValue = extractValueFromEvent(event)
			clients[i].eventCount++
		case <-time.After(1 * time.Second):
			t.Fatalf("Timeout waiting for client %d initial event", i)
		}
	}

	// Hit the counter several times
	for hits := 0; hits < 3; hits++ {
		client := &http.Client{}
		resp, err := client.Get(server.URL + "/hit/test/multi-client")
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Give time for events to propagate
		time.Sleep(100 * time.Millisecond)
	}

	// Verify all clients received the updates
	for i := 0; i < numClients; i++ {
		// Drain all events
		timeout := time.After(500 * time.Millisecond)
		draining := true

		for draining {
			select {
			case event := <-clients[i].events:
				clients[i].lastValue = extractValueFromEvent(event)
				clients[i].eventCount++
			case <-timeout:
				draining = false
			}
		}

		// Each client should have received at least 4 events (initial + 3 hits)
		assert.GreaterOrEqual(t, clients[i].eventCount, 4, "Client %d didn't receive enough events", i)
		assert.Equal(t, 3, clients[i].lastValue, "Client %d has incorrect final value", i)
	}

	// Disconnect clients one by one and verify cleanup
	for i := 0; i < numClients; i++ {
		// Close client connection
		clients[i].resp.Body.Close()

		// Give time for cleanup
		time.Sleep(200 * time.Millisecond)

		// Verify decreasing client count
		clientCount := countClientsForKey("K:test:multi-client")
		assert.Equal(t, numClients-(i+1), clientCount, "Client wasn't properly cleaned up after disconnection")
	}
}

// TestStreamConcurrencyStress tests the stream under high concurrency conditions
func TestStreamConcurrencyStress(t *testing.T) {
	// Skip in normal testing as this is a long stress test
	if testing.Short() {
		t.Skip("Skipping stress test in short mode")
	}

	gin.SetMode(gin.ReleaseMode) // Reduce logging noise
	router := setupTestRouter()

	// Create a counter for stress testing
	createResp := httptest.NewRecorder()
	createReq, _ := http.NewRequest("POST", "/create/test/stress-test", nil)
	router.ServeHTTP(createResp, createReq)
	require.Equal(t, http.StatusCreated, createResp.Code)

	// Start a real HTTP server
	server := httptest.NewServer(router)
	defer server.Close()

	// Test parameters
	numClients := 20 // Reduced from 50 for faster testing
	clientDuration := 300 * time.Millisecond

	// Start with no clients
	initialCount := countClientsForKey("K:test:stress-test")
	assert.Equal(t, 0, initialCount)

	// Launch many concurrent clients
	var wg sync.WaitGroup
	for i := 0; i < numClients; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Create client
			client := &http.Client{}

			// Create request
			req, err := http.NewRequest("GET", server.URL+"/stream/test/stress-test", nil)
			if err != nil {
				t.Logf("Error creating request: %v", err)
				return
			}
			req.Header.Set("Accept", "text/event-stream")

			// Send request
			resp, err := client.Do(req)
			if err != nil {
				t.Logf("Error connecting: %v", err)
				return
			}

			// Keep connection open for the duration
			time.Sleep(clientDuration)

			// Close connection
			resp.Body.Close()
		}(i)

		// Stagger client creation slightly
		time.Sleep(5 * time.Millisecond)
	}

	// Wait for all clients to finish
	wg.Wait()

	// Give extra time for any cleanup
	time.Sleep(1 * time.Second)

	// Verify all clients were cleaned up
	finalCount := countClientsForKey("K:test:stress-test")
	assert.Equal(t, 0, finalCount, "Not all clients were cleaned up after stress test")

	// Check we can still connect new clients
	client := &http.Client{}
	req, err := http.NewRequest("GET", server.URL+"/stream/test/stress-test", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	// Give time for connection
	time.Sleep(200 * time.Millisecond)

	// Verify new client connected
	newCount := countClientsForKey("K:test:stress-test")
	assert.Equal(t, 1, newCount, "Failed to connect new client after stress test")

	// Clean up
	resp.Body.Close()
	time.Sleep(200 * time.Millisecond)
}

func countClientsForKey(key string) int {
	return utils.ValueEventServer.CountClientsForKey(key)
}

func extractValueFromEvent(event string) int {
	// Format is "data: {"value":X}"
	jsonStr := strings.TrimPrefix(event, "data: ")
	var data struct {
		Value int `json:"value"`
	}
	err := json.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		return -1
	}
	return data.Value
}
