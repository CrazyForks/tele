package project

import "github.com/sorokin-vladimir/tele/internal/domain"

// ChatContents is everything a chat subscription currently shows: the message
// window plus the header and per-chat state the pane renders around it.
//
// #193 adds the outbox entries for this chat here, alongside the window: the
// durable outbox replaces the client-side optimistic sentinel messages.
type ChatContents struct {
	ChatID          int64
	Title           string
	IsUser          bool
	Online          bool
	Messages        []domain.Message
	AnchorMsgID     int
	HasOlder        bool
	HasNewer        bool
	ReadInboxMaxID  int
	ReadOutboxMaxID int
	Draft           string
	// Typing is ephemeral and has no persisted state to read, so the registry
	// fills it rather than this builder.
	Typing string
}

// BuildChat resolves the window's anchor against the stored history and slices
// out Before..After around it.
//
// HasOlder and HasNewer report what the store holds outside the window; they
// drive the client's scroll affordance and say nothing about Telegram. It never
// fetches: a window that comes back shorter than it asked for is how the core
// learns the store fell short (see Owner.needsBackfill).
func BuildChat(r Reader, w ChatWindow) ChatContents {
	out := ChatContents{ChatID: w.ChatID}
	chat, ok := r.GetChat(w.ChatID)
	if ok {
		out.Title = chat.Title
		out.IsUser = chat.Peer.IsUser()
		out.Online = chat.Online
		out.ReadInboxMaxID = chat.ReadInboxMaxID
		out.ReadOutboxMaxID = chat.ReadOutboxMaxID
		out.Draft = chat.Draft
	}

	all := r.Messages(w.ChatID)
	if len(all) == 0 {
		return out
	}

	idx, anchorID := resolveAnchor(all, chat, w.Anchor)
	out.AnchorMsgID = anchorID
	if idx < 0 {
		// The anchor names a message the store does not hold, so the window
		// cannot be sliced at all. HasOlder stays false: it reports what the
		// store holds outside the window, and the store holds nothing usable
		// here. The shortfall is what tells the core to fetch.
		return out
	}

	start := idx - w.Before
	if start < 0 {
		start = 0
	}
	end := idx + w.After + 1
	if end > len(all) {
		end = len(all)
	}
	out.Messages = all[start:end]
	out.HasOlder = start > 0
	out.HasNewer = end < len(all)
	return out
}

// resolveAnchor returns the index of the anchor message in all, oldest first,
// and its id. An index of -1 means the anchor names a message the store does not
// hold.
func resolveAnchor(all []domain.Message, chat domain.Chat, a Anchor) (int, int) {
	switch a.Kind {
	case AnchorMessage:
		for i, m := range all {
			if m.ID == a.MsgID {
				return i, m.ID
			}
		}
		return -1, a.MsgID

	case AnchorFirstUnread:
		if chat.UnreadCount > 0 {
			for i, m := range all {
				if m.ID > chat.ReadInboxMaxID {
					return i, m.ID
				}
			}
		}
		// No unread, or the read pointer is past everything stored: the anchor
		// is the newest message.
		fallthrough

	default: // AnchorNewest
		last := len(all) - 1
		return last, all[last].ID
	}
}
