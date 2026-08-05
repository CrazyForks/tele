package theme

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"unicode"

	"charm.land/lipgloss/v2"
)

// normalize folds a name to its canonical form: neither case nor the separators
// "-" and "_" distinguish names. One function serves theme names, token keys and
// ANSI color names, so all three accept the same spellings and a reader never
// has to remember which of them is picky.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r == '-' || r == '_' {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// ansiNames maps the sixteen terminal-palette colors to their indexes. Writing a
// token as one of these is not a shorthand for a hex value: indexes 0-15 resolve
// against the palette the terminal itself is configured with, so a theme written
// in them follows the terminal instead of overriding it.
var ansiNames = map[string]int{
	"black": 0, "red": 1, "green": 2, "yellow": 3,
	"blue": 4, "magenta": 5, "cyan": 6, "white": 7,
	"brightblack": 8, "brightred": 9, "brightgreen": 10, "brightyellow": 11,
	"brightblue": 12, "brightmagenta": 13, "brightcyan": 14, "brightwhite": 15,
}

// ParseColor turns the text of a theme file into a color.
//
// Accepted: "#rrggbb", "#rgb", a palette index 0-255, one of the sixteen ANSI
// color names, and "none".
//
// It exists because lipgloss.Color answers a malformed string with NoColor and
// no error, which would turn a typo into an invisible token. Every string
// reaching lipgloss has been through here first.
func ParseColor(s string) (color.Color, error) {
	raw := strings.TrimSpace(s)
	if raw == "" {
		return nil, fmt.Errorf("empty color")
	}

	// "none" means the attribute is not set at all: a foreground falls back to
	// the terminal's text color and a background is not painted, which is the
	// only way a theme can leave the terminal's own backdrop showing through.
	if normalize(raw) == "none" {
		return lipgloss.NoColor{}, nil
	}

	if strings.HasPrefix(raw, "#") {
		hex, err := expandHex(raw)
		if err != nil {
			return nil, err
		}
		return lipgloss.Color(hex), nil
	}

	if i, err := strconv.Atoi(raw); err == nil {
		if i < 0 || i > 255 {
			return nil, fmt.Errorf("palette index %d out of range 0-255", i)
		}
		return lipgloss.Color(raw), nil
	}

	if i, ok := ansiNames[normalize(raw)]; ok {
		return lipgloss.Color(strconv.Itoa(i)), nil
	}

	return nil, fmt.Errorf("unrecognized color %q: want #rrggbb, #rgb, an index 0-255, an ANSI color name, or none", raw)
}

// expandHex validates a hex color and returns it in the long form, so callers
// downstream only ever handle "#rrggbb".
func expandHex(s string) (string, error) {
	digits := s[1:]
	for _, r := range digits {
		if !isHexDigit(r) {
			return "", fmt.Errorf("invalid hex color %q", s)
		}
	}
	switch len(digits) {
	case 6:
		return s, nil
	case 3:
		return string([]byte{'#', digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]}), nil
	default:
		return "", fmt.Errorf("invalid hex color %q: want 3 or 6 digits", s)
	}
}

func isHexDigit(r rune) bool {
	return (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')
}

// FormatColor renders a color back as theme-file text. It is the inverse of
// ParseColor for everything a theme can hold, except that a color originally
// written as an index or an ANSI name comes back as hex — the loader keeps the
// original text for the cases where that matters.
func FormatColor(c color.Color) string {
	if c == nil {
		return "none"
	}
	if _, ok := c.(lipgloss.NoColor); ok {
		return "none"
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

// Hex builds a color from an RGB triple. It exists for the callers that compute
// a color rather than name one — the highlight fade interpolates between two
// tokens — so that naming a color stays confined to this package.
func Hex(r, g, b uint8) color.Color {
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}
