package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goccy/go-json"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestStatsView(t *testing.T) {
	// Set the test environment

	// Populate mock data
	Client.Set(context.Background(), "stats:Total", 1000, 0)
	Client.Set(context.Background(), "stats:create", 100, 0)
	Client.Set(context.Background(), "stats:get", 200, 0)
	Client.Set(context.Background(), "stats:hit", 300, 0)

	// Initialize Gin
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/stats", StatsView)

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
