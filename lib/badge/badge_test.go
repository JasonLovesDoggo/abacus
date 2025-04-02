package badge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBadgeGeneration(t *testing.T) {
	// Path to a test font
	wd, _ := os.Getwd()
	fontPath := filepath.Join(wd, "testdata", "Verdana.ttf")

	// Create directory if it doesn't exist
	if _, err := os.Stat(filepath.Join(wd, "testdata")); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Join(wd, "testdata"), 0755); err != nil {
			t.Fatalf("Failed to create testdata directory: %v", err)
		}
	}

	// Use the default font if the test font doesn't exist
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		t.Skip("Test font not found, skipping test")
	}

	// Test different font sizes
	fontSizes := []float64{4, 11, 20, 30}

	for _, fontSize := range fontSizes {
		t.Run("TestFontSize"+string(rune(fontSize)), func(t *testing.T) {
			generator, err := NewGenerator(fontPath, fontSize)
			if err != nil {
				t.Fatalf("Failed to create generator with font size %f: %v", fontSize, err)
			}

			// Test regular badge
			svg, err := generator.GenerateFlat("test", "123", "#007ec6", "#fff")
			if err != nil {
				t.Errorf("Failed to generate flat badge with font size %f: %v", fontSize, err)
			}
			if len(svg) == 0 {
				t.Errorf("Failed to generate flat badge with font size %f", fontSize)
			}

			// Verify SVG structure
			svgString := string(svg)
			if !strings.Contains(svgString, "<svg") || !strings.Contains(svgString, "</svg>") {
				t.Errorf("Generated SVG is malformed with font size %f", fontSize)
			}

			// Verify font family is included
			if !strings.Contains(svgString, "font-family=") {
				t.Errorf("Font family not found in SVG with font size %f", fontSize)
			}

			// Test other badge styles
			styles := []string{"flat-square", "plastic", "flat-simple", "flat-square-simple", "plastic-simple"}
			for _, style := range styles {
				params := Params{
					LeftText:   "test",
					RightText:  "123",
					Color:      "#007ec6",
					FontSize:   fontSize,
					FontFamily: "Test Font,sans-serif",
				}

				// If it's a simple style, empty out LeftText
				if IsSimpleStyle(style) {
					params.LeftText = ""
				}

				svg, err := generator.Generate(params, style)
				if err != nil || len(svg) == 0 {
					t.Errorf("Failed to generate %s badge with font size %f: %v", style, fontSize, err)
				}

				// Verify custom font family is used
				svgString := string(svg)
				if !strings.Contains(svgString, "font-family=\"Test Font,sans-serif\"") {
					t.Errorf("Custom font family not applied to %s badge", style)
				}
			}
		})
	}
}

func TestColorValidation(t *testing.T) {
	tests := []struct {
		color    string
		isValid  bool
		expected string
	}{
		{"007ec6", true, "#007ec6"},  // No # prefix, valid
		{"#007ec6", true, "#007ec6"}, // With # prefix, valid
		{"fff", true, "#fff"},        // Short form, no #, valid
		{"#fff", true, "#fff"},       // Short form, with #, valid
		{"#123456", true, "#123456"}, // Regular hex
		{"#f00", true, "#f00"},       // Short form red
		{"red", false, ""},           // Named color, invalid
		{"blue", false, ""},          // Named color, invalid
		{"", false, ""},              // Empty string
		{"#ff", false, ""},           // Too short
		{"#fffffff", false, ""},      // Too long
		{"123zzz", false, ""},        // Invalid characters
	}

	for _, test := range tests {
		formatted, err := ValidateColor(test.color)
		if test.isValid && err != nil {
			t.Errorf("ValidateColor(%s) returned error for valid color: %v", test.color, err)
		}
		if !test.isValid && err == nil {
			t.Errorf("ValidateColor(%s) did not return error for invalid color", test.color)
		}
		if test.isValid && formatted != test.expected {
			t.Errorf("ValidateColor(%s) produced %s, expected %s", test.color, formatted, test.expected)
		}
	}
}

func TestInvalidColorGeneration(t *testing.T) {
	// Path to a test font
	wd, _ := os.Getwd()
	fontPath := filepath.Join(wd, "testdata", "Verdana.ttf")

	// Skip if font doesn't exist
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		t.Skip("Test font not found, skipping test")
	}

	generator, err := NewGenerator(fontPath, 11)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	// Test with invalid color
	params := Params{
		LeftText:  "test",
		RightText: "123",
		Color:     "red", // Not a hex code
		FontSize:  11,
	}

	_, err = generator.Generate(params, "flat")
	if err == nil {
		t.Errorf("Generate should fail with non-hex color but didn't")
	}
}

