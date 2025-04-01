package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/essentialkaos/go-badge"
	"github.com/gin-gonic/gin"
)

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
	bgColor := getHexFromColor(c.DefaultQuery("bgcolor", "blue"))
	text := c.DefaultQuery("text", "counter")
	style := strings.ToLower(c.DefaultQuery("style", "flat"))
	fontSize := c.DefaultQuery("fontsize", "11")
	font := strings.ToLower(c.DefaultQuery("font", "verdana"))

	fontSizeInt, err := strconv.Atoi(fontSize)
	if err != nil || fontSizeInt <= 3 { // font sizes 2 or lower can result in panics due to go's ttf library
		fontSizeInt = 11 // Fallback to default if invalid
	}

	filePath, err := getFontFilePath(font)
	if err != nil {
		log.Printf("Error: Failed to get font file path: %v", err)
		return nil, err
	}
	generator, err := badge.NewGenerator(filePath, fontSizeInt)
	if err != nil {
		log.Printf("Error: Failed to initialize badge generator.")
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
