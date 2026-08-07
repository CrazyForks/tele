package theme_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

// Body is applied where text used to be emitted raw, which is only safe if an
// unset Text renders to exactly what was there before. The built-ins leave it
// unset, so this is what says nobody's screen changes until they ask for it.
func TestBody_UnsetTextRendersRaw(t *testing.T) {
	theme.SetSlots(theme.Slots{Dark: theme.TeleDark, Light: theme.TeleLight})

	for _, dark := range []bool{true, false} {
		theme.Apply(dark)
		require.True(t, theme.IsNone(theme.T().Text), "a built-in must not claim the body text")

		for _, s := range []string{"hello", "", "  padded  ", "юникод", "[12]"} {
			assert.Equal(t, s, theme.S().Body.Render(s),
				"unset Text must leave %q byte for byte", s)
		}
	}
}

// Once a theme sets it, the same call paints.
func TestBody_SetTextPaints(t *testing.T) {
	t.Cleanup(func() { theme.SetSlots(theme.Slots{Dark: theme.TeleDark, Light: theme.TeleLight}) })

	custom := theme.TeleDark
	custom.Name = "painted"
	c, err := theme.ParseColor("#ff0000")
	require.NoError(t, err)
	custom.Text = c
	theme.SetSlots(theme.Slots{Dark: custom, Light: custom})
	theme.Apply(true)

	got := theme.S().Body.Render("hello")
	assert.NotEqual(t, "hello", got)
	assert.Contains(t, got, "hello")
}
