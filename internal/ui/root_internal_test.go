package ui

import (
	"testing"

	tg "github.com/gotd/td/tg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/domain"
)

// rowsOf renders domain chats as the projection rows a test feeds the list
// component, so a fixture can go on describing chats while the component
// consumes rows.
func rowsOf(chats []domain.Chat) []project.ChatRow {
	out := make([]project.ChatRow, 0, len(chats))
	for _, c := range chats {
		out = append(out, project.ChatRow{
			ID:         c.ID,
			Title:      c.Title,
			IsUser:     c.Peer.IsUser(),
			Online:     c.Online,
			Unread:     c.UnreadCount,
			Mentions:   c.UnreadMentionsCount,
			Reactions:  c.UnreadReactionsCount,
			UnreadMark: c.UnreadMark,
			Muted:      c.IsMuted,
		})
	}
	return out
}

func TestMediaBuilderFor_FileBuildsForcedDocument(t *testing.T) {
	att := &pendingAttachment{name: "report.pdf", mime: "application/pdf", sendAs: domain.MediaFile}
	build, ok := mediaBuilderFor(att)
	require.True(t, ok)
	media := build(&tg.InputFile{ID: 1})
	doc, ok := media.(*tg.InputMediaUploadedDocument)
	require.True(t, ok, "got %T, want *tg.InputMediaUploadedDocument", media)
	assert.True(t, doc.ForceFile)
	assert.Equal(t, "application/pdf", doc.MimeType)
	require.Len(t, doc.Attributes, 1)
	fn, ok := doc.Attributes[0].(*tg.DocumentAttributeFilename)
	require.True(t, ok)
	assert.Equal(t, "report.pdf", fn.FileName)
}

func TestMediaBuilderFor_PhotoStillSupported(t *testing.T) {
	att := &pendingAttachment{sendAs: domain.MediaPhoto}
	_, ok := mediaBuilderFor(att)
	assert.True(t, ok)
}

func TestMediaBuilderFor_VideoUnsupported(t *testing.T) {
	att := &pendingAttachment{sendAs: domain.MediaVideo}
	_, ok := mediaBuilderFor(att)
	assert.False(t, ok, "video send-as is #107, not yet supported")
}
