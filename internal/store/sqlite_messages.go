package store

import "time"

// Message operations are in-memory only — messages load on demand per chat open.

func (s *SQLiteStore) Messages(chatID int64) []Message {
	s.mu.RLock()
	defer s.mu.RUnlock()
	msgs := s.messages[chatID]
	if msgs == nil {
		return nil
	}
	cp := make([]Message, len(msgs))
	copy(cp, msgs)
	return cp
}

func (s *SQLiteStore) SetMessages(chatID int64, msgs []Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make([]Message, len(msgs))
	copy(cp, msgs)
	// Re-index this chat: drop entries for the replaced messages, then add the
	// new ones if the chat lives in the shared pts box.
	for _, m := range s.messages[chatID] {
		delete(s.msgChat, m.ID)
	}
	s.messages[chatID] = cp
	s.capMessagesLocked(chatID)
	if chat, ok := s.chats[chatID]; ok && sharedPtsBox(chat.Peer) {
		for _, m := range s.messages[chatID] {
			s.msgChat[m.ID] = chatID
		}
	}
}

// capMessagesLocked trims a chat's message slice to the newest MaxMessagesPerChat,
// dropping the oldest from the front and clearing their index entries. Caller
// holds the lock. See issue #73.
func (s *SQLiteStore) capMessagesLocked(chatID int64) {
	msgs := s.messages[chatID]
	if len(msgs) <= MaxMessagesPerChat {
		return
	}
	drop := len(msgs) - MaxMessagesPerChat
	for _, m := range msgs[:drop] {
		delete(s.msgChat, m.ID)
	}
	s.messages[chatID] = msgs[drop:]
}

// BumpChatLastMessage updates a chat's last-message preview and moves it up in
// the list, WITHOUT appending to the chat's message slice. It optimistically
// surfaces a chat that just received an outgoing message sent from elsewhere
// (e.g. a forward target), whose full message arrives later via the update
// stream (or on next open). No-op if the chat is unknown.
func (s *SQLiteStore) BumpChatLastMessage(chatID int64, msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	chat, ok := s.chats[chatID]
	if !ok {
		return
	}
	m := msg
	chat.LastMessage = &m
	s.chats[chatID] = chat
	s.orderDirty = true // newer last-message moves the chat in the list
	s.markDirtyLocked(chatID)
}

func (s *SQLiteStore) AppendMessage(msg Message) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.messages[msg.ChatID] = append(s.messages[msg.ChatID], msg)
	if chat, ok := s.chats[msg.ChatID]; ok {
		m := msg
		chat.LastMessage = &m
		s.chats[msg.ChatID] = chat
		s.orderDirty = true // newer last-message moves the chat in the list
		if sharedPtsBox(chat.Peer) {
			s.msgChat[msg.ID] = msg.ChatID
		}
		s.markDirtyLocked(msg.ChatID) // write-behind: last-message persists on flush
	}
	s.capMessagesLocked(msg.ChatID)
}

func (s *SQLiteStore) UpdateMessageID(chatID int64, oldID, newID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.messages[chatID] {
		if s.messages[chatID][i].ID == oldID {
			s.messages[chatID][i].ID = newID
			if cid, ok := s.msgChat[oldID]; ok {
				delete(s.msgChat, oldID)
				s.msgChat[newID] = cid
			}
			return
		}
	}
}

// localMediaByIDLocked returns a pointer to the LocalMedia of the message with
// the given ID. Optimistic sentinel IDs are unique (negative) and the msgChat
// index only covers shared-pts peers, so this scans all chats. Caller holds s.mu.
func (s *SQLiteStore) localMediaByIDLocked(id int) *LocalMedia {
	for chatID := range s.messages {
		for i := range s.messages[chatID] {
			if s.messages[chatID][i].ID == id {
				return s.messages[chatID][i].LocalMedia
			}
		}
	}
	return nil
}

func (s *SQLiteStore) UpdateLocalMediaProgress(sentinelID int, frac float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if lm := s.localMediaByIDLocked(sentinelID); lm != nil {
		lm.UploadProgress = frac
	}
}

func (s *SQLiteStore) MarkLocalMediaFailed(sentinelID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if lm := s.localMediaByIDLocked(sentinelID); lm != nil {
		lm.UploadState = UploadFailed
	}
}

