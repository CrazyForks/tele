package store

import (
	"encoding/json"
	"sort"
	"time"

	"go.uber.org/zap"
)

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

	newIDs := make(map[int]struct{}, len(cp))
	for _, m := range cp {
		newIDs[m.ID] = struct{}{}
	}
	// Re-index this chat: drop entries for the replaced messages and mark rows the
	// new set no longer contains for deletion on disk.
	for _, m := range s.messages[chatID] {
		delete(s.msgChat, m.ID)
		if _, keep := newIDs[m.ID]; !keep {
			s.markMsgDeletedLocked(chatID, m.ID)
		}
	}
	s.messages[chatID] = cp
	s.capMessagesLocked(chatID)
	if chat, ok := s.chats[chatID]; ok && sharedPtsBox(chat.Peer) {
		for _, m := range s.messages[chatID] {
			s.msgChat[m.ID] = chatID
		}
	}
	for _, m := range s.messages[chatID] {
		s.markMsgDirtyLocked(chatID, m.ID)
	}
}

// markMsgDirtyLocked queues an upsert of (chatID, msgID) for the next flush.
// Optimistic sentinel messages (negative ids) are session-only and never
// persisted. Caller holds the lock.
func (s *SQLiteStore) markMsgDirtyLocked(chatID int64, msgID int) {
	if msgID <= 0 {
		return
	}
	if d := s.deletedMsgs[chatID]; d != nil {
		delete(d, msgID)
	}
	m := s.dirtyMsgs[chatID]
	if m == nil {
		m = make(map[int]struct{})
		s.dirtyMsgs[chatID] = m
	}
	m[msgID] = struct{}{}
}

// markMsgDeletedLocked queues a delete of (chatID, msgID) for the next flush.
// Caller holds the lock.
func (s *SQLiteStore) markMsgDeletedLocked(chatID int64, msgID int) {
	if msgID <= 0 {
		return
	}
	if d := s.dirtyMsgs[chatID]; d != nil {
		delete(d, msgID)
	}
	m := s.deletedMsgs[chatID]
	if m == nil {
		m = make(map[int]struct{})
		s.deletedMsgs[chatID] = m
	}
	m[msgID] = struct{}{}
}

type msgUpsert struct {
	chatID int64
	msgID  int
	date   int64
	data   []byte
}

type msgDelete struct {
	chatID int64
	msgID  int
}

// snapshotMessageWritesLocked drains the dirty/deleted message sets into flat
// slices, reading upsert payloads from the current in-memory messages. Caller
// holds the lock.
func (s *SQLiteStore) snapshotMessageWritesLocked() ([]msgUpsert, []msgDelete) {
	var upserts []msgUpsert
	for chatID, ids := range s.dirtyMsgs {
		for _, m := range s.messages[chatID] {
			if _, ok := ids[m.ID]; !ok {
				continue
			}
			b, err := json.Marshal(m)
			if err != nil {
				s.log.Error("marshal message failed", zap.Int64("chat_id", chatID), zap.Int("msg_id", m.ID), zap.Error(err))
				continue
			}
			upserts = append(upserts, msgUpsert{chatID: chatID, msgID: m.ID, date: m.Date.Unix(), data: b})
		}
	}
	s.dirtyMsgs = make(map[int64]map[int]struct{})

	var deletes []msgDelete
	for chatID, ids := range s.deletedMsgs {
		for msgID := range ids {
			deletes = append(deletes, msgDelete{chatID: chatID, msgID: msgID})
		}
	}
	s.deletedMsgs = make(map[int64]map[int]struct{})
	return upserts, deletes
}

