package main

import (
	"bufio"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"pkg.jsn.cam/abacus/utils"

	"github.com/goccy/go-json"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	utils.LoadEnv()

	if os.Getenv("TESTING") != "true" {
		fmt.Println("Running tests in non-testing mode. Exiting... (hint: set TESTING=true in .env)")
		os.Exit(0)
	}
	if !errors.Is(Client.Get(context.Background(), "K:stats:Total").Err(), redis.Nil) {
		fmt.Println("Running tests on a non-empty database. Exiting...")
		os.Exit(0)
	}
}

// mockResponseWriter wraps httptest.ResponseRecorder to implement http.CloseNotifier.
type mockResponseWriter struct {
	*httptest.ResponseRecorder
	closeNotifyCh chan bool
}

// CloseNotify satisfies the CloseNotifier interface.
func (m *mockResponseWriter) CloseNotify() <-chan bool {
	return m.closeNotifyCh
}

// Hijack satisfies the http.Hijacker interface.
func (m *mockResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	return nil, nil, http.ErrNotSupported
}

// Flush satisfies the http.Flusher interface.
func (m *mockResponseWriter) Flush() {
	m.ResponseRecorder.Flush()
}

// NewMockResponseWriter creates a new mockResponseWriter.
func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		ResponseRecorder: httptest.NewRecorder(),
		closeNotifyCh:    make(chan bool, 1),
	}
}

func setupTestRouter() *gin.Engine {
	// Use the same setup as in main.go for consistency
	gin.SetMode(gin.TestMode)
	return CreateRouter()
}

func TestCreateView(t *testing.T) {
	r := setupTestRouter()

	t.Run("Create key with default initializer", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/create/test/sample_key", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, "sample_key", response["key"])
		assert.Equal(t, "test", response["namespace"])
		assert.Equal(t, float64(0), response["value"])
		assert.Contains(t, response, "admin_key")
	})

	t.Run("Create key with custom initializer", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/create/test/custom_key?initializer=42", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, "custom_key", response["key"])
		assert.Equal(t, "test", response["namespace"])
		assert.Equal(t, float64(42), response["value"])
	})

	t.Run("Create duplicate key", func(t *testing.T) {
		w1 := httptest.NewRecorder()
		req1, _ := http.NewRequest("POST", "/create/test/duplicate_key", nil)
		r.ServeHTTP(w1, req1)

		w2 := httptest.NewRecorder()
		req2, _ := http.NewRequest("POST", "/create/test/duplicate_key", nil)
		r.ServeHTTP(w2, req2)

		assert.Equal(t, http.StatusCreated, w1.Code)
		assert.Equal(t, http.StatusConflict, w2.Code)
	})
}

func TestHitView(t *testing.T) {
	r := setupTestRouter()

	// First create a key
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/create/test/hit_key", nil)
	r.ServeHTTP(w, req)

	t.Run("Increment existing key", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/hit/test/hit_key", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, float64(1), response["value"])
	})

	t.Run("Multiple hits", func(t *testing.T) {
		for i := 0; i < 5; i++ {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/hit/test/hit_key", nil)
			r.ServeHTTP(w, req)
		}

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/hit/test/hit_key", nil)
		r.ServeHTTP(w, req)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, float64(7), response["value"])
	})
}

func TestHitShield(t *testing.T) {
	r := setupTestRouter()

	// First create a shield key
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/create/test/shield_key", nil)
	r.ServeHTTP(w, req)

	t.Run("Increment shield key", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/hit/test/shield_key/shield", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		responseBytes := w.Body.Bytes()

		assert.NotEmpty(t, responseBytes, "Response body should not be empty")

		responseStr := string(responseBytes)

		// Validate that it's valid XML
		var svgDoc interface{}
		err := xml.Unmarshal(responseBytes, &svgDoc)
		assert.NoError(t, err, "Response should be valid XML")

		re := regexp.MustCompile(`<text.*?>(\d+)</text>`)
		matches := re.FindAllStringSubmatch(responseStr, -1)

		assert.NotEmpty(t, matches, "SVG should contain at least one <text> element with a number")

		counterText := matches[len(matches)-1][1] // Get the last captured number
		counterValue, err := strconv.ParseFloat(counterText, 64)
		assert.NoError(t, err, "Counter value should be a valid float")

		assert.Equal(t, float64(1), counterValue, "Counter value should match expected")
	})
}

