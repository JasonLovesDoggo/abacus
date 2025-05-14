package utils

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"pkg.jsn.cam/abacus/lib"

	"pkg.jsn.cam/abacus/lib/badge"
)

type generatorCacheKey struct {
	fontPath string
	fontSize float64
}

var generatorCache sync.Map

// getOrCreateGenerator retrieves a generator from cache or creates and caches it
func getOrCreateGenerator(fontPath string, fontSize float64) (*badge.Generator, error) {
	key := generatorCacheKey{fontPath: fontPath, fontSize: fontSize}

	// Try to load from cache
	if gen, ok := generatorCache.Load(key); ok {
		return gen.(*badge.Generator), nil
	}

	// Not in cache, create a new one
	log.Printf("Cache miss: Creating new badge generator for font: %s, size: %f", fontPath, fontSize)
	generator, err := badge.NewGenerator(fontPath, fontSize)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize badge generator: %w", err)
	}

	// Store in cache (LoadOrStore handles race conditions)
	actualGen, _ := generatorCache.LoadOrStore(key, generator)

	return actualGen.(*badge.Generator), nil
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
		return nil, err // Return validation error directly
	}

	// Validate and parse text color
	textColor, err = badge.ValidateColor(textColor)
	if err != nil {
		return nil, err // Return validation error directly
	}

	// Parse font size
	fontSize, err := strconv.ParseFloat(fontSizeStr, 64)
	if err != nil || fontSize <= 3 {
		fontSize = 11 // Fallback to default if invalid
	}

	// Get font path and font family
	filePath, fontFamily, err := lib.GetFontFilePath(font)
	if err != nil {
		log.Printf("Error: Failed to get font file path: %v", err)
		// Return a more specific error if font path fails
		return nil, fmt.Errorf("font error: failed to find font '%s': %w", font, err)
	}

	// Use the cached generator
	generator, err := getOrCreateGenerator(filePath, fontSize)
	if err != nil {
		log.Printf("Error: Failed to get/create badge generator: %v", err)
		// Ensure errors from generator creation/retrieval are returned
		return nil, fmt.Errorf("badge generator error: %w", err)
	}

	// Adjust padding based on font size to maintain proportions
	paddingH := fontSize * 0.75
	paddingV := fontSize * 0.45
	generator.SetPadding(paddingH, paddingV) // Apply padding settings

	// Convert count to string for badge
	countString := strconv.FormatInt(count, 10)

	// Create Params struct
	badgeParams := badge.Params{
		LeftText:   text, // Use 'text' parsed from query
		RightText:  countString,
		Color:      bgColor,
		TextColor:  textColor,
		FontSize:   fontSize,   // Pass the specific fontSize
		FontFamily: fontFamily, // Pass the specific fontFamily
	}

	if badge.IsSimpleStyle(style) {
		badgeParams.LeftText = "" // Empty LeftText for simple styles
		switch style {
		case "plastic-simple":
			return generator.GeneratePlasticSimple(countString, bgColor, textColor)
		case "flat-square-simple":
			return generator.GenerateFlatSquareSimple(countString, bgColor, textColor)
		case "flat-simple":
			return generator.GenerateFlatSimple(countString, bgColor, textColor)
		default:
			// Fallback for unknown simple styles
			log.Printf("Unknown simple badge style '%s', defaulting to flat-simple", style)
			return generator.GenerateFlatSimple(countString, bgColor, textColor)
		}
	}

	// Regular badge styles
	switch style {
	case "plastic":
		return generator.GeneratePlastic(text, countString, bgColor, textColor)
	case "flat-square":
		return generator.GenerateFlatSquare(text, countString, bgColor, textColor)
	case "flat":
		return generator.GenerateFlat(text, countString, bgColor, textColor)
	default:
		// Fallback for unknown regular styles
		log.Printf("Unknown badge style '%s', defaulting to flat", style)
		return generator.GenerateFlat(text, countString, bgColor, textColor)
	}
}
