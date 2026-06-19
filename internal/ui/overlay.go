package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
)

// stampOverlay writes overlayLines into baseLines starting at (top, left).
func stampOverlay(baseLines, overlayLines []string, top, left int) {
	for i, mLine := range overlayLines {
		row := top + i
		if row < 0 || row >= len(baseLines) {
			continue
		}
		bLine := baseLines[row]
		if lipgloss.Width(bLine) < left {
			bLine += strings.Repeat(" ", left-lipgloss.Width(bLine))
		}
		mLineW := lipgloss.Width(mLine)
		right := left + mLineW
		prefix := xansi.Truncate(bLine, left, "")
		suffix := ""
		if lipgloss.Width(bLine) > right {
			suffix = xansi.TruncateLeft(bLine, right, "")
		}
		baseLines[row] = prefix + mLine + suffix
	}
}

// stampBoxOverlay overlays boxLines onto base at (top,left), treating the box as
// exactly boxW display cells wide. Unlike stampOverlay it does not measure the box
// lines with lipgloss (they contain Kitty placeholders whose width lipgloss cannot
// compute); the right edge is the integer left+boxW.
func stampBoxOverlay(base string, boxLines []string, top, left, boxW, h int) string {
	baseLines := strings.Split(base, "\n")
	for len(baseLines) < h {
		baseLines = append(baseLines, "")
	}
	for i, bl := range boxLines {
		row := top + i
		if row < 0 || row >= len(baseLines) {
			continue
		}
		b := baseLines[row]
		prefix := xansi.Truncate(b, left, "")
		if w := lipgloss.Width(prefix); w < left {
			prefix += strings.Repeat(" ", left-w)
		}
		suffix := ""
		if lipgloss.Width(b) > left+boxW {
			suffix = xansi.TruncateLeft(b, left+boxW, "")
		}
		baseLines[row] = prefix + bl + suffix
	}
	return strings.Join(baseLines, "\n")
}

// overlayCenter stamps overlay string centered over base string (w×h terminal).
func overlayCenter(base, overlay string, w, h int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	overlayH := len(overlayLines)
	overlayW := 0
	for _, l := range overlayLines {
		if ww := lipgloss.Width(l); ww > overlayW {
			overlayW = ww
		}
	}

	top := (h - overlayH) / 2
	left := (w - overlayW) / 2
	if left < 0 {
		left = 0
	}

	for len(baseLines) < h {
		baseLines = append(baseLines, "")
	}

	stampOverlay(baseLines, overlayLines, top, left)

	return strings.Join(baseLines, "\n")
}

// overlayAt stamps overlay onto base at the given top/left (terminal cells).
// w/h are the terminal dimensions; base is padded to h rows if shorter.
func overlayAt(base, overlay string, w, h, top, left int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	for len(baseLines) < h {
		baseLines = append(baseLines, "")
	}

	stampOverlay(baseLines, overlayLines, top, left)
	return strings.Join(baseLines, "\n")
}

// measureBox returns the display width (widest line) and height (line count) of
// a multi-line string.
func measureBox(s string) (w, h int) {
	lines := strings.Split(s, "\n")
	h = len(lines)
	for _, l := range lines {
		if ww := lipgloss.Width(l); ww > w {
			w = ww
		}
	}
	return w, h
}

// anchorMenu computes the top-left position (terminal cells) for a menu of size
// menuW x menuH placed next to bubble. onLeft places the menu to the left of the
// bubble (outgoing messages); otherwise to the right (incoming). The result is
// clamped into area so the menu stays within the chat panel viewport.
func anchorMenu(bubble, area components.Rect, menuW, menuH int, onLeft bool) (top, left int) {
	top = bubble.Top
	if maxTop := area.Top + area.Height - menuH; top > maxTop {
		top = maxTop
	}
	if top < area.Top {
		top = area.Top
	}

	if onLeft {
		left = bubble.Left - menuW
	} else {
		left = bubble.Left + bubble.Width
	}
	if maxLeft := area.Left + area.Width - menuW; left > maxLeft {
		left = maxLeft
	}
	if left < area.Left {
		left = area.Left
	}
	return top, left
}

// overlayBottomRight stamps overlay string in the bottom-right corner of a
// base string (w×h terminal). bottomOffset shifts the overlay upward by that
// many additional rows (use to clear a bottom status bar or input area).
func overlayBottomRight(base, overlay string, w, h, bottomOffset int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	overlayH := len(overlayLines)
	overlayW := 0
	for _, l := range overlayLines {
		if ww := lipgloss.Width(l); ww > overlayW {
			overlayW = ww
		}
	}

	top := h - overlayH - 1 - bottomOffset
	if top < 0 {
		top = 0
	}
	left := w - overlayW - 2
	if left < 0 {
		left = 0
	}

	for len(baseLines) < h {
		baseLines = append(baseLines, "")
	}

	stampOverlay(baseLines, overlayLines, top, left)

	return strings.Join(baseLines, "\n")
}
