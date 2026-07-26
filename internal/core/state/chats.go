package state

import "github.com/sorokin-vladimir/tele/internal/store"

// ApplyReadInbox advances a chat's inbound read pointer. Reports false when the
// pointer did not move, so nothing downstream is recomputed for a duplicate or
// stale read receipt.
func (s *State) ApplyReadInbox(chatID int64, maxID int) (Change, bool) {
	if !s.st.UpdateChatReadMaxID(chatID, maxID) {
		return Change{}, false
	}
	c := Change{Kind: ChangeReadInbox, ChatID: chatID}
	s.commit(c)
	return c, true
}

// ApplyReadOutbox advances the peer's read pointer over our own messages. The
// chat is read first because UpdateChatOutboxReadMaxID returns nothing; the
// pre-read reproduces its internal guard so this operation can report whether
// anything moved, matching its siblings.
func (s *State) ApplyReadOutbox(chatID int64, maxID int) (Change, bool) {
	before, ok := s.st.GetChat(chatID)
	if !ok || maxID <= before.ReadOutboxMaxID {
		return Change{}, false
	}
	s.st.UpdateChatOutboxReadMaxID(chatID, maxID)
	c := Change{Kind: ChangeReadOutbox, ChatID: chatID}
	s.commit(c)
	return c, true
}

// ApplyPresence records a contact's online state. Reports false when the state
// did not flip: presence updates stream continuously for every online contact,
// and an unchanged one must cost nothing downstream.
func (s *State) ApplyPresence(userID int64, online bool) (Change, bool) {
	if !s.st.UpdateChatOnline(userID, online) {
		return Change{}, false
	}
	c := Change{Kind: ChangePresence, ChatID: userID, Online: online}
	s.commit(c)
	return c, true
}

// ApplyMute records a mute toggled on another device. Reports false for an
// unknown chat or an unchanged flag.
func (s *State) ApplyMute(chatID int64, muted bool) (Change, bool) {
	chat, ok := s.st.GetChat(chatID)
	if !ok || chat.IsMuted == muted {
		return Change{}, false
	}
	s.st.SetChatMuted(chatID, muted)
	c := Change{Kind: ChangeMute, ChatID: chatID, Muted: muted}
	s.commit(c)
	return c, true
}

// ApplyDraft records a draft changed on another device, or cleared server-side
// on send (#62).
func (s *State) ApplyDraft(chatID int64, text string) (Change, bool) {
	s.st.SetChatDraft(chatID, text)
	c := Change{Kind: ChangeDraft, ChatID: chatID, Draft: text}
	s.commit(c)
	return c, true
}

// ApplyTyping publishes a typing indicator. It has no persisted state: the
// typing label is ephemeral and belongs to the chat view, not to the account.
func (s *State) ApplyTyping(chatID int64, action store.TypingAction) (Change, bool) {
	c := Change{Kind: ChangeTyping, ChatID: chatID, Typing: action}
	s.commit(c)
	return c, true
}
