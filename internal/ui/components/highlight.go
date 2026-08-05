package components

import (
	"image/color"
	"time"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

// Highlight fade timing, shared by the message-bubble and chat-list highlights.
// A fresh highlight starts at HighlightInitialStep and counts down to 0 over
// ticks spaced HighlightFadeInterval apart. It stays at full accent for the
// hold ticks, then fades to the base color over the last HighlightFadeSteps
// ticks. Total ≈ HighlightInitialStep * HighlightFadeInterval (≈3s).
const (
	// HighlightFadeSteps is the number of trailing ticks over which the accent
	// fades back to the base color.
	HighlightFadeSteps = 5
	// HighlightHoldSteps is the number of leading ticks the highlight stays at
	// full accent before the fade begins, so the cue is easy to notice.
	HighlightHoldSteps = 10
	// HighlightInitialStep is the step value a fresh highlight starts at: the
	// full-accent hold followed by the fade.
	HighlightInitialStep = HighlightHoldSteps + HighlightFadeSteps
	// HighlightFadeInterval is the delay between successive fade ticks.
	HighlightFadeInterval = 200 * time.Millisecond
)

// HighlightKind selects which accent a highlight fades from: the amber info tone
// (jump-to) or the red error tone (optimistic-action rollback).
type HighlightKind int

const (
	HighlightInfo HighlightKind = iota
	HighlightError
)

// FadeAccentColor linearly interpolates RGB from base (step 0) toward accent
// (step == total), returning a truecolor lipgloss color. lipgloss downsamples
// on limited-color terminals. step is clamped to [0, total].
func FadeAccentColor(accent, base color.Color, step, total int) color.Color {
	if total <= 0 {
		return accent
	}
	if step < 0 {
		step = 0
	}
	if step > total {
		step = total
	}
	ar, ag, ab := rgb8(accent)
	br, bg, bb := rgb8(base)
	t := float64(step) / float64(total)
	lerp := func(from, to uint8) uint8 {
		return uint8(float64(from) + (float64(to)-float64(from))*t)
	}
	return theme.Hex(lerp(br, ar), lerp(bg, ag), lerp(bb, ab))
}

func rgb8(c color.Color) (uint8, uint8, uint8) {
	r, g, b, _ := c.RGBA()
	return uint8(r >> 8), uint8(g >> 8), uint8(b >> 8)
}
