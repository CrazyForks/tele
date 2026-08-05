// Package theme holds every color the TUI renders with. Nothing under
// internal/ui may name a color directly; a color that is not a token here does
// not exist. The package is a leaf: it imports nothing from internal/ui.
package theme

import (
	"image/color"
	"sync/atomic"
)

// GradientStop is one stop of an interpolated color ramp, at position Pos in
// [0,1].
type GradientStop struct {
	Pos   float64
	Color color.Color
}

// Theme is a named set of semantic color tokens.
//
// Tokens are grouped by prefix. Two tokens may share a value in a given theme
// (StatusOnline and NameIncoming are the same green in tele-dark) and still be
// separate fields: they are separate ideas, and a theme must be free to split
// them.
//
// There is deliberately no primary-text token. Body text is unstyled and
// inherits the terminal foreground; a token would force a color where none is
// forced now.
//
// Field names are the public spelling of the tokens: a theme file's keys are
// matched against them through normalize, so SurfaceOverlay is written
// surface_overlay. Renaming a field renames a key that user files depend on,
// which is why TestTokenKeys pins the whole list to a golden file.
type Theme struct {
	Name string

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
	TextOnSurface       color.Color // body text on any panel the app paints: help modal, popup menus, pickers
	TextStatusBar       color.Color // status bar body
	TextOnSelected      color.Color // text over SurfaceSelected
	TextOnSelectedMuted color.Color // secondary text over SurfaceSelected
	TextOnToast         color.Color // toast body
	TextModeLabel       color.Color // NORMAL/INSERT label
	TextCode            color.Color // inline code and pre blocks in message markup

	// Accents. There are three because the accent is drawn on three different
	// backgrounds, and one value cannot serve them all: a light theme needs a
	// dark accent on the terminal background and a light one on the status bar,
	// which stays dark in both themes.
	Accent           color.Color // on the terminal background: the photo, video and search hints, which have no panel behind them
	AccentOnSurface  color.Color // on a panel the app paints: help modal, popup menus, picker numbers, toast action
	AccentStatusBar  color.Color // on the status bar, in NORMAL
	AccentInsert     color.Color // on the status bar, in INSERT
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

	// Transient highlights. These four are interpolated rather than rendered
	// directly, so none is not a legal value for them: see interpolated.
	HighlightAccent     color.Color // jump-to cue, fades toward a base
	HighlightError      color.Color // rolled-back optimistic action
	HighlightBaseChat   color.Color // tone the chat-row highlight fades toward
	HighlightBaseBubble color.Color // tone the bubble highlight fades toward
	OverlayDim          color.Color // content behind a modal

	// Composer.
	ComposerCounterDim color.Color
	ComposerGlyphIdle  color.Color
	ComposerGlyphReady color.Color

	// Lists. Both hold as many entries as the theme cares to give them, and a
	// theme that sets either one replaces it whole rather than merging into the
	// list it inherited.
	SenderPalette []color.Color  // per-sender name colors, picked by sender id
	LogoGradient  []GradientStop // the logo wave ramp
}

// interpolated names the tokens whose value is arithmetic input rather than
// something handed straight to a style. NoColor reports itself as opaque black,
// so none on one of these would silently mean "fade to black" instead of "leave
// it alone"; the loader rejects it there.
var interpolated = map[string]bool{
	"HighlightAccent":     true,
	"HighlightError":      true,
	"HighlightBaseChat":   true,
	"HighlightBaseBubble": true,
}

// Slots holds the theme used against each terminal background. Both are always
// filled — a config that names one theme puts it in both — so selecting one is
// a choice between two present values and never a fallback.
type Slots struct {
	Dark, Light Theme
}

// pick returns the theme for the current terminal background.
func (s Slots) pick(dark bool) Theme {
	if dark {
		return s.Dark
	}
	return s.Light
}

// snapshot pairs a theme with the styles derived from it. The two are swapped
// as one value so a render can never mix a new theme with stale styles. It also
// remembers which background it was applied for, so a later SetSlots can
// reinstall against the same background.
type snapshot struct {
	theme  Theme
	styles Styles
	dark   bool
}

var (
	slots   atomic.Pointer[Slots]
	current atomic.Pointer[snapshot]
)

func init() {
	SetSlots(Slots{Dark: TeleDark, Light: TeleLight})
}

// SetSlots installs the themes to switch between. It is called once at startup,
// after the config has been read, and again by the reload action. The built-ins
// are installed by init, so the slots are never empty and Apply can never race
// ahead of configuration.
func SetSlots(s Slots) {
	slots.Store(&s)
	Apply(currentIsDark())
}

// Apply makes the theme for the given terminal background current. It is the
// only way the current theme changes, and it is one store at the root — a theme
// can never be half-applied.
func Apply(dark bool) {
	t := slots.Load().pick(dark)
	current.Store(&snapshot{theme: t, styles: buildStyles(t), dark: dark})
}

// currentIsDark reports the background the current theme was applied for,
// defaulting to dark before anything has been applied. It lets SetSlots
// reinstall without knowing what the terminal reported.
func currentIsDark() bool {
	if s := current.Load(); s != nil {
		return s.dark
	}
	return true
}

// T returns the current theme. Safe to call on every render: it is a pointer
// load.
func T() *Theme { return &current.Load().theme }

// S returns the styles derived from the current theme.
func S() *Styles { return &current.Load().styles }
