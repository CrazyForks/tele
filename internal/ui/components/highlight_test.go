package components_test

import (
	"image/color"
	"testing"

	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/sorokin-vladimir/tele/internal/ui/theme"
	"github.com/stretchr/testify/assert"
)

func rgb8(t *testing.T, c color.Color) (uint8, uint8, uint8) {
	t.Helper()
	r, g, b, _ := c.RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}

func TestFadeAccentColor_FullStepIsAccent(t *testing.T) {
	accent := theme.Hex(255, 175, 0) // 255,175,0
	base := theme.Hex(0, 0, 0)
	r, g, b := rgb8(t, components.FadeAccentColor(accent, base, 5, 5))
	assert.Equal(t, uint8(255), r)
	assert.Equal(t, uint8(175), g)
	assert.Equal(t, uint8(0), b)
}

func TestFadeAccentColor_ZeroStepIsBase(t *testing.T) {
	accent := theme.Hex(255, 175, 0)
	base := theme.Hex(16, 32, 48) // 16,32,48
	r, g, b := rgb8(t, components.FadeAccentColor(accent, base, 0, 5))
	assert.Equal(t, uint8(16), r)
	assert.Equal(t, uint8(32), g)
	assert.Equal(t, uint8(48), b)
}

func TestFadeAccentColor_MidStepInterpolates(t *testing.T) {
	accent := theme.Hex(255, 175, 0) // 255,175,0
	base := theme.Hex(0, 0, 0)
	// step 2/5 -> 40% of the way to accent: r≈102, g≈70, b=0
	r, g, b := rgb8(t, components.FadeAccentColor(accent, base, 2, 5))
	assert.InDelta(t, 102, int(r), 1)
	assert.InDelta(t, 70, int(g), 1)
	assert.Equal(t, uint8(0), b)
}

func TestFadeAccentColor_ClampsOutOfRange(t *testing.T) {
	accent := theme.Hex(255, 175, 0)
	base := theme.Hex(0, 0, 0)
	hi := components.FadeAccentColor(accent, base, 99, 5)
	r, _, _ := rgb8(t, hi)
	assert.Equal(t, uint8(255), r) // clamped to full accent
	lo := components.FadeAccentColor(accent, base, -3, 5)
	r2, g2, b2 := rgb8(t, lo)
	assert.Equal(t, [3]uint8{0, 0, 0}, [3]uint8{r2, g2, b2}) // clamped to base
}

func TestHighlightConstants(t *testing.T) {
	assert.Equal(t, 5, components.HighlightFadeSteps)
}

func TestHighlightAccent_PicksThemeTone(t *testing.T) {
	theme.Apply(theme.Default, true)
	dark := theme.T().HighlightAccent
	theme.Apply(theme.Default, false)
	assert.NotEqual(t, dark, theme.T().HighlightAccent, "the highlight accent must adapt to the background")
}
