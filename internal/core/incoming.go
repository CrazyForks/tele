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

// Failure reports work the owner could not finish on a client's behalf. A client
// cannot see the attempt — filling a window may or may not touch the network —
// so a silent failure would leave a spinner running forever.
//
// Err carries the domain error kind from #191; the client decides how to say it.
type Failure struct {
	ChatID int64
	Op     string
	Err    error
}

// OpSend names a send the queue gave up on. Clients single it out: a refused
// send is not a window that failed to fill, so it belongs in a toast rather
// than in the pane's load-error banner, and the message is still on screen to
// retry (#193).
const OpSend = "send"

// Typing reports that someone started or stopped composing in a chat. An empty
// Label means stopped.
//
// It is an event rather than part of the chat projection, because there is no
// state behind it to be right about: Telegram is not obliged to send a stop, so
// a stored label has nothing to clear it and would outlive the typing forever.
// The client shows it and lets it expire.
type Typing struct {
	ChatID int64
	Label  string
}

// Incoming is the event stream a client renders alongside its projections.
func (o *Owner) Incoming() <-chan Incoming { return o.incoming }

// Typing reports composing activity in the chats a client may be showing.
func (o *Owner) Typing() <-chan Typing { return o.typing }

func (o *Owner) publishTyping(t Typing) {
	select {
	case o.typing <- t:
	default:
		o.log.Warn("typing event dropped: client is not draining")
	}
}

// Progress is how far a queued media send has uploaded.
//
// It is an event rather than part of the chat projection, for the reason typing
// is: there is nothing persisted behind it, a dropped frame is corrected by the
// next one, and the uploader emits one per chunk. Routed through the projection,
// that stream would rebuild and diff every subscription per chunk, and would
// compete for the delta buffer with changes whose loss costs a stale window.
type Progress struct {
	ChatID int64
	Ref    string
	// Part is the 1-based file being uploaded now, Parts the total in the entry.
	Part  int
	Parts int
	// Done and Total are bytes over the whole entry, so a client renders the
	// aggregate without having to know what each part weighs.
	Done  int64
	Total int64
}

// Progress reports upload advance for the queued sends a client may be showing.
func (o *Owner) Progress() <-chan Progress { return o.progress }

// publishProgress drops rather than blocks. Unlike typing it does not log the
// drop: under a large upload that would be thousands of lines about a repaint
// nobody missed.
func (o *Owner) publishProgress(p Progress) {
	select {
	case o.progress <- p:
	default:
	}
}

// Failures reports operations the owner could not complete.
func (o *Owner) Failures() <-chan Failure { return o.failures }

func (o *Owner) publishFailure(f Failure) {
	select {
	case o.failures <- f:
	default:
		o.log.Warn("failure event dropped: client is not draining")
	}
}

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
