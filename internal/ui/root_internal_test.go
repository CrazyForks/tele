package ui

import (
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

// Choosing the InputMedia for a staged file left this package with #195: it is
// protocol work, and it now lives with the upload in internal/core. See
// TestUploadPart_* in internal/core/media_build_test.go.