func TestGetView(t *testing.T) {
	r := setupTestRouter()

	t.Run("Get non-existent key", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/get/test/nonexistent_key", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Get existing key", func(t *testing.T) {
		// Create a key first
		createW := httptest.NewRecorder()
		createReq, _ := http.NewRequest("POST", "/create/test/get_test_key?initializer=100", nil)
		r.ServeHTTP(createW, createReq)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/get/test/get_test_key", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, float64(100), response["value"])
	})
}

func TestGetShield(t *testing.T) {
	r := setupTestRouter()

	t.Run("Get non-existent shield key", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/get/test/nonexistent_shield_key/shield", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("Get existing shield key", func(t *testing.T) {
		// Create a shield key first
		createW := httptest.NewRecorder()
		createReq, _ := http.NewRequest("POST", "/create/test/get_shield_key?initializer=50", nil)
		r.ServeHTTP(createW, createReq)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/get/test/get_shield_key/shield", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		responseBytes := w.Body.Bytes()

		assert.NotEmpty(t, responseBytes, "Response body should not be empty")

		responseStr := string(responseBytes)

		// Validate that it's valid XML
		var svgDoc interface{}
		err := xml.Unmarshal(responseBytes, &svgDoc)
		assert.NoError(t, err, "Response should be valid XML")

		re := regexp.MustCompile(`<text.*?>(\d+)</text>`)
		matches := re.FindAllStringSubmatch(responseStr, -1)

		assert.NotEmpty(t, matches, "SVG should contain at least one <text> element with a number")

		counterText := matches[len(matches)-1][1] // Get the last captured number
		counterValue, err := strconv.ParseFloat(counterText, 64)
		assert.NoError(t, err, "Counter value should be a valid float")

		assert.Equal(t, float64(50), counterValue, "Counter value should match expected")
	})
}

func TestCreateRandomView(t *testing.T) {
	r := setupTestRouter()

	t.Run("Create random key", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/create/", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Contains(t, response, "key")
		assert.Contains(t, response, "namespace")
		assert.Contains(t, response, "admin_key")
	})
}

func TestInfoView(t *testing.T) {
	r := setupTestRouter()

	t.Run("Get info for existing key", func(t *testing.T) {
		// Create a key first
		createW := httptest.NewRecorder()
		createReq, _ := http.NewRequest("POST", "/create/test/info_key?initializer=150", nil)
		r.ServeHTTP(createW, createReq)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/info/test/info_key", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, float64(150), response["value"])
		assert.True(t, response["exists"].(bool))
		assert.NotZero(t, response["expires_in"])
	})

	t.Run("Get info for non-existent key", func(t *testing.T) {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/info/test/nonexistent_info_key", nil)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		assert.NoError(t, err)

		assert.Equal(t, float64(-1), response["value"])
		assert.False(t, response["exists"].(bool))
	})
}