func TestTemplateRendering(t *testing.T) {
	// Path to a test font
	wd, _ := os.Getwd()
	fontPath := filepath.Join(wd, "testdata", "Verdana.ttf")

	// Skip if font doesn't exist
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		t.Skip("Test font not found, skipping test")
	}

	generator, err := NewGenerator(fontPath, 11)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	// Test all templates
	styles := []string{
		"flat", "flat-square", "plastic",
		"flat-simple", "flat-square-simple", "plastic-simple",
	}

	for _, style := range styles {
		t.Run("Template_"+style, func(t *testing.T) {
			params := Params{
				LeftText:   "test",
				RightText:  "123",
				Color:      "#007ec6",
				FontSize:   11,
				FontFamily: "Test Font,sans-serif",
			}

			// If it's a simple style, empty out LeftText
			if IsSimpleStyle(style) {
				params.LeftText = ""
			}

			svg, err := generator.Generate(params, style)
			if err != nil {
				t.Errorf("Failed to generate %s badge: %v", style, err)
			}

			if len(svg) == 0 {
				t.Errorf("Generated empty SVG for %s style", style)
			}

			// Verify SVG contains style-specific elements
			svgString := string(svg)

			if strings.Contains(style, "flat") && !strings.Contains(style, "square") {
				if !strings.Contains(svgString, "linearGradient id=\"smooth\"") {
					t.Errorf("Flat style should contain smooth gradient but doesn't")
				}
			}

			if strings.Contains(style, "plastic") {
				if !strings.Contains(svgString, "linearGradient id=\"gradient\"") {
					t.Errorf("Plastic style should contain gradient but doesn't")
				}
			}

			if strings.Contains(style, "square") {
				if strings.Contains(svgString, "rx=") {
					t.Errorf("Square style shouldn't have rounded corners but does")
				}
			}
		})
	}
}

// TestFontExistence verifies that all fonts referenced in the utility exist
func TestFontExistence(t *testing.T) {
	// Define the font mappings (copied from utils.getFontFilePath)
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

	// Get current working directory
	execDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Check if the fonts directory exists
	fontsDir := filepath.Join(execDir, "assets", "fonts")
	if _, err := os.Stat(fontsDir); os.IsNotExist(err) {
		t.Skip("Fonts directory doesn't exist, skipping font existence test")
		return
	}

	// Check each font file
	var missingFonts []string
	for fontName, fontFile := range fontMap {
		fontPath := filepath.Join(fontsDir, fontFile)
		if _, err := os.Stat(fontPath); os.IsNotExist(err) {
			missingFonts = append(missingFonts, fontName+" ("+fontFile+")")
		}
	}

	if len(missingFonts) > 0 {
		t.Errorf("The following fonts are missing: %s", strings.Join(missingFonts, ", "))
	}
}

// TestSimpleStyles tests specifically the simple style badges
func TestSimpleStyles(t *testing.T) {
	// Path to a test font
	wd, _ := os.Getwd()
	fontPath := filepath.Join(wd, "testdata", "Verdana.ttf")

	// Skip if font doesn't exist
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		t.Skip("Test font not found, skipping test")
	}

	generator, err := NewGenerator(fontPath, 11)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	// Test various text content to ensure proper handling
	testTexts := []string{
		"123",
		"Count: 123456",
		"A very long text that should still render properly",
		"!@#$%^&*()", // Special characters
	}

	simpleStyles := []string{
		"flat-simple",
		"flat-square-simple",
		"plastic-simple",
	}

	for _, style := range simpleStyles {
		for _, text := range testTexts {
			t.Run(style+"_"+text, func(t *testing.T) {
				params := Params{
					LeftText:   "",
					RightText:  text,
					Color:      "#007ec6",
					FontSize:   11,
					FontFamily: "Test Font,sans-serif",
				}

				svg, err := generator.Generate(params, style)
				if err != nil {
					t.Errorf("Failed to generate %s badge with text '%s': %v", style, text, err)
				}

				if len(svg) == 0 {
					t.Errorf("Generated empty SVG for %s badge with text '%s'", style, text)
				}

				// Verify text content is included
				svgString := string(svg)
				if !strings.Contains(svgString, text) {
					t.Errorf("Text content '%s' missing in %s badge", text, style)
				}

				// Verify basic SVG structure
				if !strings.Contains(svgString, "<svg") || !strings.Contains(svgString, "</svg>") {
					t.Errorf("Generated SVG is malformed for %s badge with text '%s'", style, text)
				}
			})
		}
	}

	// Test the helper functions directly
	t.Run("GenerateSimpleFunctions", func(t *testing.T) {
		// Test each simple badge generation function
		text := "TestText"
		color := "#ff5500"

		flatSimple, err := generator.GenerateFlatSimple(text, color, color)
		if err != nil {
			t.Errorf("GenerateFlatSimple returned error: %v", err)
		}
		if len(flatSimple) == 0 {
			t.Error("GenerateFlatSimple returned empty SVG")
		}

		flatSquareSimple, err := generator.GenerateFlatSquareSimple(text, color, color)
		if err != nil {
			t.Errorf("GenerateFlatSquareSimple returned error: %v", err)
		}
		if len(flatSquareSimple) == 0 {
			t.Error("GenerateFlatSquareSimple returned empty SVG")
		}

		plasticSimple, err := generator.GeneratePlasticSimple(text, color, color)
		if err != nil {
			t.Errorf("GeneratePlasticSimple returned error: %v", err)
		}
		if len(plasticSimple) == 0 {
			t.Error("GeneratePlasticSimple returned empty SVG")
		}
	})
}

