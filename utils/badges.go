package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jasonlovesdoggo/abacus/lib/badge"

	"github.com/gin-gonic/gin"
)

// FontInfo contains details about a font
type FontInfo struct {
	FileName   string
	FontFamily string
}

// Map of supported fonts with their file names and CSS font-family values
var fontMap = map[string]FontInfo{
	"verdana": {
		FileName:   "Verdana.ttf",
		FontFamily: "Verdana,DejaVu Sans,sans-serif",
	},
	"verdana-bold": {
		FileName:   "Verdana_Bold.ttf",
		FontFamily: "Verdana Bold,DejaVu Sans,sans-serif",
	},
	"verdana-bold-italic": {
		FileName:   "Verdana_Bold_Italic.ttf",
		FontFamily: "Verdana Bold Italic,DejaVu Sans,sans-serif",
	},
	"arial": {
		FileName:   "Arial.ttf",
		FontFamily: "Arial,Helvetica,sans-serif",
	},
	"arial-bold": {
		FileName:   "Arial_Bold.ttf",
		FontFamily: "Arial Bold,Helvetica,sans-serif",
	},
	"arial-italic": {
		FileName:   "Arial_Italic.ttf",
		FontFamily: "Arial Italic,Helvetica,sans-serif",
	},
	"arial-bold-italic": {
		FileName:   "Arial_Bold_Italic.ttf",
		FontFamily: "Arial Bold Italic,Helvetica,sans-serif",
	},
	"courier-new": {
		FileName:   "Courier_New.ttf",
		FontFamily: "Courier New,Courier,monospace",
	},
	"jetbrains-mono": {
		FileName:   "JetbrainsMono.ttf",
		FontFamily: "JetBrains Mono,Courier New,monospace",
	},
}

func getFontFilePath(font string) (string, string, error) {
	execDir, _ := os.Getwd()

	fontInfo, exists := fontMap[font]
	if !exists {
		// Default to Verdana if font name not found
		fontInfo = fontMap["verdana"]
	}

	fontPath := filepath.Join(execDir, "assets", "fonts", fontInfo.FileName)

	// Check if font file exists
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("font file not found: %s", fontPath)
	}

	return fontPath, fontInfo.FontFamily, nil
}

func GenerateBadge(c *gin.Context, count int64) ([]byte, error) {
	bgColor := c.DefaultQuery("bgcolor", "007ec6")
	textColor := c.DefaultQuery("textcolor", "fff")
	text := c.DefaultQuery("text", "counter")
	style := strings.ToLower(c.DefaultQuery("style", "flat"))
	fontSizeStr := c.DefaultQuery("fontsize", "11")
	font := strings.ToLower(c.DefaultQuery("font", "verdana"))

	// Validate and parse background color
	bgColor, err := badge.ValidateColor(bgColor)
	if err != nil {
		return nil, err
	}

	// Validate and parse text color
	textColor, err = badge.ValidateColor(textColor)
	if err != nil {
		return nil, err
	}

	// Parse font size
	fontSize, err := strconv.ParseFloat(fontSizeStr, 64)
	if err != nil || fontSize <= 3 { // font sizes too small can result in rendering issues
		fontSize = 11 // Fallback to default if invalid
	}

	// Get font path and font family
	filePath, fontFamily, err := getFontFilePath(font)
	if err != nil {
		log.Printf("Error: Failed to get font file path: %v", err)
		return nil, fmt.Errorf("font error: %w", err)
	}

	// Create badge generator with the specified font and size
	generator, err := badge.NewGenerator(filePath, fontSize)
	if err != nil {
		log.Printf("Error: Failed to initialize badge generator: %v", err)
		return nil, fmt.Errorf("initialization error: %w", err)
	}

	// Set the font family explicitly
	generator.SetFontFamily(fontFamily)

	// Adjust padding based on font size to maintain proportions
	paddingH := fontSize * 0.75
	paddingV := fontSize * 0.45
	generator.SetPadding(paddingH, paddingV)

	// Convert count to string for badge
	countString := strconv.FormatInt(count, 10)

	// Check if it's a simple badge style (without left text)
	if badge.IsSimpleStyle(style) {
		switch style {
		case "plastic-simple":
			return generator.GeneratePlasticSimple(countString, bgColor, textColor)
		case "flat-square-simple":
			return generator.GenerateFlatSquareSimple(countString, bgColor, textColor)
		case "flat-simple":
			return generator.GenerateFlatSimple(countString, bgColor, textColor)
		default:
			// Default to flat-simple if the style is unknown but contains "-simple"
			return generator.GenerateFlatSimple(countString, bgColor, textColor)
		}
	}

	// Regular badge styles with both left and right text
	switch style {
	case "plastic":
		return generator.GeneratePlastic(text, countString, bgColor, textColor)
	case "flat-square":
		return generator.GenerateFlatSquare(text, countString, bgColor, textColor)
	case "flat":
		return generator.GenerateFlat(text, countString, bgColor, textColor)
	default:
		// Default to flat style for unknown styles
		return generator.GenerateFlat(text, countString, bgColor, textColor)
	}
}
