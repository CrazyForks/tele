package store

// recomputeUnreadLocked restores the unread invariant for c: the count is the
// last server-authoritative baseline plus the inbound messages observed locally
// above the read pointer. It is the only writer of Chat.UnreadCount inside the
// store, so the value can never drift the way a bare increment could. Caller
// holds the lock.
func (s *SQLiteStore) recomputeUnreadLocked(c *Chat) {
	c.UnreadCount = s.baselineUnread[c.ID] + len(s.unreadMsgs[c.ID])
}

// ApplyUnreadMessage records an inbound message as unread for its chat and
// returns whether the chat's unread count changed. It is idempotent per message
// ID, so a replayed or duplicated update cannot inflate the count. No-op for an
// unknown chat or for a message at or below the read pointer (already read
// elsewhere and arriving via getDifference catch-up).
//
// The caller decides what counts as inbound; see ApplyIncomingMessage.
func (s *SQLiteStore) ApplyUnreadMessage(chatID int64, msgID int) bool {
	s.mu.Lock()
	c, ok := s.chats[chatID]
	if !ok || msgID <= c.ReadInboxMaxID {
		s.mu.Unlock()
		return false
	}
	tracked := s.unreadMsgs[chatID]
	if _, isTracked := tracked[msgID]; isTracked {
		s.mu.Unlock()
		return false
	}
	if tracked == nil {
		tracked = make(map[int]struct{})
		s.unreadMsgs[chatID] = tracked
	}
	tracked[msgID] = struct{}{}
	s.recomputeUnreadLocked(&c)
	s.chats[chatID] = c
	s.markDirtyLocked(chatID)
	s.mu.Unlock()
	return true
}