// flushMessageRows applies queued upserts and deletes in one transaction. Runs
// off-lock. Logs errors; the Store interface does not propagate them.
func (s *SQLiteStore) flushMessageRows(upserts []msgUpsert, deletes []msgDelete) {
	if len(upserts) == 0 && len(deletes) == 0 {
		return
	}
	tx, err := s.db.Begin()
	if err != nil {
		s.log.Error("begin message flush failed", zap.Error(err))
		return
	}
	for _, u := range upserts {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO messages(chat_id, msg_id, date, data) VALUES (?, ?, ?, ?)`,
			u.chatID, u.msgID, u.date, u.data); err != nil {
			_ = tx.Rollback()
			s.log.Error("upsert message failed", zap.Int64("chat_id", u.chatID), zap.Int("msg_id", u.msgID), zap.Error(err))
			return
		}
	}
	for _, d := range deletes {
		if _, err := tx.Exec(`DELETE FROM messages WHERE chat_id = ? AND msg_id = ?`, d.chatID, d.msgID); err != nil {
			_ = tx.Rollback()
			s.log.Error("delete message failed", zap.Int64("chat_id", d.chatID), zap.Int("msg_id", d.msgID), zap.Error(err))
			return
		}
	}
	if err := tx.Commit(); err != nil {
		s.log.Error("commit message flush failed", zap.Error(err))
	}
}

// mergeMessagesByID unions a chat's on-disk tail (disk) with its in-memory
// messages (mem), keeping the in-memory copy on an id collision (live updates
// are fresher) and returning the result sorted by (date, id).
func mergeMessagesByID(disk, mem []Message) []Message {
	seen := make(map[int]struct{}, len(mem))
	for _, m := range mem {
		seen[m.ID] = struct{}{}
	}
	out := make([]Message, 0, len(disk)+len(mem))
	for _, d := range disk {
		if _, ok := seen[d.ID]; !ok {
			out = append(out, d)
		}
	}
	out = append(out, mem...)
	sort.SliceStable(out, func(i, j int) bool {
		if !out[i].Date.Equal(out[j].Date) {
			return out[i].Date.Before(out[j].Date)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// LoadMessages loads a chat's persisted message tail into memory on first open,
// merging it with any messages already held (live updates). Idempotent per chat;
// a pure read that never queues a write. See issue #139.
func (s *SQLiteStore) LoadMessages(chatID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loaded[chatID] {
		return
	}
	s.loaded[chatID] = true

	rows, err := s.db.Query(`SELECT data FROM messages WHERE chat_id = ? ORDER BY date, msg_id`, chatID)
	if err != nil {
		s.log.Error("load messages failed", zap.Int64("chat_id", chatID), zap.Error(err))
		return
	}
	defer func() { _ = rows.Close() }()

	var disk []Message
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			s.log.Error("scan message failed", zap.Int64("chat_id", chatID), zap.Error(err))
			return
		}
		var m Message
		if err := json.Unmarshal(data, &m); err != nil {
			s.log.Error("unmarshal message failed", zap.Int64("chat_id", chatID), zap.Error(err))
			continue
		}
		disk = append(disk, m)
	}
	if err := rows.Err(); err != nil {
		s.log.Error("iterate messages failed", zap.Int64("chat_id", chatID), zap.Error(err))
		return
	}

	if mem := s.messages[chatID]; len(mem) == 0 {
		s.messages[chatID] = disk
	} else {
		s.messages[chatID] = mergeMessagesByID(disk, mem)
	}
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
		s.markMsgDeletedLocked(chatID, m.ID)
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
	s.markMsgDirtyLocked(msg.ChatID, msg.ID)
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
			s.markMsgDeletedLocked(chatID, oldID)
			s.markMsgDirtyLocked(chatID, newID)
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
			s.markMsgDirtyLocked(chatID, msgID)
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
			s.markMsgDirtyLocked(chatID, msgID)
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
			s.markMsgDirtyLocked(chatID, msgID)
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
			s.markMsgDirtyLocked(chatID, msgID)
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
			s.markMsgDeletedLocked(chatID, msgID)
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
			s.markMsgDeletedLocked(chatID, m.ID)
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
