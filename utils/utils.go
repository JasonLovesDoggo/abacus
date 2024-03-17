package utils

import (
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/joho/godotenv"

	"github.com/gin-gonic/gin"
)

func CreateKey(namespace, key string, skipValidation bool) (string, error) {
	if skipValidation == true {
		fmt.Println("skipValidation")
		if err := validate(namespace); err != nil {
			return "", err
		}
		if err := validate(key); err != nil {
			return "", err
		}
	}

	// Construct the Redis key
	fmt.Println("k:" + namespace + ":" + key)
	key = strings.Trim(key, "/")
	return "K:" + namespace + ":" + key, nil
}

// validate checks if the namespace/key meet the validation criteria.
func validate(input string) error {
	if len(input) <= 3 || len(input) >= 64 {
		return fmt.Errorf("length must be between 3 and 64 characters inclusive")
	}
	match, err := regexp.MatchString(`^[A-Za-z0-9_\-.]{3,64}$`, input)
	fmt.Println(match, err, input)
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
