package theme_test

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

// painted is a theme that claims the canvas, with the body text the canvas
// depends on. It is what a user writing a full palette produces.
func painted(t *testing.T) theme.Theme {
	t.Helper()
	bg, err := theme.ParseColor("#1e1e2e")
	require.NoError(t, err)
	fg, err := theme.ParseColor("#cdd6f4")
	require.NoError(t, err)

	th := theme.TeleDark
	th.Name = "painted"
	th.Background, th.Text = bg, fg
	return th
}

// install makes th current for the duration of the test.
func install(t *testing.T, th theme.Theme) {
	t.Helper()
	t.Cleanup(func() { theme.SetSlots(theme.Slots{Dark: theme.TeleDark, Light: theme.TeleLight}) })
	theme.SetSlots(theme.Slots{Dark: th, Light: th})
	theme.Apply(true)
}

// The built-ins leave the canvas unset, and that has to cost nothing: every
// style and every pad must render exactly what the raw string did before the
// token existed. This is what says no one's screen changes until they ask.
func TestCanvas_UnsetPaintsNothing(t *testing.T) {
	theme.SetSlots(theme.Slots{Dark: theme.TeleDark, Light: theme.TeleLight})

	for _, dark := range []bool{true, false} {
		theme.Apply(dark)
		require.True(t, theme.IsNone(theme.T().Background), "a built-in must not claim the canvas")

		assert.Equal(t, "hello", theme.NewStyle().Render("hello"))
		for _, n := range []int{0, 1, 7} {
			assert.Equal(t, strings.Repeat(" ", n), theme.Pad(n),
				"an unset canvas must pad with plain spaces")
		}
		assert.Equal(t, "", theme.Pad(-3), "a negative width pads nothing")
	}
}

// Once a theme claims the canvas, both the constructor and the pad carry it —
// the pad especially, because bare spaces are the largest area of colour on
// screen and belong to no style at all.
func TestCanvas_SetPaintsStylesAndPadding(t *testing.T) {
	install(t, painted(t))

	styled := theme.NewStyle().Render("hello")
	assert.NotEqual(t, "hello", styled)
	assert.Contains(t, styled, "hello")

	pad := theme.Pad(4)
	assert.Contains(t, pad, "    ", "the pad still occupies four cells")
	assert.NotEqual(t, "    ", pad, "the pad must carry the canvas")
	assert.Equal(t, 4, lipglossWidth(pad), "the canvas must not change the pad's width")
}

// PadTo is the shape the call sites have: pad a line of known width out to a
// target. A line already at or past the target adds nothing, or the padding
// would push the layout out.
func TestCanvas_PadTo(t *testing.T) {
	install(t, painted(t))

	assert.Equal(t, 3, lipglossWidth(theme.PadTo(7, 10)))
	assert.Equal(t, "", theme.PadTo(10, 10))
	assert.Equal(t, "", theme.PadTo(12, 10))
}

// A theme that claims the canvas without claiming the body text is refused: the
// terminal's foreground was chosen against the terminal's background, not
// against the canvas the theme just painted over it.
func TestDependency_CanvasWithoutTextIsCleared(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"halfway.yml": "background: \"#1e1e2e\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("halfway", theme.TeleDark)

	assert.True(t, theme.IsNone(got.Theme.Background),
		"a canvas with no body text must not survive resolution")
	require.Len(t, l.Warnings(), 1)
	assert.Contains(t, l.Warnings()[0], "background")
	assert.Contains(t, l.Warnings()[0], "text",
		"the warning has to name the token that has to be added")
	assert.Equal(t, "none", got.Origins["background"].Raw,
		"a dump must show what is in effect, not what was refused")
}

// The dependency is a property of the resolved theme, not of the file that
// declares it. Splitting the two across a chain is exactly what base: is for,
// and a per-file check would forbid it.
func TestDependency_IsSatisfiedAcrossTheChain(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"mine.yml":  "text: \"#cdd6f4\"\n",
		"child.yml": "base: mine\nbackground: \"#1e1e2e\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("child", theme.TeleDark)

	assert.False(t, theme.IsNone(got.Theme.Background),
		"the canvas is legitimate: the chain supplies the text it depends on")
	// Named rather than asserted empty: this canvas is legitimate but not
	// beyond reproach, and the legibility audit has its own opinion about the
	// tokens inherited under it. That opinion is not this test's business.
	for _, w := range l.Warnings() {
		assert.NotContains(t, w, "background is set but text is not",
			"the dependency is met across the chain and must not be reported")
	}
}

// And the reverse: a descendant that puts the required token back to none breaks
// the pair, which is the case a per-file check would wave through because the
// layer that set the canvas did set both.
func TestDependency_IsBrokenByADescendant(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"full.yml":  "background: \"#1e1e2e\"\ntext: \"#cdd6f4\"\n",
		"plain.yml": "base: full\ntext: none\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("plain", theme.TeleDark)

	assert.True(t, theme.IsNone(got.Theme.Background))
	require.Len(t, l.Warnings(), 1)
	assert.Contains(t, l.Warnings()[0], "plain")
}

// Text without a canvas is legitimate and is what ships today. The dependency
// runs one way and must not be read as a pairing.
func TestDependency_TextWithoutCanvasIsFine(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"texty.yml": "text: \"#cdd6f4\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("texty", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	assert.False(t, theme.IsNone(got.Theme.Text))
	assert.True(t, theme.IsNone(got.Theme.Background))
}

// Apply enforces the dependency too. The loader is where a user hears about a
// broken file, but a Theme also reaches the slots as a Go value, and nothing
// that renders may see one whose dependency is unmet.
func TestDependency_ApplyEnforcesItForAThemeBuiltInCode(t *testing.T) {
	th := painted(t)
	th.Text = mustNone(t)
	install(t, th)

	assert.True(t, theme.IsNone(theme.T().Background),
		"Apply must clear a canvas that reached the slots without its text")
	assert.Equal(t, "hello", theme.NewStyle().Render("hello"),
		"and the styles built from it must go back to painting nothing")
}

func mustNone(t *testing.T) color.Color {
	t.Helper()
	none, err := theme.ParseColor("none")
	require.NoError(t, err)
	return none
}

// lipglossWidth is the display width of a string with its escapes discounted —
// what the layout math measures. A canvas that changed it would break every
// width calculation in the app.
func lipglossWidth(s string) int { return lipgloss.Width(s) }
