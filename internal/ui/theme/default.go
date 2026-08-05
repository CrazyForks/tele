package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Default reproduces the appearance the TUI had before colors were tokenised.
// The light flavour is therefore not yet tuned: where the old code had a single
// unconditional color it appears here unchanged in both flavours. Tuning it is
// a follow-up, and it is only tractable now that it is one flavour in one file.
//
// The trailing comment on each value is the xterm index it came from. Indexes
// 16-255 convert losslessly; 0-15 were hand-picked, because the literal xterm
// values for those (#00ff00, #0000ff) are harsher than any real terminal
// palette renders them.
var Default = Family{
	Name:  "default",
	Dark:  defaultDark,
	Light: defaultLight,
}

func hex(s string) color.Color { return lipgloss.Color(s) }

var defaultDark = Theme{
	Name: "default",
	Dark: true,

	SurfaceOverlay:     hex("#262626"), // 235
	SurfaceHelp:        hex("#262626"), // 235
	SurfaceToast:       hex("#3a3a3a"), // 237
	SurfaceStatusBar:   hex("#303030"), // 236
	SurfaceSelected:    hex("#5f5fff"), // 63
	SurfaceSelfMention: hex("#ff87ff"), // 213
	SurfaceCode:        hex("#303030"), // 236

	TextDim:             hex("#8a8a8a"), // 245
	TextMuted:           hex("#808080"), // 244
	TextFaint:           hex("#808080"), // 8
	TextSubtle:          hex("#585858"), // 240
	TextOnSurface:       hex("#bcbcbc"), // 250
	TextStatusBar:       hex("#eeeeee"),
	TextOnSelected:      hex("#ffffff"),
	TextOnSelectedMuted: hex("#e4e4e4"), // 254
	TextOnToast:         hex("#d0d0d0"), // 252
	TextModeLabel:       hex("#ffffff"), // 231
	TextCode:            hex("#d0d0d0"), // 252

	Accent:           hex("#00afff"), // 39
	AccentOnSurface:  hex("#00afff"), // 39
	AccentInsert:     hex("#00d700"), // 40
	AccentModeNormal: hex("#0087ff"), // 33
	AccentModeInsert: hex("#00af5f"), // 35

	StatusError:     hex("#ff5f5f"), // 203
	StatusWarning:   hex("#ffaf00"), // 214
	StatusInfo:      hex("#5fafff"), // 75
	StatusOnline:    hex("#5fd75f"), // 10
	TickSent:        hex("#8a8a8a"), // 245
	TickOutbox:      hex("#8a8a8a"), // 245
	TickRead:        hex("#5f87ff"), // 12
	TickFailed:      hex("#ff5f5f"), // 9
	NameIncoming:    hex("#5fd75f"), // 10
	NameEditing:     hex("#ffd75f"), // 11
	Indicator:       hex("#00afaf"), // 6
	UnreadSeparator: hex("#5f87ff"), // 12
	WaveformPlayed:  hex("#5f87ff"), // 12
	ReactionChosen:  hex("#5fd75f"), // 10
	UnreadReaction:  hex("#ff5faf"), // 205
	UnreadMention:   hex("#00afff"), // 39

	BorderPaneActive:      hex("#5fd75f"), // 10
	BorderBubbleIn:        hex("#444444"), // 238
	BorderBubbleOut:       hex("#005faf"), // 25
	BorderOverlay:         hex("#585858"), // 240
	BorderComposerFocused: hex("#00d700"), // 40
	BorderComposerFlash:   hex("#ff5f5f"), // 203
	BorderStatusSep:       hex("#585858"), // 240

	MarkupLink:          hex("#00d7ff"), // 45
	MarkupRef:           hex("#ff87ff"), // 213
	MarkupSelfMentionFg: hex("#000000"), // 0

	HighlightAccent:     hex("#ffaf00"),
	HighlightError:      hex("#ff5f5f"),
	HighlightBaseChat:   hex("#bcbcbc"), // 250
	HighlightBaseBubble: hex("#444444"), // 238
	OverlayDim:          hex("#585858"), // 240

	ComposerCounterDim: hex("#585858"), // 240
	ComposerGlyphIdle:  hex("#585858"), // 240
	ComposerGlyphReady: hex("#00afff"), // 39

	SenderPalette: [8]color.Color{
		hex("#ff5f5f"), // 9
		hex("#5fd75f"), // 10
		hex("#5f87ff"), // 12
		hex("#ff5fff"), // 13
		hex("#5fffff"), // 14
		hex("#ff8700"), // 208
		hex("#af87ff"), // 141
		hex("#87ff00"), // 118
	},

	LogoGradient: [5]GradientStop{
		{0.00, 60, 90, 160},
		{0.30, 90, 135, 210},
		{0.60, 130, 170, 230},
		{0.85, 175, 210, 248},
		{1.00, 215, 238, 255},
	},
}

