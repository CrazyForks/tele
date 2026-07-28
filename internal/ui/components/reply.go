package components

import (
	"strings"

	runewidth "github.com/mattn/go-runewidth"
	"github.com/sorokin-vladimir/tele/internal/domain"
)

// BuildEditPreview returns a two-line preview string for the edit bar above the composer:
//
//	line 1: "▌ Edit Message"
//	line 2: "▌ first-line-of-text (truncated to 40 runes)"
func BuildEditPreview(msg domain.Message) string {
	snippet := msg.Text
	if idx := strings.IndexByte(snippet, '\n'); idx >= 0 {
		snippet = snippet[:idx]
	}
	snippet = runewidth.Truncate(snippet, 39, "…")
	nameLine := quoteGlyph + editNameStyle.Render("Edit Message")
	snippetLine := quoteGlyph + quoteStyle.Render(snippet)
	return nameLine + "\n" + snippetLine
}

// BuildReplyPreview returns a two-line preview string for the reply bar above the composer:
//
//	line 1: "▌ SenderName"
//	line 2: "▌ first-line-of-text (truncated to 40 runes)"
func BuildReplyPreview(msg domain.Message) string {
	name := msg.SenderName
	if name == "" {
		if msg.IsOut {
			name = "You"
		} else {
			name = "?"
		}
	}

	snippet := msg.Text
	if idx := strings.IndexByte(snippet, '\n'); idx >= 0 {
		snippet = snippet[:idx]
	}
	snippet = runewidth.Truncate(snippet, 39, "…")

	nameLine := quoteGlyph + inNameStyle.Render(name)
	snippetLine := quoteGlyph + quoteStyle.Render(snippet)
	return nameLine + "\n" + snippetLine
}