func TestStatsView(t *testing.T) {

	// Initialize Gin
	r := setupTestRouter()
	// Populate mock data
	Client.Set(context.Background(), "stats:Total", 1000, 0)
	Client.Set(context.Background(), "stats:create", 100, 0)
	Client.Set(context.Background(), "stats:get", 200, 0)
	Client.Set(context.Background(), "stats:hit", 300, 0)

	// Test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/stats", nil)
	r.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	var responseData map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &responseData)
	assert.NoError(t, err)
	commands := responseData["commands"].(map[string]interface{})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, float64(100), commands["create"])
	assert.Equal(t, float64(200), commands["get"])
	assert.Equal(t, float64(300), commands["hit"])
	assert.Equal(t, float64(1000), commands["total"]) // Note: JSON numbers are unmarshaled as float64
	assert.Equal(t, Version, responseData["version"])
}
func TestDeleteView(t *testing.T) {
	r := setupTestRouter()

	t.Run("Delete existing key with admin token", func(t *testing.T) {
		// Create a key first to get the admin token
		createW := httptest.NewRecorder()
		createReq, _ := http.NewRequest("POST", "/create/test/delete_key", nil)
		r.ServeHTTP(createW, createReq)

		// Extract admin token from the response
		var createResponse map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &createResponse)
		adminToken := createResponse["admin_key"].(string)

		// Now delete the key with the admin token
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/delete/test/delete_key", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		exists := Client.Exists(context.Background(), "abacus:test:delete_key").Val()
		assert.Equal(t, int64(0), exists)
	})

}

func TestSetView(t *testing.T) {
	r := setupTestRouter()

	t.Run("Set existing key with admin token", func(t *testing.T) {
		// Create a key first to get the admin token
		createW := httptest.NewRecorder()
		createReq, _ := http.NewRequest("POST", "/create/test/set_key", nil)
		r.ServeHTTP(createW, createReq)

		// Extract admin token
		var createResponse map[string]interface{}
		err := json.Unmarshal(createW.Body.Bytes(), &createResponse)
		if err != nil {
			fmt.Printf("failed to unmarshal create response: %v", err.Error())
		}
		adminToken := createResponse["admin_key"].(string)
		assert.NotEmpty(t, adminToken)
		// Now set the key to a new value
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/set/test/set_key?value=42", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)

		val, _ := Client.Get(context.Background(), "K:test:set_key").Int()
		assert.Equal(t, 42, val)
	})

}

func TestResetView(t *testing.T) {
	r := setupTestRouter()

	t.Run("Reset existing key with admin token", func(t *testing.T) {
		// Create a key first to get the admin token
		createW := httptest.NewRecorder()
		createReq, _ := http.NewRequest("POST", "/create/test/reset_key?initializer=100", nil)
		r.ServeHTTP(createW, createReq)

		// Extract admin token
		var createResponse map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &createResponse)
		adminToken := createResponse["admin_key"].(string)

		// Now reset the key
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/reset/test/reset_key", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken) // Set the admin token
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check if the value is reset in Redis
		val, _ := Client.Get(context.Background(), "abacus:test:reset_key").Int()
		assert.Equal(t, 0, val)
	})

	t.Run("Reset non-existent key with admin token", func(t *testing.T) {
		// You still need an admin token even for a non-existent key
		createW := httptest.NewRecorder()
		createReq, _ := http.NewRequest("POST", "/create/test/some_other_key", nil)
		r.ServeHTTP(createW, createReq)

		var createResponse map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &createResponse)
		adminToken := createResponse["admin_key"].(string)

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/reset/test/nonexistent_reset_key", nil)
		req.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "This entry is genuine and does not have an admin key")
	})

	t.Run("Reset without admin token", func(t *testing.T) {
		// Test without Authorization header
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/reset/test/reset_key", nil)
		r.ServeHTTP(w, req)

		// Should get an unauthorized error (or whichever error your Auth middleware returns)
		assert.NotEqual(t, http.StatusOK, w.Code) // Assert it's not 200 OK
	})
}

