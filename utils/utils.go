package utils

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"

	"github.com/gin-gonic/gin"
)

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length]
}
func getHostPath(c *gin.Context) (string, string) {
	path := truncateString(strings.ReplaceAll(c.Request.URL.Path, "/", ""), 64)
	// Extract domain and path                          // todo fix path logic
	return "", path
}

func convertReserved(c *gin.Context, input string) (string, bool) {
	input = strings.Trim(input, "/")
	if input == ":HOST:" {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Origin header is required if :HOST: is used"})
			return "", false
		}
		return origin, true
	} else if input == ":PATH:" {
		_, path := getHostPath(c)
		return path, true
	}
	host, path := getHostPath(c)
	fmt.Println(host, path)
	return input, true
}
func CreateKey(c *gin.Context, namespace, key string, skipValidation bool) string {
	namespace, continueOn := convertReserved(c, namespace)
	key, continueOn2 := convertReserved(c, key)
	if !(continueOn && continueOn2) {
		return ""
	}
	if skipValidation == false {
		if err := validate(namespace); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid namespace: " + err.Error()})
			return ""
		}
		if err := validate(key); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid key: " + err.Error()})
			return ""
		}
	}
	return "K:" + namespace + ":" + key
}

// validate checks if the namespace/key meet the validation criteria.
func validate(input string) error {
	if len(input) < 3 || len(input) > 64 {
		return fmt.Errorf("length must be between 3 and 64 characters inclusive")
	}
	match, err := regexp.MatchString(`^[A-Za-z0-9_\-.]{3,64}$`, input)
	if err != nil {
		return err
	}
	if !match {
		return fmt.Errorf("must match the pattern ^[A-Za-z0-9_\\-.]{3,64}$")
	}
	return nil
}

func GetNamespaceKey(c *gin.Context) (string, string) {
	var namespace, key string
	key = strings.Trim(c.Param("key"), "/")
	if strings.Contains(key, "/") {
		c.JSON(http.StatusNotFound, gin.H{"error": "Route not found. Use /create/:namespace/:key or /hit/:key instead."})
		return "", ""
	}
	if !(len(key) > 0) {
		namespace = "default"
		key = c.Param("namespace")
	} else {
		namespace = c.Param("namespace")
	}
	return namespace, key
}

func CreateAdminKey(key string) string {
	// remove the K: prefix
	key = strings.TrimPrefix(key, "K:")
	return "A:" + key
}

func LoadEnv() {
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		fmt.Println("No .env file found")
	} else {
		err := godotenv.Load()
		if err != nil {
			log.Fatalf("Error loading .env file")
		}
	}
}

func GenerateRandomString(length int) (string, error) {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_"
	charsetLen := big.NewInt(int64(len(charset)))

	// Generate random indices and construct the string
	result := make([]byte, length)
	for i := range result {
		randIndex, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		result[i] = charset[randIndex.Int64()]
	}
	return string(result), nil
}
