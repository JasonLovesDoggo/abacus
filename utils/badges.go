package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jasonlovesdoggo/abacus/lib/badge"
)

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
		"jetbrains-mono":      "JetbrainsMono.ttf",
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
	bgColor := c.DefaultQuery("bgcolor", "#007ec6") // Default to blue
	text := c.DefaultQuery("text", "counter")
	style := strings.ToLower(c.DefaultQuery("style", "flat"))
	fontSizeStr := c.DefaultQuery("fontsize", "11")
	font := strings.ToLower(c.DefaultQuery("font", "verdana"))

	// Validate color is a hex code
	if !strings.HasPrefix(bgColor, "#") {
		return nil, fmt.Errorf("bgcolor must be a hex code starting with #")
	}

	fontSize, err := strconv.ParseFloat(fontSizeStr, 64)
	if err != nil || fontSize <= 0 { // font sizes too small can result in rendering issues
		fontSize = 11 // Fallback to default if invalid
	}

	filePath, err := getFontFilePath(font)
	if err != nil {
		log.Printf("Error: Failed to get font file path: %v", err)
		return nil, err
	}

	// Create badge generator with the specified font and size
	generator, err := badge.NewGenerator(filePath, fontSize)
	if err != nil {
		log.Printf("Error: Failed to initialize badge generator: %v", err)
		return nil, err
	}

	// Adjust padding based on font size to maintain proportions
	paddingH := float64(fontSize) * 0.75
	paddingV := float64(fontSize) * 0.45
	generator.SetPadding(paddingH, paddingV)

	// Convert count to string for badge
	countString := strconv.FormatInt(count, 10)

	// Check if it's a simple badge style (without left text)
	if badge.IsSimpleStyle(style) {
		switch style {
		case "plastic-simple":
			return generator.GeneratePlasticSimple(countString, bgColor), nil
		case "flat-square-simple":
			return generator.GenerateFlatSquareSimple(countString, bgColor), nil
		case "flat-simple":
			return generator.GenerateFlatSimple(countString, bgColor), nil
		default:
			return generator.GenerateFlatSimple(countString, bgColor), nil
		}
	}

	// Regular badge styles with both left and right text
	switch style {
	case "plastic":
		return generator.GeneratePlastic(text, countString, bgColor), nil
	case "flat-square":
		return generator.GenerateFlatSquare(text, countString, bgColor), nil
	case "flat":
		return generator.GenerateFlat(text, countString, bgColor), nil
	default:
		return generator.GenerateFlat(text, countString, bgColor), nil
	}
}
