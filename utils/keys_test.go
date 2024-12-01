package utils

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/stretchr/testify/assert"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		input    string
		expected error
	}{
		{"", fmt.Errorf("length must be between 3 and 64 characters inclusive")},
		{"a", fmt.Errorf("length must be between 3 and 64 characters inclusive")},
		{"ab", fmt.Errorf("length must be between 3 and 64 characters inclusive")},
		{"abc", nil},
		{"abcdefghijklmnopqrstuvwxyz", nil},
		{"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", nil},
		{"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890", nil},
		{"abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ12345678901234567890", fmt.Errorf("length must be between 3 and 64 characters inclusive")},
		{"abc$", fmt.Errorf("must match the pattern ^[A-Za-z0-9_\\-.]{3,64}$")},
		{"abc-123$", fmt.Errorf("must match the pattern ^[A-Za-z0-9_\\-.]{3,64}$")},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := validate(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestTruncateString(t *testing.T) {
	assert.Equal(t, MinLength, 3)  // tests assume MIN_LENGTH of 3
	assert.Equal(t, MaxLength, 64) // tests assume MAX_LENGTH of 64
	testCases := []struct {
		input    string
		expected string
	}{
		{"ab", ".ab"},          // Less than 3 characters
		{"abcdefg", "abcdefg"}, // Within allowed length
		{"abcdefghijklmnopqrstuvwxyz1234567890", // Longer than 64 characters
			"abcdefghijklmnopqrstuvwxyz1234567890"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := truncateString(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetNamespaceKey(t *testing.T) {
	gin.SetMode(gin.TestMode)
	testCases := []struct {
		namespace         string
		key               string
		expectedNamespace string
		expectedKey       string
		expectedCode      int
	}{
		{"jasoncameron.dev", "test", "jasoncameron.dev", "test", http.StatusOK},
		{"jasoncameron.dev", "test/test", "", "", http.StatusNotFound},
		{"jasoncameron.dev", "", "default", "jasoncameron.dev", http.StatusOK},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s-%s", tc.namespace, tc.key), func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Params = []gin.Param{gin.Param{Key: "namespace", Value: tc.namespace}, gin.Param{Key: "key", Value: tc.key}}
			c.Request, _ = http.NewRequest(http.MethodGet, "/", nil)
			namespace, key := GetNamespaceKey(c)
			assert.Equal(t, tc.expectedNamespace, namespace)
			assert.Equal(t, tc.expectedKey, key)
			assert.Equal(t, tc.expectedCode, w.Code)
		})
	}
}
