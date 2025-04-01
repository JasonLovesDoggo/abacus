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
	fontSizes := []float64{11, 20, 30}

	for _, fontSize := range fontSizes {
		t.Run("TestFontSize"+string(rune(fontSize)), func(t *testing.T) {
			generator, err := NewGenerator(fontPath, fontSize)
			if err != nil {
				t.Fatalf("Failed to create generator with font size %f: %v", fontSize, err)
			}

			// Test regular badge
			svg := generator.GenerateFlat("test", "123", "#007ec6")
			if len(svg) == 0 {
				t.Errorf("Failed to generate flat badge with font size %f", fontSize)
			}

			// Verify SVG structure
			svgString := string(svg)
			if !strings.Contains(svgString, "<svg") || !strings.Contains(svgString, "</svg>") {
				t.Errorf("Generated SVG is malformed with font size %f", fontSize)
			}

			// Verify text positioning
			if !strings.Contains(svgString, "font-size=\""+string(rune(fontSize))+"\"") {
				t.Errorf("Font size %f not set correctly in SVG", fontSize)
			}

			// Test other badge styles
			styles := []string{"flat-square", "plastic"}
			for _, style := range styles {
				params := BadgeParams{
					LeftText:  "test",
					RightText: "123",
					Color:     "#007ec6",
					FontSize:  fontSize,
				}

				svg, err := generator.Generate(params, style)
				if err != nil || len(svg) == 0 {
					t.Errorf("Failed to generate %s badge with font size %f: %v", style, fontSize, err)
				}
			}
		})
	}
}

func TestColorValidation(t *testing.T) {
	tests := []struct {
		color   string
		isValid bool
	}{
		{"#007ec6", true},
		{"#fff", true},
		{"#123456", true},
		{"#f00", true},
		{"red", false},
		{"blue", false},
		{"", false},
		{"#ff", false},
		{"#fffffff", false},
	}

	for _, test := range tests {
		err := ValidateColor(test.color)
		if test.isValid && err != nil {
			t.Errorf("ValidateColor(%s) returned error for valid color: %v", test.color, err)
		}
		if !test.isValid && err == nil {
			t.Errorf("ValidateColor(%s) did not return error for invalid color", test.color)
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
	params := BadgeParams{
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
			params := BadgeParams{
				LeftText:  "test",
				RightText: "123",
				Color:     "#007ec6",
				FontSize:  11,
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
				if strings.Contains(svgString, "rx=\"3\"") {
					t.Errorf("Square style shouldn't have rounded corners but does")
				}
			}
		})
	}
}
