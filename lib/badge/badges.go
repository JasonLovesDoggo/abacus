package badge

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// Generator is the main badge generator structure
type Generator struct {
	font        *truetype.Font
	fontSize    float64
	dpi         float64
	templates   map[string]*template.Template
	paddingH    float64
	paddingV    float64
	lineSpacing float64
	fontFamily  string
}

// Params BadgeParams contains the parameters for badge generation
type Params struct {
	LeftText   string
	RightText  string
	Color      string
	TextColor  string
	FontSize   float64
	FontFamily string
}

// Text dimensions calculation result
type textDimensions struct {
	Width   float64
	Height  float64
	Ascent  float64
	Descent float64
}

var // Map of known font files to their proper CSS family names
fontFamilyMap = map[string]string{
	"Verdana.ttf":             "Verdana,DejaVu Sans,sans-serif",
	"Verdana_Bold.ttf":        "Verdana Bold,DejaVu Sans,sans-serif",
	"Verdana_Bold_Italic.ttf": "Verdana Bold Italic,DejaVu Sans,sans-serif",
	"Arial.ttf":               "Arial,Helvetica,sans-serif",
	"Arial_Bold.ttf":          "Arial Bold,Helvetica,sans-serif",
	"Arial_Italic.ttf":        "Arial Italic,Helvetica,sans-serif",
	"Arial_Bold_Italic.ttf":   "Arial Bold Italic,Helvetica,sans-serif",
	"Courier_New.ttf":         "Courier New,Courier,monospace",
	"JetbrainsMono.ttf":       "JetBrains Mono,Courier New,monospace",
}

// NewGenerator creates a new badge generator
func NewGenerator(fontPath string, fontSize float64) (*Generator, error) {
	if fontSize <= 0 {
		fontSize = 11
	}

	// Load font file
	fontData, err := os.ReadFile(fontPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read font file: %w", err)
	}

	// Parse font
	ttfFont, err := freetype.ParseFont(fontData)
	if err != nil {
		return nil, fmt.Errorf("unable to parse font: %w", err)
	}

	// Determine font family from filename
	fontFamily := determineFontFamily(fontPath)

	// Create template functions
	funcMap := template.FuncMap{
		"div": func(a float64, b int) float64 {
			return a / float64(b)
		},
		"divInt": func(a int, b int) int {
			return a / b
		},
		"calcRadius": func(height float64) float64 {
			// Make radius proportional to height, with min/max limits
			radius := height * 0.15 // 15% of height
			if radius < 2 {
				return 2 // Minimum radius
			}
			if radius > 5 {
				return 5 // Maximum radius
			}
			return radius
		},
	}

	// Load templates
	templates := make(map[string]*template.Template)

	// Create template for each style
	styles := map[string]string{
		"flat":               templateFlatStyle,
		"flat-square":        templateFlatSquareStyle,
		"plastic":            templatePlasticStyle,
		"flat-simple":        templateFlatSimpleStyle,
		"flat-square-simple": templateFlatSquareSimpleStyle,
		"plastic-simple":     templatePlasticSimpleStyle,
	}

	for name, tmplString := range styles {
		tmpl, err := template.New(name).Funcs(funcMap).Parse(tmplString)
		if err != nil {
			return nil, fmt.Errorf("unable to parse template for style %s: %w", name, err)
		}
		templates[name] = tmpl
	}

	return &Generator{
		font:        ttfFont,
		fontSize:    fontSize,
		dpi:         72,
		templates:   templates,
		paddingH:    8,          // Horizontal padding
		paddingV:    5,          // Vertical padding
		lineSpacing: 1.2,        // Line spacing multiplier
		fontFamily:  fontFamily, // Set font family
	}, nil
}

// determineFontFamily gets the font family name from the font file path
func determineFontFamily(fontPath string) string {
	// Extract font name from path
	filename := filepath.Base(fontPath)

	if family, exists := fontFamilyMap[filename]; exists {
		return family
	}

	// Default fallback: strip extension and use as family with fallbacks
	baseName := strings.TrimSuffix(filename, filepath.Ext(filename))
	return baseName + ",DejaVu Sans,Verdana,Geneva,sans-serif"
}

// calculateTextDimensions calculates the width and height of the given text
func (g *Generator) calculateTextDimensions(text string) textDimensions {
	opts := truetype.Options{
		Size:    g.fontSize,
		DPI:     g.dpi,
		Hinting: font.HintingFull,
	}

	face := truetype.NewFace(g.font, &opts)

	var width fixed.Int26_6
	for _, r := range text {
		adv, ok := face.GlyphAdvance(r)
		if !ok {
			adv, _ = face.GlyphAdvance('?')
		}
		width += adv
	}

	// Convert from fixed point to float64
	widthFloat := float64(width) / 64.0

	// Calculate height based on font metrics
	metrics := face.Metrics()
	ascentFloat := float64(metrics.Ascent) / 64.0
	descentFloat := float64(metrics.Descent) / 64.0
	heightFloat := ascentFloat + descentFloat

	return textDimensions{
		Width:   widthFloat,
		Height:  heightFloat,
		Ascent:  ascentFloat,
		Descent: descentFloat,
	}
}

