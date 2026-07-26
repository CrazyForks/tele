package core

import (
	"time"

	"github.com/sorokin-vladimir/tele/internal/store"
)

// Notifier sends OS desktop notifications.
type Notifier interface {
	Notify(title, body string) error
}

func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

// shouldNotify decides whether evt warrants a desktop notification. Pure and
// clock-injected so the freshness rule is unit-testable. now is the reference
// time the event age is measured against (time.Now() in production).
func shouldNotify(st store.Store, evt store.Event, currentChatID int64, now time.Time) bool {
	switch evt.Kind {
	case store.EventNewMessage:
		if evt.Message.IsOut {
			return false
		}
		return store.Notifiable(st, evt.Message.ChatID, currentChatID, evt.Message.Date, now)
	case store.EventReactionsUpdate:
		// A peer reacted to one of our messages in a group/channel.
		if !evt.ReactionsUnread {
			return false
		}
		return store.Notifiable(st, evt.ChatID, currentChatID, evt.ReactionDate, now)
	case store.EventEditMessage:
		// 1:1 peer reactions arrive as a hidden edit; a real text edit carries no
		// unread reactions and must not notify.
		if !evt.Message.HasUnreadReactions {
			return false
		}
		return store.Notifiable(st, evt.Message.ChatID, currentChatID, evt.ReactionDate, now)
	}
	return false
}

func maybeNotify(notifier Notifier, st store.Store, evt store.Event, currentChatID int64, preview bool) {
	if !shouldNotify(st, evt, currentChatID, time.Now()) {
		return
	}
	switch evt.Kind {
	case store.EventNewMessage:
		chat, _ := st.GetChat(evt.Message.ChatID) // shouldNotify guarantees the chat exists.
		body := truncate(evt.Message.Text, 100)
		if !preview {
			// Privacy: some platforms persist notification bodies (macOS
			// Notification Center, systemd journal), so omit the text (#80).
			body = "New message"
		}
		_ = notifier.Notify(chat.Title, body)
	case store.EventReactionsUpdate:
		chat, _ := st.GetChat(evt.ChatID)
		_ = notifier.Notify(chat.Title, reactionBody(evt.ReactionEmoji))
	case store.EventEditMessage:
		chat, _ := st.GetChat(evt.Message.ChatID)
		_ = notifier.Notify(chat.Title, reactionBody(evt.ReactionEmoji))
	}
}

// reactionBody renders the notification body for a reaction, including the emoji
// when one is available (custom-emoji reactions carry none).
func reactionBody(emoji string) string {
	if emoji == "" {
		return "reacted to your message"
	}
	return "reacted " + emoji + " to your message"
}
