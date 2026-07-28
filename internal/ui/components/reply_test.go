package components_test

import (
	"strings"
	"testing"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/stretchr/testify/assert"
)

func TestBuildReplyPreview_ContainsName(t *testing.T) {
	msg := domain.Message{SenderName: "Alice", Text: "hello"}
	preview := components.BuildReplyPreview(msg)
	assert.Contains(t, preview, "Alice")
}

func TestBuildReplyPreview_ContainsSnippet(t *testing.T) {
	msg := domain.Message{SenderName: "Alice", Text: "hello world"}
	preview := components.BuildReplyPreview(msg)
	assert.Contains(t, preview, "hello world")
}

func TestBuildReplyPreview_TwoLines(t *testing.T) {
	msg := domain.Message{SenderName: "Bob", Text: "hello"}
	preview := components.BuildReplyPreview(msg)
	assert.Equal(t, 1, strings.Count(preview, "\n"), "preview must be exactly two lines")
}

func TestBuildReplyPreview_OutgoingEmptyName(t *testing.T) {
	msg := domain.Message{SenderName: "", IsOut: true, Text: "hi"}
	preview := components.BuildReplyPreview(msg)
	assert.Contains(t, preview, "You")
}

func TestBuildReplyPreview_IncomingEmptyName(t *testing.T) {
	msg := domain.Message{SenderName: "", IsOut: false, Text: "hi"}
	preview := components.BuildReplyPreview(msg)
	assert.Contains(t, preview, "?")
}

func TestBuildReplyPreview_LongTextTruncated(t *testing.T) {
	msg := domain.Message{SenderName: "Bob", Text: strings.Repeat("x", 50)}
	preview := components.BuildReplyPreview(msg)
	assert.Contains(t, preview, "…")
}

func TestBuildReplyPreview_MultilineTextUsesFirstLine(t *testing.T) {
	msg := domain.Message{SenderName: "Eve", Text: "first line\nsecond line"}
	preview := components.BuildReplyPreview(msg)
	assert.Contains(t, preview, "first line")
	assert.NotContains(t, preview, "second line")
}

func TestBuildEditPreview_ContainsLabel(t *testing.T) {
	msg := domain.Message{Text: "hello"}
	got := components.BuildEditPreview(msg)
	assert.Contains(t, got, "Edit Message")
}

func TestBuildEditPreview_ContainsSnippet(t *testing.T) {
	msg := domain.Message{Text: "hello world"}
	got := components.BuildEditPreview(msg)
	assert.Contains(t, got, "hello world")
}

func TestBuildEditPreview_TwoLines(t *testing.T) {
	msg := domain.Message{Text: "hello"}
	got := components.BuildEditPreview(msg)
	assert.Equal(t, 1, strings.Count(got, "\n"), "preview must be exactly two lines")
}

func TestBuildEditPreview_LongTextTruncated(t *testing.T) {
	msg := domain.Message{Text: strings.Repeat("x", 50)}
	got := components.BuildEditPreview(msg)
	assert.Contains(t, got, "…")
}

func TestBuildEditPreview_MultilineUsesFirstLine(t *testing.T) {
	msg := domain.Message{Text: "first\nsecond"}
	got := components.BuildEditPreview(msg)
	assert.Contains(t, got, "first")
	assert.NotContains(t, got, "second")
}

func TestBuildReplyPreview_CJKSnippetTruncated(t *testing.T) {
	// 25 CJK chars = 50 visual cols > 39 limit, but len(runes)=25 < 40, so
	// rune-based code does NOT truncate. runewidth-based code must.
	msg := domain.Message{SenderName: "Bob", Text: strings.Repeat("中", 25)}
	preview := components.BuildReplyPreview(msg)
	assert.Contains(t, preview, "…")
}

func TestBuildEditPreview_CJKSnippetTruncated(t *testing.T) {
	msg := domain.Message{Text: strings.Repeat("中", 25)}
	got := components.BuildEditPreview(msg)
	assert.Contains(t, got, "…")
}
