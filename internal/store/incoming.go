package store

// ApplyIncomingMessage records a newly received message and updates the
// account-level unread state it implies. It takes no viewport: unread is a
// property of the account, not of whichever chat a client happens to have open,
// so the same event produces the same state with several clients attached or
// none at all (#189, groundwork for #183).
//
// Returns true when a chat's unread or mention count changed, so callers can
// refresh derived views such as folder counts.
func ApplyIncomingMessage(s Store, msg Message) bool {
	s.AppendMessage(msg)
	if msg.IsOut {
		return false
	}
	changed := s.ApplyUnreadMessage(msg.ChatID, msg.ID)
	if msg.Mentioned && s.ApplyUnreadMention(msg.ChatID, msg.ID, true) {
		changed = true
	}
	return changed
}
