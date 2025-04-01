package badge

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
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
}

// BadgeParams contains the parameters for badge generation
type BadgeParams struct {
	LeftText  string
	RightText string
	Color     string
	FontSize  float64
}

// Text dimensions calculation result
type textDimensions struct {
	Width  float64
	Height float64
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
		tmpl, err := template.New(name).Funcs(template.FuncMap{
			"div": func(a, b int) int {
				return a / b
			},
		}).Parse(tmplString)
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
		paddingH:    8,   // Horizontal padding
		paddingV:    5,   // Vertical padding
		lineSpacing: 1.2, // Line spacing multiplier
	}, nil
}

// NewGeneratorFromFS creates a new badge generator from a filesystem
func NewGeneratorFromFS(fsys fs.FS, fontPath string, fontSize float64) (*Generator, error) {
	if fontSize <= 0 {
		fontSize = 11
	}

	// Load font file from FS
	fontData, err := fs.ReadFile(fsys, fontPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read font file from FS: %w", err)
	}

	// Parse font
	ttfFont, err := freetype.ParseFont(fontData)
	if err != nil {
		return nil, fmt.Errorf("unable to parse font: %w", err)
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
		tmpl, err := template.New(name).Funcs(template.FuncMap{
			"div": func(a, b int) int {
				return a / b
			},
		}).Parse(tmplString)
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
		paddingH:    8,   // Horizontal padding
		paddingV:    5,   // Vertical padding
		lineSpacing: 1.2, // Line spacing multiplier
	}, nil
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
	heightFloat := float64(metrics.Ascent+metrics.Descent) / 64.0

	return textDimensions{
		Width:  widthFloat,
		Height: heightFloat,
	}
}

// Generate creates a badge with the given parameters and style
func (g *Generator) Generate(params BadgeParams, style string) ([]byte, error) {
	// Validate color is a hex code
	if err := ValidateColor(params.Color); err != nil {
		return nil, fmt.Errorf("invalid color: %w", err)
	}

	leftDims := g.calculateTextDimensions(params.LeftText)
	rightDims := g.calculateTextDimensions(params.RightText)

	// Calculate badge dimensions with padding
	leftWidth := leftDims.Width + (g.paddingH * 2)
	rightWidth := rightDims.Width + (g.paddingH * 2)
	height := max(leftDims.Height, rightDims.Height)*g.lineSpacing + (g.paddingV * 2)

	// Calculate text vertical positions to center text vertically
	textY := height - g.paddingV - ((height-(g.paddingV*2))/2 - leftDims.Height/2)

	// Prepare template data
	data := map[string]interface{}{
		"LeftText":   params.LeftText,
		"RightText":  params.RightText,
		"Color":      params.Color,
		"LeftWidth":  leftWidth,
		"RightWidth": rightWidth,
		"TotalWidth": leftWidth + rightWidth,
		"Height":     height,
		"TextY":      textY,
		"LeftTextX":  leftWidth / 2,
		"RightTextX": leftWidth + (rightWidth / 2),
		"FontSize":   g.fontSize,
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
func (g *Generator) GenerateFlat(leftText, rightText, color string) []byte {
	badge, err := g.Generate(BadgeParams{
		LeftText:  leftText,
		RightText: rightText,
		Color:     color,
		FontSize:  g.fontSize,
	}, "flat")

	if err != nil {
		// Log the error and return empty SVG
		fmt.Println("Error generating flat badge:", err)
		return []byte{}
	}

	return badge
}

// GenerateFlatSquare generates a flat-square style badge
func (g *Generator) GenerateFlatSquare(leftText, rightText, color string) []byte {
	badge, err := g.Generate(BadgeParams{
		LeftText:  leftText,
		RightText: rightText,
		Color:     color,
		FontSize:  g.fontSize,
	}, "flat-square")

	if err != nil {
		// Log the error and return empty SVG
		fmt.Println("Error generating flat-square badge:", err)
		return []byte{}
	}

	return badge
}

// GeneratePlastic generates a plastic style badge
func (g *Generator) GeneratePlastic(leftText, rightText, color string) []byte {
	badge, err := g.Generate(BadgeParams{
		LeftText:  leftText,
		RightText: rightText,
		Color:     color,
		FontSize:  g.fontSize,
	}, "plastic")

	if err != nil {
		// Log the error and return empty SVG
		fmt.Println("Error generating plastic badge:", err)
		return []byte{}
	}

	return badge
}

// Simple badge variants for single-text badges

// GenerateFlatSimple generates a simple flat badge with single text
func (g *Generator) GenerateFlatSimple(text, color string) []byte {
	badge, err := g.Generate(BadgeParams{
		LeftText:  "",
		RightText: text,
		Color:     color,
		FontSize:  g.fontSize,
	}, "flat-simple")

	if err != nil {
		// Log the error and return empty SVG
		fmt.Println("Error generating flat-simple badge:", err)
		return []byte{}
	}

	return badge
}

// GenerateFlatSquareSimple generates a simple flat-square badge with single text
func (g *Generator) GenerateFlatSquareSimple(text, color string) []byte {
	badge, err := g.Generate(BadgeParams{
		LeftText:  "",
		RightText: text,
		Color:     color,
		FontSize:  g.fontSize,
	}, "flat-square-simple")

	if err != nil {
		// Log the error and return empty SVG
		fmt.Println("Error generating flat-square-simple badge:", err)
		return []byte{}
	}

	return badge
}

// GeneratePlasticSimple generates a simple plastic badge with single text
func (g *Generator) GeneratePlasticSimple(text, color string) []byte {
	badge, err := g.Generate(BadgeParams{
		LeftText:  "",
		RightText: text,
		Color:     color,
		FontSize:  g.fontSize,
	}, "plastic-simple")

	if err != nil {
		// Log the error and return empty SVG
		fmt.Println("Error generating plastic-simple badge:", err)
		return []byte{}
	}

	return badge
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
