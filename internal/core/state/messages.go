package state

import "github.com/sorokin-vladimir/tele/internal/store"

// ApplyIncoming records a newly received message. The second result reports
// whether the client needs to hear about it at all; Change.UnreadChanged
// separately reports whether a counter moved.
func (s *State) ApplyIncoming(msg store.Message) (Change, bool) {
	unreadChanged := store.ApplyIncomingMessage(s.st, msg)
	c := Change{
		Kind:          ChangeNewMessage,
		ChatID:        msg.ChatID,
		Message:       msg,
		MsgID:         msg.ID,
		UnreadChanged: unreadChanged,
	}
	s.commit(c)
	return c, true
}

// ApplyEdit records a message edited on another client.
//
// Every edit update carries the message's whole current reaction set, so the
// reactions are applied either way. What EditDate decides is only whether the
// text changed too:
//
// A nil EditDate means the converter dropped it as a hidden edit (Telegram
// edit_hide), for example a reaction bump. That is not a content edit and must
// not flip the message to "edited" (#118) — but in 1:1 chats an incoming
// reaction is delivered ONLY as this hidden edit, carrying the message's new
// reactions rather than a separate UpdateMessageReactions (#160). So a hidden
// edit is applied as, and reported as, a reactions change.
//
// A non-nil EditDate is a real content edit — or a reaction on a message that
// was genuinely edited earlier, where edit_date still carries the original edit
// time and edit_hide is false because the "edited" label should keep showing.
// Text and reactions are not alternatives, and treating them as such dropped
// those reactions until the chat was reopened (#199).
func (s *State) ApplyEdit(msg store.Message) (Change, bool) {
	if msg.EditDate == nil {
		return s.ApplyReactions(msg.ChatID, msg.ID, msg.Reactions, msg.HasUnreadReactions)
	}
	s.st.UpdateMessageText(msg.ChatID, msg.ID, msg.Text, msg.Entities, *msg.EditDate)
	s.st.UpdateMessageReactions(msg.ChatID, msg.ID, msg.Reactions)
	unreadChanged := false
	if msg.HasUnreadReactions {
		unreadChanged = s.st.ApplyUnreadReaction(msg.ChatID, msg.ID, true)
	}
	c := Change{
		Kind:                  ChangeMessageEdited,
		ChatID:                msg.ChatID,
		Message:               msg,
		MsgID:                 msg.ID,
		ReactionsUnread:       msg.HasUnreadReactions,
		UnreadReactionChanged: unreadChanged,
	}
	s.commit(c)
	return c, true
}

// ApplyReactions records the current reaction set for a message and tracks
// whether the chat's unread-reaction count moved.
func (s *State) ApplyReactions(chatID int64, msgID int, r []store.Reaction, unread bool) (Change, bool) {
	s.st.UpdateMessageReactions(chatID, msgID, r)
	changed := false
	if unread {
		changed = s.st.ApplyUnreadReaction(chatID, msgID, true)
	}
	c := Change{
		Kind:                  ChangeMessageReactions,
		ChatID:                chatID,
		MsgID:                 msgID,
		ReactionsUnread:       unread,
		UnreadReactionChanged: changed,
	}
	s.commit(c)
	return c, true
}

// ApplyDelete removes messages. A zero chatID means a non-channel delete with
// no peer context: the store resolves each ID to its owning chat through its
// index rather than scanning every chat (#72).
func (s *State) ApplyDelete(chatID int64, msgIDs []int) (Change, bool) {
	if chatID != 0 {
		s.st.RemoveMessages(chatID, msgIDs)
	} else {
		s.st.RemoveMessagesByID(msgIDs)
	}
	c := Change{
		Kind:   ChangeMessagesDeleted,
		ChatID: chatID,
		MsgIDs: msgIDs,
	}
	s.commit(c)
	return c, true
}
