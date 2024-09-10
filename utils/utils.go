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

// truncateString truncates the string to a maximum of 64 characters. If the string is less than 3 characters, it will be padded with dots.
func truncateString(s string) string {

	strLen := len(s)
	if strLen < MinLength {
		return strings.Repeat(".", MinLength-strLen) + s
	}
	if strLen > MaxLength {
		return s[:MaxLength]
	}
	return s

}

func convertReserved(c *gin.Context, input string) string {
	input = strings.Trim(input, "/")
	if input == ":HOST:" {
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Origin header is required if :HOST: is used"})
			return ""
		}
		return truncateString(origin)
	} else if input == ":PATH:" {
		path := c.Request.Header.Get("Referer")
		if path == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Referer header is required if :PATH: is used"})
			return ""
		}

		return truncateString(path)

	}

	return input
}

func CreateRawAdminKey(c *gin.Context) string {
	namespace, key := GetNamespaceKey(c)
	namespace = convertReserved(c, namespace)
	key = convertReserved(c, key)
	if key == "" || namespace == "" {
		return ""
	}
	return "A:" + namespace + ":" + key

}
func CreateKey(c *gin.Context, namespace, key string, skipValidation bool) string {
	namespace = convertReserved(c, namespace)
	if namespace == "" {
		return ""
	}
	key = convertReserved(c, key)
	if key == "" {
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
	// check if env was loaded via some other format
	if os.Getenv("API_ANALYTICS_ENABLED") != "" {
		return
	}
	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		log.Println("No .env file found")
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
