package badge

import (
	"errors"
	"regexp"
	"strings"
)

// ValidateColor checks if the color is valid and returns a properly formatted hex color
func ValidateColor(color string) (string, error) {
	if len(color) == 0 {
		return "", errors.New("color cannot be empty")
	}

	// Trim any whitespace
	color = strings.TrimSpace(color)

	// Add # prefix if missing
	if !strings.HasPrefix(color, "#") {
		color = "#" + color
	}

	// Check if it's a valid hex code format (either #RGB or #RRGGBB)
	hexRegex := regexp.MustCompile(`^#([0-9A-Fa-f]{3}|[0-9A-Fa-f]{6})$`)
	if !hexRegex.MatchString(color) {
		return "", errors.New("invalid hex color format: must be a valid hex color like 'fff' or 'ff5500'")
	}

	return color, nil
}

// BadgeStyle represents different badge styles
type BadgeStyle string

// Available badge styles
const (
	StyleFlat             BadgeStyle = "flat"
	StyleFlatSquare       BadgeStyle = "flat-square"
	StylePlastic          BadgeStyle = "plastic"
	StyleFlatSimple       BadgeStyle = "flat-simple"
	StyleFlatSquareSimple BadgeStyle = "flat-square-simple"
	StylePlasticSimple    BadgeStyle = "plastic-simple"
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