func TestUpdateByView(t *testing.T) {
	r := setupTestRouter()

	t.Run("Update existing key with admin token", func(t *testing.T) {
		// Create a key first to get the admin token
		createW := httptest.NewRecorder()
		createReq, _ := http.NewRequest("POST", "/create/test/update_key?initializer=10", nil)
		r.ServeHTTP(createW, createReq)

		// Extract admin token
		var createResponse map[string]interface{}
		json.Unmarshal(createW.Body.Bytes(), &createResponse)
		adminToken := createResponse["admin_key"].(string)

		// Now update the key
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/update/test/update_key?value=15", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer "+adminToken) // Set the admin token
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		// Check if the value is updated in Redis
		val, _ := Client.Get(context.Background(), "K:test:update_key").Int()
		assert.Equal(t, 25, val)
	})

	t.Run("Update non-existent key with admin token", func(t *testing.T) {
		// You still need an admin token even for a non-existent key
		createW := httptest.NewRecorder()
		createReq, _ := http.NewRequest("POST", "/create/test/wow-some_other_key", nil)
		r.ServeHTTP(createW, createReq)

		var createResponse map[string]interface{}
		err := json.Unmarshal(createW.Body.Bytes(), &createResponse)
		if err != nil {
			fmt.Printf("failed to unmarshal create response: %v", err.Error())
			return
		}
		adminToken := createResponse["admin_key"].(string)

		w := httptest.NewRecorder()

		req, _ := http.NewRequest("POST", "/update/test/nonexistent_update_key?value=5", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "Bearer "+adminToken)
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Contains(t, w.Body.String(), "Key does not exist")
	})

	t.Run("Update without admin token", func(t *testing.T) {
		// Test without Authorization header
		w := httptest.NewRecorder()

		beforeUpdateVal, _ := Client.Get(context.Background(), "K:test:update_key").Int()

		req, _ := http.NewRequest("POST", "/update/test/update_key?value=5", nil)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.ServeHTTP(w, req)

		// Should get an unauthorized error
		assert.NotEqual(t, http.StatusOK, w.Code)

		// Key should not be updated
		val, _ := Client.Get(context.Background(), "K:test:update_key").Int()
		assert.Equal(t, beforeUpdateVal, val)

	})
}

func TestStreamValueView(t *testing.T) {
	r := setupTestRouter()

	t.Run("Stream updates for existing key", func(t *testing.T) {
		// Create a key first
		createW := httptest.NewRecorder()
		createReq, _ := http.NewRequest("POST", "/create/test/stream_key", nil)
		r.ServeHTTP(createW, createReq)

		// Start streaming using a custom response writer
		w := newMockResponseWriter()
		req, _ := http.NewRequest("GET", "/stream/test/stream_key", nil)

		// Create a new context with cancellation
		requestCtx, cancelFunc := context.WithCancel(req.Context())
		req = req.WithContext(requestCtx)

		// Channel to signal test completion
		done := make(chan struct{})
		go func() {
			defer close(done)
			r.ServeHTTP(w, req)
		}()

		// Wait for initial response
		waitForContains := func(w *mockResponseWriter, expected string, timeout time.Duration) bool {
			deadline := time.Now().Add(timeout)
			for time.Now().Before(deadline) {
				if strings.Contains(w.Body.String(), expected) {
					return true
				}
				time.Sleep(10 * time.Millisecond)
			}
			return false
		}

		if !waitForContains(w, "data:", 500*time.Millisecond) {
			t.Fatal("Initial SSE connection not established in time")
		}

		// Hit the key to generate updates
		hitReq, _ := http.NewRequest("GET", "/hit/test/stream_key", nil)

		// Trigger updates
		hitW := httptest.NewRecorder()
		r.ServeHTTP(hitW, hitReq)

		// Check for value 1 with timeout
		if !waitForContains(w, "data: {\"value\":1}\n\n", 500*time.Millisecond) {
			t.Fatal("Did not receive first update in time")
		}

		r.ServeHTTP(hitW, hitReq) // Hit it again

		// Check for value 2 with timeout
		if !waitForContains(w, "data: {\"value\":2}\n\n", 500*time.Millisecond) {
			t.Fatal("Did not receive second update in time")
		}

		// Signal the stream to stop
		cancelFunc()

		// Wait for goroutine to finish with timeout
		select {
		case <-done:
			// Test completed successfully
		case <-time.After(1 * time.Second):
			t.Fatal("Test timed out waiting for stream to close")
		}
	})
}
