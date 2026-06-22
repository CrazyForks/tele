package components

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	lipcompat "charm.land/lipgloss/v2/compat"
)

// Overlay hint styling: accented keys (bright blue) and dim descriptions and
// separators, matching the status-bar hint format. On opaque overlays (menus,
// reaction picker) the container background is baked into every run so the bottom
// border stays solid across the reset sequences between runs.
var (
	overlayHintDim    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	overlayHintAccent = lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	// OverlayMenuBg is the background of the popup menus and the reaction picker;
	// pass it to OverlayHint so the hint sits on the same fill.
	OverlayMenuBg color.Color = lipcompat.AdaptiveColor{
		Light: lipgloss.Color("252"),
		Dark:  lipgloss.Color("235"),
	}
)

// OverlayHint renders key/desc pairs in the status-bar hint format — the key
// accented in place (a letter highlighted within the word, "enter" shown as a
// trailing ↵ glyph, or an accented prefix otherwise), descriptions dim, joined
// by " · ". Pairs with an empty key and desc are skipped. bg is the container
// background to bake into every run (nil for a transparent overlay). Use it for
// overlay box hints so they match the main status bar.
func OverlayHint(pairs [][2]string, bg color.Color) string {
	base := overlayHintDim
	accent := overlayHintAccent
	if bg != nil {
		base = base.Background(bg)
		accent = accent.Background(bg)
	}
	parts := make([]string, 0, len(pairs))
	for _, p := range pairs {
		key, desc := p[0], p[1]
		if key == "" && desc == "" {
			continue
		}
		text, spans := hintLayout(key, desc)
		parts = append(parts, applyAccent(text, spans, base, accent))
	}
	return strings.Join(parts, base.Render(" · "))
}
