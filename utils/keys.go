package utils

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"github.com/essentialkaos/go-badge"
	"github.com/gin-gonic/gin"
)

var validationRegex = regexp.MustCompile(`^[A-Za-z0-9_\-.]{3,64}$`)

// truncateString truncates the string to a maximum of 64 characters. If the string is less than 3 characters,
// it will be left padded with dots.
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
		// Added validation for Origin header
		if !validateURL(origin) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Origin header format"})
			return ""
		}
		return truncateString(origin)
	} else if input == ":PATH:" {
		path := c.Request.Header.Get("Referer")
		if path == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Referer header is required if :PATH: is used"})
			return ""
		}
		// Added validation for Referer header
		if !validateURL(path) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Referer header format"})
			return ""
		}
		// todo: should we split and only store the actual PATH part? Changing this may break existing clients.
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
	match := validationRegex.MatchString(input)
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

// Add this function to validate URLs
func validateURL(input string) bool {
	// Basic validation for URLs - check for common protocols, no spaces, etc.
	if input == "" {
		return false
	}

	// Check for valid URL protocols
	validProtocols := []string{"http://", "https://"}
	hasValidProtocol := false
	for _, protocol := range validProtocols {
		if strings.HasPrefix(input, protocol) {
			hasValidProtocol = true
			break
		}
	}

	// Check for invalid characters
	containsInvalidChars := strings.ContainsAny(input, " \t\n\r<>\"'\\%{}")

	return hasValidProtocol && !containsInvalidChars
}

func getHexFromColor(color string) string {
	color = strings.ToLower(strings.TrimSpace(color))

	colorMap := map[string]string{
		"blue":        "#007ec6",
		"brightgreen": "#4c1",
		"green":       "#97ca00",
		"grey":        "#555",
		"lightgrey":   "#9f9f9f",
		"orange":      "#fe7d37",
		"red":         "#e05d44",
		"yellow":      "#dfb317",
		"yellowgreen": "#a4a61d",
	}

	if hex, exists := colorMap[color]; exists {
		return hex
	}

	return "#000000" // Default fallback color
}

func getFontFilePath(font string) (string, error) {
	execDir, _ := os.Getwd()
	fontMap := map[string]string{
		"verdana":             "Verdana.ttf",
		"verdana-bold":        "Verdana_Bold.ttf",
		"verdana-bold-italic": "Verdana_Bold_Italic.ttf",
		"arial":               "Arial.ttf",
		"arial-bold":          "Arial_Bold.ttf",
		"arial-italic":        "Arial_Italic.ttf",
		"arial-bold-italic":   "Arial_Bold_Italic.ttf",
		"courier-new":         "Courier_New.ttf",
	}

	fontFile, exists := fontMap[font]
	if !exists {
		fontFile = "Verdana.ttf" // Default
	}

	fontPath := filepath.Join(execDir, "assets", "fonts", fontFile)

	// Check if font file exists
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		return "", fmt.Errorf("font file not found: %s", fontPath)
	}

	return fontPath, nil
}

func GenerateBadge(c *gin.Context, count int64) ([]byte, error) {
	bgColor := getHexFromColor(c.DefaultQuery("bgcolor", "blue"))
	text := c.DefaultQuery("text", "counter")
	style := strings.ToLower(c.DefaultQuery("style", "flat"))
	fontSize := c.DefaultQuery("fontsize", "11")
	font := strings.ToLower(c.DefaultQuery("font", "verdana"))

	fontSizeInt, err := strconv.Atoi(fontSize)
	if err != nil || fontSizeInt <= 0 {
		fontSizeInt = 11 // Fallback to default if invalid
	}

	filepath, err := getFontFilePath(font)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SVG file data."})
		return nil, err
	}
	generator, err := badge.NewGenerator(filepath, fontSizeInt)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to initialize badge generator."})
		return nil, err
	}

	badgeStylesRegular := map[string]func(string, string, string) []byte{
		"flat":        generator.GenerateFlat,
		"flat-square": generator.GenerateFlatSquare,
		"plastic":     generator.GeneratePlastic,
	}

	badgeStylesSimple := map[string]func(string, string) []byte{
		"plastic-simple":     generator.GeneratePlasticSimple,
		"flat-simple":        generator.GenerateFlatSimple,
		"flat-square-simple": generator.GenerateFlatSquareSimple,
	}

	countString := strconv.FormatInt(count, 10)

	if generateFunc, exists := badgeStylesRegular[style]; exists {
		return generateFunc(text, countString, bgColor), nil
	}

	if generateFunc, exists := badgeStylesSimple[style]; exists {
		return generateFunc(countString, bgColor), nil
	}

	// default
	return generator.GenerateFlat(text, countString, bgColor), nil
}