func (s *SQLiteStore) ClearLocalMedia(sentinelID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for chatID := range s.messages {
		for i := range s.messages[chatID] {
			if s.messages[chatID][i].ID == sentinelID {
				s.messages[chatID][i].LocalMedia = nil
				return
			}
		}
	}
}

func (s *SQLiteStore) AdoptServerMedia(chatID int64, msgID int, photo *PhotoRef, doc *DocumentRef, media *MediaRef) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.messages[chatID] {
		if s.messages[chatID][i].ID == msgID {
			s.messages[chatID][i].Photo = photo
			s.messages[chatID][i].Document = doc
			s.messages[chatID][i].Media = media
			s.messages[chatID][i].LocalMedia = nil
			return
		}
	}
}

// UpdateMessageText replaces a message's text and its entities together. They
// must move as a unit: entity offsets address the text they were parsed from,
// so keeping the old ones would leave them pointing at characters that changed.
func (s *SQLiteStore) UpdateMessageText(chatID int64, msgID int, text string, entities []MessageEntity, editDate time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.messages[chatID] {
		if s.messages[chatID][i].ID == msgID {
			s.messages[chatID][i].Text = text
			cp := make([]MessageEntity, len(entities))
			copy(cp, entities)
			s.messages[chatID][i].Entities = cp
			t := editDate
			s.messages[chatID][i].EditDate = &t
			return
		}
	}
}

func (s *SQLiteStore) UpdateMessageReactions(chatID int64, msgID int, reactions []Reaction) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.messages[chatID] {
		if s.messages[chatID][i].ID == msgID {
			cp := make([]Reaction, len(reactions))
			copy(cp, reactions)
			s.messages[chatID][i].Reactions = cp
			return
		}
	}
}

// UpdateMessageMedia replaces the photo/document refs of a cached message. A nil
// ref leaves that field unchanged.
func (s *SQLiteStore) UpdateMessageMedia(chatID int64, msgID int, photo *PhotoRef, document *DocumentRef) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.messages[chatID] {
		if s.messages[chatID][i].ID == msgID {
			if photo != nil {
				s.messages[chatID][i].Photo = photo
			}
			if document != nil {
				s.messages[chatID][i].Document = document
			}
			return
		}
	}
}

func (s *SQLiteStore) RemoveMessage(chatID int64, msgID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs := s.messages[chatID]
	for i, m := range msgs {
		if m.ID == msgID {
			s.messages[chatID] = append(msgs[:i], msgs[i+1:]...)
			delete(s.msgChat, msgID)
			return
		}
	}
}

func (s *SQLiteStore) RemoveMessages(chatID int64, msgIDs []int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeMessagesLocked(chatID, msgIDs)
}

// removeMessagesLocked drops the given message IDs from one chat and the msgChat
// index. Caller holds the lock.
func (s *SQLiteStore) removeMessagesLocked(chatID int64, msgIDs []int) {
	if len(s.messages[chatID]) == 0 {
		return
	}
	toRemove := make(map[int]struct{}, len(msgIDs))
	for _, id := range msgIDs {
		toRemove[id] = struct{}{}
	}
	msgs := s.messages[chatID]
	kept := msgs[:0]
	for _, m := range msgs {
		if _, remove := toRemove[m.ID]; remove {
			delete(s.msgChat, m.ID)
			continue
		}
		kept = append(kept, m)
	}
	s.messages[chatID] = kept
}

// RemoveMessagesByID resolves each message ID to its owning chat via the index
// and removes it there, returning the affected chat IDs. Used for the Telegram
// non-channel delete that carries message IDs but no peer context (issue #72).
func (s *SQLiteStore) RemoveMessagesByID(msgIDs []int) []int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	byChat := make(map[int64][]int)
	for _, id := range msgIDs {
		if cid, ok := s.msgChat[id]; ok {
			byChat[cid] = append(byChat[cid], id)
		}
	}
	affected := make([]int64, 0, len(byChat))
	for cid, ids := range byChat {
		s.removeMessagesLocked(cid, ids)
		affected = append(affected, cid)
	}
	return affected
}
