package badge

import (
	"errors"
	"strings"
)

// ValidateColor checks if the color is valid
func ValidateColor(color string) error {
	if len(color) == 0 {
		return errors.New("color cannot be empty")
	}

	// Ensure color is a hex code
	if color[0] != '#' {
		return errors.New("color must be a hex code starting with #")
	}

	// Ensure hex code has valid length (either #RGB or #RRGGBB)
	if len(color) != 4 && len(color) != 7 {
		return errors.New("invalid hex code format: must be #RGB or #RRGGBB")
	}

	return nil
}

// BadgeStyle represents different badge styles
type BadgeStyle string

// Available badge styles
const (
	StyleFlat          BadgeStyle = "flat"
	StyleFlatSquare    BadgeStyle = "flat-square"
	StylePlastic       BadgeStyle = "plastic"
	StyleFlatSimple    BadgeStyle = "flat-simple"
	StyleSquareSimple  BadgeStyle = "flat-square-simple"
	StylePlasticSimple BadgeStyle = "plastic-simple"
)

// IsSimpleStyle checks if the badge style is a simple one (no left text)
func IsSimpleStyle(style string) bool {
	return strings.HasSuffix(strings.ToLower(style), "-simple")
}

// ParseBadgeStyle converts a string to a BadgeStyle
func ParseBadgeStyle(style string) BadgeStyle {
	style = strings.ToLower(style)

	switch style {
	case "flat", "flat-simple":
		return StyleFlat
	case "flat-square", "flat-square-simple":
		return StyleFlatSquare
	case "plastic", "plastic-simple":
		return StylePlastic
	default:
		return StyleFlat // Default to flat style
	}
}
