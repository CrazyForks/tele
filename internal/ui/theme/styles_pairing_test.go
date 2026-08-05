package theme_test

import (
	"reflect"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

// A style that paints an opaque background must also set the foreground.
//
// Leaving the foreground unset means the text keeps the terminal's own, and the
// terminal's foreground was chosen against the terminal's background — not
// against the fill the app just painted over it. The two are then unrelated, and
// on a light terminal whose text is a dark blue the popup menus came out blue on
// grey, with the accented hotkey indistinguishable from the label around it.
//
// The theme cannot know the terminal's foreground, so the only fix is to stop
// depending on it wherever the theme has taken over the background.
func TestStyles_ABackgroundAlwaysComesWithAForeground(t *testing.T) {
	theme.Apply(true)
	s := reflect.ValueOf(*theme.S())
	typ := s.Type()

	for i := range typ.NumField() {
		f := typ.Field(i)
		st, ok := s.Field(i).Interface().(lipgloss.Style)
		if !ok {
			continue
		}
		if isUnset(st.GetBackground()) {
			continue
		}
		assert.False(t, isUnset(st.GetForeground()),
			"%s paints a background but leaves the text to the terminal", f.Name)
	}
}

// isUnset reports whether a style property was never given a color. lipgloss
// answers with NoColor for both "unset" and "explicitly none"; a style that
// paints a background is never the latter.
func isUnset(c any) bool {
	_, ok := c.(lipgloss.NoColor)
	return ok || c == nil
}
