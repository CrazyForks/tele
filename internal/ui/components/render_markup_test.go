package components_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sorokin-vladimir/tele/internal/markup"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
)

// The seam between the two halves of the feature: what the composer parses must
// be what the chat view styles. Guards against an offset convention drifting
// between markup.Parse and RenderEntities (#152 AC 4).
func TestComposedMarkupRendersStyled(t *testing.T) {
	text, ents := markup.Parse("привет **важно**")
	out := components.RenderEntities(text, ents)
	assert.NotEqual(t, text, out, "expected styling escapes to be applied")
	assert.Contains(t, out, "важно")
	assert.Contains(t, out, "\x1b[1m", "expected SGR 1 (bold)")
}

func TestComposedMarkupWithEmojiRendersStyled(t *testing.T) {
	// Surrogate pairs are where an offset mismatch between the two sides would
	// show up first.
	text, ents := markup.Parse("**жирный 😀** хвост")
	out := components.RenderEntities(text, ents)
	assert.Contains(t, out, "жирный 😀")
	assert.True(t, strings.Contains(out, "хвост"))
}