var defaultLight = Theme{
	Name: "default",
	Dark: false,

	SurfaceOverlay:     hex("#d0d0d0"), // 252
	SurfaceHelp:        hex("#e4e4e4"), // 254
	SurfaceToast:       hex("#e4e4e4"), // 254
	SurfaceStatusBar:   hex("#303030"), // 236, unconditional today
	SurfaceSelected:    hex("#5f5fff"), // 63, unconditional today
	SurfaceSelfMention: hex("#ff87ff"), // 213, unconditional today
	SurfaceCode:        hex("#303030"), // 236, unconditional today

	TextDim:             hex("#8a8a8a"), // 245, unconditional today
	TextMuted:           hex("#808080"), // 244, unconditional today
	TextFaint:           hex("#808080"), // 8, unconditional today
	TextSubtle:          hex("#585858"), // 240, unconditional today
	TextOnSurface:       hex("#444444"), // 238
	TextStatusBar:       hex("#eeeeee"),
	TextOnSelected:      hex("#ffffff"),
	TextOnSelectedMuted: hex("#e4e4e4"), // 254, unconditional today
	TextOnToast:         hex("#303030"), // 236
	TextModeLabel:       hex("#ffffff"), // 231, unconditional today
	TextCode:            hex("#d0d0d0"), // 252, unconditional today

	Accent:           hex("#00afff"), // 39, unconditional today
	AccentOnSurface:  hex("#005faf"), // 25
	AccentInsert:     hex("#00d700"), // 40, unconditional today
	AccentModeNormal: hex("#0087ff"), // 33, unconditional today
	AccentModeInsert: hex("#00af5f"), // 35, unconditional today

	StatusError:     hex("#d70000"), // 160
	StatusWarning:   hex("#af5f00"), // 130
	StatusInfo:      hex("#5fafff"), // 75, unconditional today
	StatusOnline:    hex("#5fd75f"), // 10, unconditional today
	TickSent:        hex("#8a8a8a"), // 245, unconditional today
	TickOutbox:      hex("#8a8a8a"), // 245, unconditional today
	TickRead:        hex("#5f87ff"), // 12, unconditional today
	TickFailed:      hex("#ff5f5f"), // 9, unconditional today
	NameIncoming:    hex("#5fd75f"), // 10, unconditional today
	NameEditing:     hex("#ffd75f"), // 11, unconditional today
	Indicator:       hex("#00afaf"), // 6, unconditional today
	UnreadSeparator: hex("#5f87ff"), // 12, unconditional today
	WaveformPlayed:  hex("#5f87ff"), // 12, unconditional today
	ReactionChosen:  hex("#5fd75f"), // 10, unconditional today
	UnreadReaction:  hex("#ff5faf"), // 205, unconditional today
	UnreadMention:   hex("#00afff"), // 39, unconditional today

	BorderPaneActive:      hex("#005f00"), // 22
	BorderBubbleIn:        hex("#444444"), // 238, unconditional today
	BorderBubbleOut:       hex("#005faf"), // 25, unconditional today
	BorderOverlay:         hex("#8a8a8a"), // 245
	BorderComposerFocused: hex("#008700"), // 28
	BorderComposerFlash:   hex("#d70000"), // 160
	BorderStatusSep:       hex("#585858"), // 240, unconditional today

	MarkupLink:          hex("#005faf"), // 25
	MarkupRef:           hex("#870087"), // 90
	MarkupSelfMentionFg: hex("#000000"), // 0, unconditional today

	HighlightAccent:     hex("#ff8c00"),
	HighlightError:      hex("#d70000"),
	HighlightBaseChat:   hex("#bcbcbc"), // 250, unconditional today
	HighlightBaseBubble: hex("#444444"), // 238, unconditional today
	OverlayDim:          hex("#bcbcbc"), // 250

	ComposerCounterDim: hex("#444444"), // 238
	ComposerGlyphIdle:  hex("#585858"), // 240, unconditional today
	ComposerGlyphReady: hex("#005fff"), // 27

	SenderPalette: [8]color.Color{
		hex("#af0000"), // 1
		hex("#008700"), // 2
		hex("#0000af"), // 4
		hex("#8700af"), // 5
		hex("#00afaf"), // 6
		hex("#af5f00"), // 130
		hex("#8700d7"), // 92
		hex("#005f00"), // 28
	},

	LogoGradient: [5]GradientStop{
		{0.00, 14, 18, 40},
		{0.30, 38, 65, 129},
		{0.60, 88, 136, 208},
		{0.85, 148, 192, 242},
		{1.00, 198, 228, 255},
	},
}
