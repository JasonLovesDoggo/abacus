package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	targetConnections = flag.Int("connections", 10000, "Number of concurrent connections")
	serverURL         = flag.String("url", "http://localhost:8080", "Server URL")
	testKey           = flag.String("key", "loadtest/10k", "Test key path")
	duration          = flag.Duration("duration", 30*time.Second, "Test duration")
	rampUpRate        = flag.Int("ramp", 500, "Connections per second during ramp-up")
)

func main() {
	flag.Parse()

	log.Printf("Starting load test: %d connections to %s", *targetConnections, *serverURL)

	// Create test counter first
	createCounter()

	// Metrics
	var (
		activeConnections  int32
		successfulConnects int32
		failedConnects     int32
		messagesReceived   int64
		connectionErrors   int32
		readErrors         int32
		totalLatency       int64
		latencyCount       int32
	)

	ctx, cancel := context.WithTimeout(context.Background(), *duration+10*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	connections := make(chan *http.Response, *targetConnections)

	// Start metrics reporter
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				active := atomic.LoadInt32(&activeConnections)
				successful := atomic.LoadInt32(&successfulConnects)
				failed := atomic.LoadInt32(&failedConnects)
				messages := atomic.LoadInt64(&messagesReceived)

				avgLatency := int64(0)
				if count := atomic.LoadInt32(&latencyCount); count > 0 {
					avgLatency = atomic.LoadInt64(&totalLatency) / int64(count)
				}

				log.Printf("Connections: active=%d, successful=%d, failed=%d | Messages=%d | AvgLatency=%dms",
					active, successful, failed, messages, avgLatency)

				// Memory stats
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				log.Printf("Memory: Alloc=%dMB, Sys=%dMB, NumGC=%d, Goroutines=%d",
					m.Alloc/1024/1024, m.Sys/1024/1024, m.NumGC, runtime.NumGoroutine())
			}
		}
	}()

	// Connection establishment phase
	log.Println("Starting connection ramp-up...")
	connectionBatch := *targetConnections / (*rampUpRate / 100)
	if connectionBatch < 1 {
		connectionBatch = 1
	}

	for i := 0; i < *targetConnections; i += connectionBatch {
		select {
		case <-ctx.Done():
			break
		default:
		}

		batchSize := connectionBatch
		if i+batchSize > *targetConnections {
			batchSize = *targetConnections - i
		}

		for j := 0; j < batchSize; j++ {
			wg.Add(1)
			go func(connID int) {
				defer wg.Done()

				client := &http.Client{
					// No timeout for SSE connections - they're long-lived streams
					Transport: &http.Transport{
						MaxIdleConns:        100,
						MaxIdleConnsPerHost: 10,
					},
				}

				req, err := http.NewRequest("GET", fmt.Sprintf("%s/stream/%s", *serverURL, *testKey), nil)
				if err != nil {
					atomic.AddInt32(&failedConnects, 1)
					return
				}

				req.Header.Set("Accept", "text/event-stream")

				start := time.Now()
				resp, err := client.Do(req)
				if err != nil {
					atomic.AddInt32(&failedConnects, 1)
					atomic.AddInt32(&connectionErrors, 1)
					log.Printf("Connection error: %v", err)
					return
				}

				latency := time.Since(start).Milliseconds()
				atomic.AddInt64(&totalLatency, latency)
				atomic.AddInt32(&latencyCount, 1)

				atomic.AddInt32(&successfulConnects, 1)
				atomic.AddInt32(&activeConnections, 1)

				// Store connection for cleanup later (check if context is done first)
				select {
				case <-ctx.Done():
					resp.Body.Close()
					return
				case connections <- resp:
					// Successfully stored connection
				}

				// Read events in a separate goroutine - keep connection alive
				go func(response *http.Response) {
					defer func() {
						atomic.AddInt32(&activeConnections, -1)
					}()

					scanner := bufio.NewScanner(response.Body)
					for scanner.Scan() {
						select {
						case <-ctx.Done():
							return
						default:
						}

						line := scanner.Text()
						if strings.HasPrefix(line, "data: ") {
							atomic.AddInt64(&messagesReceived, 1)
						}
					}

					if err := scanner.Err(); err != nil {
						if err.Error() != "EOF" && !strings.Contains(err.Error(), "closed") {
							atomic.AddInt32(&readErrors, 1)
						}
					}
				}(resp)
			}(i + j)
		}

		time.Sleep(100 * time.Millisecond) // Control ramp-up rate
	}

	// Wait for connections to establish
	log.Println("Waiting for connections to establish...")
	time.Sleep(5 * time.Second)

	// Sustain phase - send updates
	log.Println("Starting sustained load phase...")
	sustainCtx, sustainCancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer sustainCancel()

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		updateCount := 0
		for {
			select {
			case <-sustainCtx.Done():
				return
			case <-ticker.C:
				// Hit the counter to generate SSE events
				client := &http.Client{Timeout: 5 * time.Second}
				resp, err := client.Get(fmt.Sprintf("%s/hit/%s", *serverURL, *testKey))
				if err == nil {
					resp.Body.Close()
					updateCount++
					log.Printf("Sent update #%d", updateCount)
				} else {
					log.Printf("Failed to send update: %v", err)
				}
			}
		}
	}()

	// Wait for sustain phase
	<-sustainCtx.Done()

	// Graceful shutdown
	log.Println("Starting graceful shutdown...")
	cancel()

	// Close all connections
	close(connections)
	for resp := range connections {
		resp.Body.Close()
	}

	// Wait for all goroutines
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Println("All connections closed gracefully")
	case <-time.After(10 * time.Second):
		log.Println("Timeout waiting for connections to close")
	}

	// Final metrics
	fmt.Println("\n=== Final Metrics ===")
	fmt.Printf("Target Connections: %d\n", *targetConnections)
	fmt.Printf("Successful Connections: %d\n", atomic.LoadInt32(&successfulConnects))
	fmt.Printf("Failed Connections: %d\n", atomic.LoadInt32(&failedConnects))
	fmt.Printf("Connection Errors: %d\n", atomic.LoadInt32(&connectionErrors))
	fmt.Printf("Read Errors: %d\n", atomic.LoadInt32(&readErrors))
	fmt.Printf("Total Messages Received: %d\n", atomic.LoadInt64(&messagesReceived))

	avgLatency := int64(0)
	if count := atomic.LoadInt32(&latencyCount); count > 0 {
		avgLatency = atomic.LoadInt64(&totalLatency) / int64(count)
	}
	fmt.Printf("Average Connection Latency: %dms\n", avgLatency)

	// Memory stats
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Final Memory - Alloc: %dMB, Sys: %dMB, NumGC: %d\n",
		m.Alloc/1024/1024, m.Sys/1024/1024, m.NumGC)

	// Get SSE stats from server
	resp, err := http.Get(fmt.Sprintf("%s/stats", *serverURL))
	if err == nil {
		defer resp.Body.Close()
		// You could parse and display SSE stats here
		fmt.Println("\nCheck /stats endpoint for SSE server statistics")
	}

	successRate := float64(atomic.LoadInt32(&successfulConnects)) / float64(*targetConnections)
	fmt.Printf("\nSuccess Rate: %.2f%%\n", successRate*100)

	if successRate < 0.95 {
		log.Fatal("Failed to achieve 95% connection success rate")
	}
}

func createCounter() {
	client := &http.Client{Timeout: 5 * time.Second}

	// Try to create the counter
	resp, err := client.Post(fmt.Sprintf("%s/create/%s", *serverURL, *testKey), "", nil)
	if err != nil {
		log.Fatalf("Failed to create counter: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		log.Println("Counter already exists, continuing...")
	} else if resp.StatusCode != http.StatusCreated {
		log.Fatalf("Failed to create counter: status %d", resp.StatusCode)
	} else {
		log.Println("Counter created successfully")
	}
}