// Generate creates a badge with the given parameters and style
func (g *Generator) Generate(params Params, style string) ([]byte, error) {
	// Validate and format the background color
	formattedColor, err := ValidateColor(params.Color)
	if err != nil {
		return nil, fmt.Errorf("invalid background color: %w", err)
	}

	// Validate and format the text color (default to white if not specified)
	formattedTextColor := "#fff"
	if params.TextColor != "" {
		formattedTextColor, err = ValidateColor(params.TextColor)
		if err != nil {
			return nil, fmt.Errorf("invalid text color: %w", err)
		}
	}

	leftDims := g.calculateTextDimensions(params.LeftText)
	rightDims := g.calculateTextDimensions(params.RightText)

	// Calculate badge dimensions with padding
	leftWidth := leftDims.Width + (g.paddingH * 2)
	rightWidth := rightDims.Width + (g.paddingH * 2)
	height := max(leftDims.Height, rightDims.Height)*g.lineSpacing + (g.paddingV * 2)

	// Calculate text vertical positions for proper centering
	textY := g.paddingV + leftDims.Ascent + ((height - g.paddingV*2 - leftDims.Height) / 2)

	// Use provided font family or fallback to generator's default
	fontFamily := params.FontFamily
	if fontFamily == "" {
		fontFamily = g.fontFamily
	}

	// Prepare template data
	data := map[string]interface{}{
		"LeftText":   params.LeftText,
		"RightText":  params.RightText,
		"Color":      formattedColor,
		"TextColor":  formattedTextColor,
		"LeftWidth":  leftWidth,
		"RightWidth": rightWidth,
		"TotalWidth": leftWidth + rightWidth,
		"Height":     height,
		"TextY":      textY,
		"LeftTextX":  leftWidth / 2,
		"RightTextX": leftWidth + (rightWidth / 2),
		"FontSize":   g.fontSize,
		"FontFamily": fontFamily,
		"CenterX":    rightWidth / 2,
	}

	// Select the appropriate template
	tmpl, exists := g.templates[style]
	if !exists {
		return nil, fmt.Errorf("unknown badge style: %s", style)
	}

	// Render the badge
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("failed to render badge: %w", err)
	}

	return buf.Bytes(), nil
}

// GenerateFlat generates a flat style badge
func (g *Generator) GenerateFlat(leftText, rightText, color string, textColor string) ([]byte, error) {
	return g.Generate(Params{
		LeftText:   leftText,
		RightText:  rightText,
		Color:      color,
		TextColor:  textColor,
		FontSize:   g.fontSize,
		FontFamily: g.fontFamily,
	}, "flat")
}

// GenerateFlatSquare generates a flat-square style badge
func (g *Generator) GenerateFlatSquare(leftText, rightText, color string, textColor string) ([]byte, error) {
	return g.Generate(Params{
		LeftText:   leftText,
		RightText:  rightText,
		Color:      color,
		TextColor:  textColor,
		FontSize:   g.fontSize,
		FontFamily: g.fontFamily,
	}, "flat-square")
}

// GeneratePlastic generates a plastic style badge
func (g *Generator) GeneratePlastic(leftText, rightText, color string, textColor string) ([]byte, error) {
	return g.Generate(Params{
		LeftText:   leftText,
		RightText:  rightText,
		Color:      color,
		TextColor:  textColor,
		FontSize:   g.fontSize,
		FontFamily: g.fontFamily,
	}, "plastic")
}

// Simple badge variants for single-text badges

// GenerateFlatSimple generates a simple flat badge with single text
func (g *Generator) GenerateFlatSimple(text, color string, textColor string) ([]byte, error) {
	return g.Generate(Params{
		LeftText:   "",
		RightText:  text,
		Color:      color,
		TextColor:  textColor,
		FontSize:   g.fontSize,
		FontFamily: g.fontFamily,
	}, "flat-simple")
}

// GenerateFlatSquareSimple generates a simple flat-square badge with single text
func (g *Generator) GenerateFlatSquareSimple(text, color string, textColor string) ([]byte, error) {
	return g.Generate(Params{
		LeftText:   "",
		RightText:  text,
		Color:      color,
		TextColor:  textColor,
		FontSize:   g.fontSize,
		FontFamily: g.fontFamily,
	}, "flat-square-simple")
}

// GeneratePlasticSimple generates a simple plastic badge with single text
func (g *Generator) GeneratePlasticSimple(text, color string, textColor string) ([]byte, error) {
	return g.Generate(Params{
		LeftText:   "",
		RightText:  text,
		Color:      color,
		TextColor:  textColor,
		FontSize:   g.fontSize,
		FontFamily: g.fontFamily,
	}, "plastic-simple")
}

// SetFontSize allows changing the font size after generator creation
func (g *Generator) SetFontSize(size float64) error {
	if size <= 0 {
		return errors.New("font size must be greater than 0")
	}
	g.fontSize = size
	return nil
}

// SetPadding allows customizing the badge padding
func (g *Generator) SetPadding(horizontal, vertical float64) {
	if horizontal > 0 {
		g.paddingH = horizontal
	}
	if vertical > 0 {
		g.paddingV = vertical
	}
}

// SetFontFamily allows changing the font family
func (g *Generator) SetFontFamily(fontFamily string) {
	g.fontFamily = fontFamily
}
