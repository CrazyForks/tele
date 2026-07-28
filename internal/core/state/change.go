package state

import "github.com/sorokin-vladimir/tele/internal/domain"

// ChangeKind names what happened to domain state. It is deliberately not
// store.EventKind: an event means "Telegram told us X", a Change means "state
// now differs in way Y". They diverge — an incoming message already covered by
// the read pointer is an event that changes no counter.
type ChangeKind int

const (
	ChangeNewMessage ChangeKind = iota
	ChangeReadInbox
	ChangeReadOutbox
	ChangeMessageEdited
	// ChangeMessageReactions covers both the explicit reactions update and the
	// hidden edit Telegram uses to deliver 1:1 peer reactions. They were two
	// branches with identical handling; the distinction was never meaningful to
	// a client.
	ChangeMessageReactions
	ChangeDraft
	ChangeMessagesDeleted
	ChangePresence
	ChangeMute
	// ChangeTyping is ephemeral: it has no persisted state and is published
	// straight through. In the projection model the typing label belongs to
	// chat:<id> alongside the message window and the draft.
	ChangeTyping
	// ChangeHistory reports that a chat's stored history was replaced by a
	// fetched page. It is how a backfill re-enters state so the chat:<id>
	// projection rebuilds from the same place every other change does.
	ChangeHistory
)

// Change describes one applied difference in domain state, carrying what a
// client needs to react to it.
type Change struct {
	Kind    ChangeKind
	ChatID  int64
	Message domain.Message
	MsgID   int
	MsgIDs  []int
	Draft   string
	Online  bool
	Muted   bool
	Typing  domain.TypingAction
	// UnreadChanged reports that this change moved a chat's unread or mention
	// count, so views derived from unread (the folder bar) need recomputing.
	UnreadChanged bool
	// ReactionsUnread reports that the reactions carry at least one unread
	// entry; UnreadReactionChanged reports that the per-chat unread-reaction
	// count actually moved as a result.
	ReactionsUnread       bool
	UnreadReactionChanged bool
}
