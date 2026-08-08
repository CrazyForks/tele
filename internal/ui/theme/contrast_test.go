// This file is inside the package rather than in theme_test, unlike every other
// test here. It is the one that needs contrast and minContrast directly:
// the surface pairs below are not judged against a canvas, so Audit cannot
// answer for them, and exporting the measure to reach it from outside would
// widen the package's API for a single caller that is a test.
package theme

import (
	"image/color"
	"reflect"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A theme has to be readable against the background it is for. tele-light
// inherited its first values from the dark palette wholesale, which left pale
// greens and yellows on white at barely 1.5:1 — invisible, and invisible in a way
// no test noticed. This is that test.
//
// It goes through Audit rather than measuring for itself: the list of foreground
// tokens now belongs to the package, and a test holding its own copy is how the
// two drift. Giving each built-in the canvas it is meant for is what makes the
// question askable at all, since neither claims one.
func TestBuiltins_ForegroundTokensReadOnTheirBackground(t *testing.T) {
	for _, tc := range []struct {
		theme      Theme
		background color.Color
		name       string
	}{
		{TeleDark, black{}, "tele-dark on a dark terminal"},
		{TeleLight, white{}, "tele-light on a light terminal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, f := range auditOn(tc.theme, tc.background) {
				if f.Palette {
					continue // covered by its own test, which names it
				}
				assert.GreaterOrEqual(t, f.Ratio, minContrast,
					"%s contrasts %.2f:1 with the background it is drawn on", f.Token, f.Ratio)
			}
		})
	}
}

// The sender palette is drawn on the terminal background too, and one
// unreadable entry means every Nth person in a group has an invisible name.
func TestBuiltins_SenderPaletteReadsOnItsBackground(t *testing.T) {
	for _, tc := range []struct {
		theme      Theme
		background color.Color
		name       string
	}{
		{TeleDark, black{}, "tele-dark"},
		{TeleLight, white{}, "tele-light"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, f := range auditOn(tc.theme, tc.background) {
				if !f.Palette {
					continue
				}
				assert.GreaterOrEqual(t, f.Ratio, minContrast, "%s contrasts %.2f:1", f.Token, f.Ratio)
			}
		})
	}
}

// auditOn asks the audit about a built-in against the terminal it is meant for.
// The canvas is the test's invention: neither built-in claims one, and without a
// canvas there is nothing to measure against.
//
// Findings for unset tokens are dropped here. Both built-ins leave text at none
// on purpose — it means the terminal's own foreground, which is what tele has
// always rendered with — and reporting that under a canvas the theme never asked
// for would be the test failing its own premise, not the theme failing.
func auditOn(t Theme, canvas color.Color) []Finding {
	t.Background = canvas
	out := Audit(t)
	return slices.DeleteFunc(out, func(f Finding) bool { return f.Unset })
}

// The foreground list is a claim about the struct, and a token renamed out from
// under it would leave that token silently unaudited.
func TestForegroundTokens_AllNameRealTokens(t *testing.T) {
	v := reflect.ValueOf(TeleDark)
	for _, name := range foregroundTokens {
		assert.True(t, v.FieldByName(name).IsValid(), "no token named %s", name)
	}
}

// surfacePair is a token together with the surface the app paints behind it,
// and the floor that token has to clear against it.
type surfacePair struct {
	fg, bg string
	min    float64
}

// minMark is the floor for a token whose job is to mark rather than to be read
// on its own: the accented hotkey letter inside a menu label. It is lower than
// minContrast on purpose, and the reason is a known blind spot of the measure
// rather than a concession.
//
// Contrast ratio is computed from luminance alone and discards hue entirely. The
// hotkey is identified by differing from the letters beside it, and it differs
// from them mostly in hue. tele-dark makes this plain: its accent sits at 1.29
// against the body text it marks, and reads without any trouble, because one is
// a saturated cyan and the other a neutral grey. Holding the mark to the floor
// meant for body text would force it pale enough to wash out against the panel —
// trading a difference the eye sees for a number that looks better.
const minMark = 2.5

// onSurface are the pairs the terminal background says nothing about: the status
// bar is dark in both themes, so its accents have to be light in both — tuning
// them for a light terminal is what put dark blue on a dark bar.
//
// The audit does not cover these. It measures against the canvas a theme names,
// and a surface is a second background the theme also names; extending it there
// is worth doing and is not done here.
var onSurface = []surfacePair{
	{"AccentStatusBar", "SurfaceStatusBar", minContrast},
	{"AccentInsert", "SurfaceStatusBar", minContrast},
	{"TextStatusBar", "SurfaceStatusBar", minContrast},
	// BorderStatusSep is not here: it draws the "·" between hint groups, and a
	// separator that is as loud as what it separates is worse than a quiet one.
	// The same reason keeps the bubble borders out of foregroundTokens.

	{"AccentOnSurface", "SurfaceOverlay", minMark},
	{"AccentOnSurface", "SurfaceHelp", minMark},
	{"AccentOnSurface", "SurfaceToast", minMark},
	{"TextOnSurface", "SurfaceHelp", minContrast},
	{"TextOnToast", "SurfaceToast", minContrast},
	{"TextSubtle", "SurfaceToast", minContrast},

	{"TextOnSelected", "SurfaceSelected", minContrast},
	{"TextOnSelectedMuted", "SurfaceSelected", minContrast},
	{"TextModeLabel", "AccentModeNormal", minContrast},
	{"TextModeLabel", "AccentModeInsert", minContrast},
	{"MarkupSelfMentionFg", "SurfaceSelfMention", minContrast},
	{"TextCode", "SurfaceCode", minContrast},
}

// A token drawn on a surface the app paints must read against that surface, not
// against the terminal. Both themes paint a dark status bar, so an accent tuned
// for a light terminal disappears on it — which is exactly what happened, and
// what this pins.
func TestBuiltins_TokensReadOnTheSurfacesTheyAreDrawnOn(t *testing.T) {
	for _, th := range []Theme{TeleDark, TeleLight} {
		t.Run(th.Name, func(t *testing.T) {
			v := reflect.ValueOf(th)
			for _, pair := range onSurface {
				fgField, bgField := v.FieldByName(pair.fg), v.FieldByName(pair.bg)
				if !fgField.IsValid() || !bgField.IsValid() {
					t.Fatalf("no token named %s or %s; update onSurface", pair.fg, pair.bg)
				}
				got := contrast(fgField.Interface().(color.Color), bgField.Interface().(color.Color))
				assert.GreaterOrEqual(t, got, pair.min,
					"%s contrasts %.2f:1 with %s, the surface behind it",
					TokenKey(pair.fg), got, TokenKey(pair.bg))
			}
		})
	}
}

type black struct{}

func (black) RGBA() (r, g, b, a uint32) { return 0, 0, 0, 0xffff }

type white struct{}

func (white) RGBA() (r, g, b, a uint32) { return 0xffff, 0xffff, 0xffff, 0xffff }
