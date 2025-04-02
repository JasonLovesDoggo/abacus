package badge

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ValidateColor checks if the color is valid and returns a properly formatted hex color
func ValidateColor(color string) (string, error) {
	if len(color) == 0 {
		return "", errors.New("color cannot be empty (hint: If you are prefixing with a # symbol, remove it and try again)")
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
		return "", fmt.Errorf("'%s' is not a valid hex color (should be like 'fff' or 'ff5500')", strings.TrimPrefix(color, "#"))
	}

	return color, nil
}

// IsSimpleStyle checks if the badge style is a simple one (no left text)
func IsSimpleStyle(style string) bool {
	return strings.HasSuffix(strings.ToLower(style), "-simple")
}
