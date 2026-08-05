// Package theme holds every color the TUI renders with. Nothing under
// internal/ui may name a color directly; a color that is not a token here does
// not exist. The package is a leaf: it imports nothing from internal/ui.
package theme

import (
	"fmt"
	"image/color"
	"sync/atomic"

	"charm.land/lipgloss/v2"
)

// Hex builds a color from an RGB triple. It exists for the callers that compute
// a color rather than name one — the highlight fade interpolates between two
// tokens — so that naming a color stays confined to this package.
func Hex(r, g, b uint8) color.Color {
	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}

// GradientStop is one stop of an interpolated color ramp, at position Pos in
// [0,1].
type GradientStop struct {
	Pos     float64
	R, G, B uint8
}

// Theme is one flavour of a palette: a flat set of semantic color tokens.
// Dark records which terminal background the flavour targets.
//
// Tokens are grouped by prefix. Two tokens may share a value in a given flavour
// (StatusOnline and NameIncoming are the same green in the default palette) and
// still be separate fields: they are separate ideas, and a palette must be free
// to split them.
//
// There is deliberately no primary-text token. Body text is unstyled and
// inherits the terminal foreground; a token would force a color where none is
// forced now.
type Theme struct {
	Name string
	Dark bool

	// Surfaces — filled areas the app paints behind content.
	SurfaceOverlay     color.Color // popup menus, reaction picker, mention popup
	SurfaceHelp        color.Color // help modal panel
	SurfaceToast       color.Color // toast panel
	SurfaceStatusBar   color.Color // status bar
	SurfaceSelected    color.Color // selected row fill; also the mention-popup border and the search prompt
	SurfaceSelfMention color.Color // background of an @mention of the signed-in user
	SurfaceCode        color.Color // inline code and pre blocks in message markup

	// Text.
	TextDim             color.Color // timestamps, quotes, separators
	TextMuted           color.Color // muted chats
	TextFaint           color.Color // "no results", "empty", overlay hint descriptions
	TextSubtle          color.Color // toast overflow line
	TextOnSurface       color.Color // help modal body
	TextStatusBar       color.Color // status bar body
	TextOnSelected      color.Color // text over SurfaceSelected
	TextOnSelectedMuted color.Color // secondary text over SurfaceSelected
	TextOnToast         color.Color // toast body
	TextModeLabel       color.Color // NORMAL/INSERT label
	TextCode            color.Color // inline code and pre blocks in message markup

	// Accents.
	Accent           color.Color // hint keys, mention glyph, toast action, picker numbers
	AccentOnSurface  color.Color // the accent darkened enough for the help panel fill
	AccentInsert     color.Color // status bar key accent in INSERT
	AccentModeNormal color.Color // NORMAL mode label fill
	AccentModeInsert color.Color // INSERT mode label fill

	// Status and message state.
	StatusError     color.Color
	StatusWarning   color.Color
	StatusInfo      color.Color
	StatusOnline    color.Color
	TickSent        color.Color
	TickOutbox      color.Color
	TickRead        color.Color
	TickFailed      color.Color
	NameIncoming    color.Color
	NameEditing     color.Color
	Indicator       color.Color
	UnreadSeparator color.Color
	WaveformPlayed  color.Color
	ReactionChosen  color.Color
	UnreadReaction  color.Color // unread-reaction glyph in the chat list
	UnreadMention   color.Color // unread-mention glyph in the chat list

	// Borders.
	BorderPaneActive      color.Color
	BorderBubbleIn        color.Color
	BorderBubbleOut       color.Color
	BorderOverlay         color.Color // help modal
	BorderComposerFocused color.Color
	BorderComposerFlash   color.Color
	BorderStatusSep       color.Color

	// Message markup entities.
	MarkupLink          color.Color // url, email, phone, bank_card, text_url
	MarkupRef           color.Color // mention, mention_name, hashtag, cashtag, bot_command
	MarkupSelfMentionFg color.Color

	// Transient highlights.
	HighlightAccent     color.Color // jump-to cue, fades toward a base
	HighlightError      color.Color // rolled-back optimistic action
	HighlightBaseChat   color.Color // tone the chat-row highlight fades toward
	HighlightBaseBubble color.Color // tone the bubble highlight fades toward
	OverlayDim          color.Color // content behind a modal

	// Composer.
	ComposerCounterDim color.Color
	ComposerGlyphIdle  color.Color
	ComposerGlyphReady color.Color

	// Palettes.
	SenderPalette [8]color.Color  // per-sender name colors, picked by sender id
	LogoGradient  [5]GradientStop // the logo wave ramp
}

// Family bundles the two flavours a palette ships with.
type Family struct {
	Name        string
	Dark, Light Theme
}

// Resolve returns the flavour matching the terminal background.
func (f Family) Resolve(dark bool) Theme {
	if dark {
		return f.Dark
	}
	return f.Light
}

// snapshot pairs a theme with the styles derived from it. The two are swapped
// as one value so a render can never mix a new theme with stale styles.
type snapshot struct {
	theme  Theme
	styles Styles
}

var current atomic.Pointer[snapshot]

func init() {
	Apply(Default, true)
}

// Apply makes the flavour of f matching the terminal background current. It is
// the only way the current theme changes, and it is one call at the root — a
// theme can never be half-applied.
func Apply(f Family, dark bool) {
	t := f.Resolve(dark)
	current.Store(&snapshot{theme: t, styles: buildStyles(t)})
}

// T returns the current theme. Safe to call on every render: it is a pointer
// load.
func T() *Theme { return &current.Load().theme }

// S returns the styles derived from the current theme.
func S() *Styles { return &current.Load().styles }