func TestTextColorSupport(t *testing.T) {
	// Path to a test font
	wd, _ := os.Getwd()
	fontPath := filepath.Join(wd, "testdata", "Verdana.ttf")

	// Skip if font doesn't exist
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		t.Skip("Test font not found, skipping test")
	}

	generator, err := NewGenerator(fontPath, 11)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	// Test different text colors
	testColors := []struct {
		bgColor   string
		textColor string
	}{
		{"007ec6", "fff"},    // Standard white on blue
		{"ff0000", "000"},    // Black on red
		{"ffffff", "ff0000"}, // Red on white
		{"000000", "ffff00"}, // Yellow on black
	}

	for _, colors := range testColors {
		t.Run("TextColor_"+colors.textColor+"_on_"+colors.bgColor, func(t *testing.T) {
			// Test with regular badge
			svg, err := generator.GenerateFlat("test", "123", colors.bgColor, colors.textColor)
			if err != nil {
				t.Errorf("Failed to generate badge with text color %s: %v", colors.textColor, err)
			}

			if len(svg) == 0 {
				t.Errorf("Generated empty SVG for text color %s", colors.textColor)
			}

			// Verify text color is applied
			svgString := string(svg)
			expectedTextColor := "#" + colors.textColor
			if !strings.HasPrefix(colors.textColor, "#") {
				expectedTextColor = "#" + colors.textColor
			}

			if !strings.Contains(svgString, `fill="`+expectedTextColor+`"`) {
				t.Errorf("Text color %s not found in SVG", expectedTextColor)
			}

			// Also test simple style
			svgSimple, err := generator.GenerateFlatSimple("123", colors.bgColor, colors.textColor)
			if err != nil {
				t.Errorf("Failed to generate simple badge with text color %s: %v", colors.textColor, err)
			}

			if len(svgSimple) == 0 {
				t.Errorf("Generated empty SVG for simple badge with text color %s", colors.textColor)
			}
		})
	}
}

// TestColorErrorMessages tests the improved error messages for invalid colors
func TestColorErrorMessages(t *testing.T) {
	// Path to a test font
	wd, _ := os.Getwd()
	fontPath := filepath.Join(wd, "testdata", "Verdana.ttf")

	// Skip if font doesn't exist
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		t.Skip("Test font not found, skipping test")
	}

	generator, err := NewGenerator(fontPath, 11)
	if err != nil {
		t.Fatalf("Failed to create generator: %v", err)
	}

	// Test with invalid background color
	_, err = generator.GenerateFlat("test", "123", "xyz", "fff")
	if err == nil {
		t.Error("Expected error for invalid background color but got none")
	} else {
		// Check that the error message includes the invalid color
		if !strings.Contains(err.Error(), "'xyz'") {
			t.Errorf("Error message doesn't mention the invalid color: %v", err)
		}
	}

	// Test with invalid text color
	_, err = generator.GenerateFlat("test", "123", "007ec6", "xyz")
	if err == nil {
		t.Error("Expected error for invalid text color but got none")
	} else {
		// Check that the error message includes the invalid color
		if !strings.Contains(err.Error(), "'xyz'") {
			t.Errorf("Error message doesn't mention the invalid color: %v", err)
		}
	}
}
