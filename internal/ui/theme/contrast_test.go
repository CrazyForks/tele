package theme_test

import (
	"image/color"
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

// foregroundTokens are the tokens drawn straight onto the terminal background,
// with nothing painted behind them. Only these can be checked against the
// background: a token that sits on a surface the theme paints (text_on_toast,
// text_on_selected) has to be judged against that surface instead, and a border
// is a line rather than text and legitimately sits lower.
var foregroundTokens = []string{
	"TextDim", "TextMuted", "TextFaint",
	"Accent",
	"StatusError", "StatusWarning", "StatusInfo", "StatusOnline",
	"TickSent", "TickOutbox", "TickRead", "TickFailed",
	"NameIncoming", "NameEditing",
	"Indicator", "UnreadSeparator", "WaveformPlayed", "ReactionChosen",
	"UnreadReaction", "UnreadMention",
	"MarkupLink", "MarkupRef",
	"ComposerCounterDim", "ComposerGlyphIdle", "ComposerGlyphReady",
}

// minContrast is the floor a foreground token must clear against the background
// it is drawn on. It is deliberately the UI-component bar rather than the
// body-text one: some of these are meant to be quiet (text_faint is a
// placeholder), and holding them to 4.5 would force them louder than they should
// be. What it does catch is a token that is the wrong way round for its
// background — the case that put pale greens and yellows into the light theme.
const minContrast = 3.0

// A theme has to be readable against the background it is for. tele-light
// inherited its first values from the dark palette wholesale, which left pale
// greens and yellows on white at barely 1.5:1 — invisible, and invisible in a way
// no test noticed. This is that test.
func TestBuiltins_ForegroundTokensReadOnTheirBackground(t *testing.T) {
	for _, tc := range []struct {
		theme      theme.Theme
		background color.Color
		name       string
	}{
		{theme.TeleDark, black{}, "tele-dark on a dark terminal"},
		{theme.TeleLight, white{}, "tele-light on a light terminal"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := reflect.ValueOf(tc.theme)
			for _, field := range foregroundTokens {
				f := v.FieldByName(field)
				if !f.IsValid() {
					t.Fatalf("no token named %s; update foregroundTokens", field)
				}
				got := contrast(f.Interface().(color.Color), tc.background)
				assert.GreaterOrEqual(t, got, minContrast,
					"%s contrasts %.2f:1 with the background it is drawn on", theme.TokenKey(field), got)
			}
		})
	}
}

// The sender palette is drawn on the terminal background too, and one
// unreadable entry means every Nth person in a group has an invisible name.
func TestBuiltins_SenderPaletteReadsOnItsBackground(t *testing.T) {
	for _, tc := range []struct {
		theme      theme.Theme
		background color.Color
		name       string
	}{
		{theme.TeleDark, black{}, "tele-dark"},
		{theme.TeleLight, white{}, "tele-light"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for i, c := range tc.theme.SenderPalette {
				got := contrast(c, tc.background)
				assert.GreaterOrEqual(t, got, minContrast,
					"sender_palette[%d] contrasts %.2f:1", i, got)
			}
		})
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
	for _, th := range []theme.Theme{theme.TeleDark, theme.TeleLight} {
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
					theme.TokenKey(pair.fg), got, theme.TokenKey(pair.bg))
			}
		})
	}
}

type black struct{}

func (black) RGBA() (r, g, b, a uint32) { return 0, 0, 0, 0xffff }

type white struct{}

func (white) RGBA() (r, g, b, a uint32) { return 0xffff, 0xffff, 0xffff, 0xffff }

// contrast is the WCAG contrast ratio between two colors.
func contrast(fg, bg color.Color) float64 {
	l1, l2 := luminance(fg), luminance(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// luminance is the WCAG relative luminance of a color.
func luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	lin := func(v uint32) float64 {
		s := float64(v>>8) / 255
		if s <= 0.04045 {
			return s / 12.92
		}
		return math.Pow((s+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}
