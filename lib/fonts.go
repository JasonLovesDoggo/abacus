package lib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FontInfo contains details about a font
type FontInfo struct {
	FileName   string
	FontFamily string
}

// FontMap is a map of supported fonts with their file names and CSS font-family values
var FontMap = map[string]FontInfo{
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
		FileName:   "JetBrainsMono.ttf",
		FontFamily: "JetBrains Mono,Courier New,monospace",
	},
}

func GetFontFilePath(font string) (string, string, error) {
	execDir, _ := os.Getwd()

	fontInfo, exists := FontMap[font]
	if !exists {
		// Default to Verdana if font name not found
		fontInfo = FontMap["verdana"]
	}

	fontPath := filepath.Join(execDir, "assets", "fonts", fontInfo.FileName)

	// Check if font file exists
	if _, err := os.Stat(fontPath); os.IsNotExist(err) {
		if strings.ToLower(os.Getenv("DEBUG")) == "true" {
			return "", "", fmt.Errorf("font file not found: %s", fontPath)
		}
		return "", "", fmt.Errorf("font file not found: %s", fontPath)
	}

	return fontPath, fontInfo.FontFamily, nil
}
