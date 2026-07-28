package core

import (
	"time"

	"github.com/sorokin-vladimir/tele/internal/store"
)

// Incoming reports that a message arrived in a chat the client does not have
// open. It is an event, not state, which is why it does not ride on a
// projection: a chat-list row says what the unread count is now, never that
// something just happened — and a row flash and a toast are reactions to the
// happening.
//
// The owner decides whether it warrants a notification, so a client renders
// rather than judges. #192 folds the OS notification and the in-app toast into
// that one decision; this is where it already lives.
type Incoming struct {
	ChatID  int64
	Title   string
	Preview string
	Notify  bool
}

// Incoming is the event stream a client renders alongside its projections.
func (o *Owner) Incoming() <-chan Incoming { return o.incoming }

// publishIncoming reports an inbound message in a chat the client is not
// showing. Outgoing messages and the open chat are excluded: neither should
// flash a row or raise a toast.
func (o *Owner) publishIncoming(evt store.Event, currentChatID int64, now time.Time) {
	if evt.Kind != store.EventNewMessage || evt.Message.IsOut {
		return
	}
	if evt.Message.ChatID == currentChatID {
		return
	}
	chat, ok := o.state.Store().GetChat(evt.Message.ChatID)
	if !ok {
		return
	}
	preview := truncate(evt.Message.Text, 100)
	if !o.cfg.UI.NotificationPreview {
		// Privacy: the same rule the OS notification follows (#80).
		preview = "New message"
	}
	in := Incoming{
		ChatID:  evt.Message.ChatID,
		Title:   chat.Title,
		Preview: preview,
		Notify:  shouldNotify(o.state.Store(), evt, currentChatID, now),
	}
	select {
	case o.incoming <- in:
	default:
		o.log.Warn("incoming event dropped: client is not draining")
	}
}
