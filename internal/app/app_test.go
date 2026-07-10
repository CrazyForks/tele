package app

import (
	"testing"
	"time"

	"github.com/sorokin-vladimir/tele/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockNotifier struct {
	calls []struct{ title, body string }
}

func (m *mockNotifier) Notify(title, body string) error {
	m.calls = append(m.calls, struct{ title, body string }{title, body})
	return nil
}

func TestMaybeNotify_SendsForOtherChat(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "Bob"})
	evt := store.Event{
		Kind:    store.EventNewMessage,
		Message: store.Message{ChatID: 2, Text: "hello there", Date: time.Now()},
	}
	maybeNotify(n, st, evt, 1, true)
	require.Len(t, n.calls, 1)
	assert.Equal(t, "Bob", n.calls[0].title)
	assert.Equal(t, "hello there", n.calls[0].body)
}

func TestMaybeNotify_SilentForOpenChat(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 1, Title: "Alice"})
	evt := store.Event{
		Kind:    store.EventNewMessage,
		Message: store.Message{ChatID: 1, Text: "hey"},
	}
	maybeNotify(n, st, evt, 1, true)
	assert.Empty(t, n.calls)
}

func TestMaybeNotify_PreviewDisabled_HidesText(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "Bob"})
	evt := store.Event{
		Kind:    store.EventNewMessage,
		Message: store.Message{ChatID: 2, Text: "secret text", Date: time.Now()},
	}
	maybeNotify(n, st, evt, 1, false)
	require.Len(t, n.calls, 1)
	assert.Equal(t, "Bob", n.calls[0].title)
	assert.Equal(t, "New message", n.calls[0].body)
}

func TestMaybeNotify_TruncatesLongText(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "C"})
	b := make([]byte, 200)
	for i := range b {
		b[i] = 'x'
	}
	evt := store.Event{
		Kind:    store.EventNewMessage,
		Message: store.Message{ChatID: 2, Text: string(b), Date: time.Now()},
	}
	maybeNotify(n, st, evt, 0, true)
	require.Len(t, n.calls, 1)
	body := n.calls[0].body
	runes := []rune(body)
	assert.Equal(t, 101, len(runes))          // 100 chars + U+2026 ellipsis
	assert.Equal(t, "…", string(runes[100:])) // last rune is ellipsis
}

func TestMaybeNotify_IgnoresNonMessageEvents(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	evt := store.Event{Kind: store.EventKind(99)}
	maybeNotify(n, st, evt, 0, true)
	assert.Empty(t, n.calls)
}

func TestMaybeNotify_SilentForOutgoingMessage(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "Bob"})
	evt := store.Event{
		Kind:    store.EventNewMessage,
		Message: store.Message{ChatID: 2, Text: "sent from phone", IsOut: true},
	}
	maybeNotify(n, st, evt, 1, true)
	assert.Empty(t, n.calls)
}

func TestMaybeNotify_SilentForMutedChat(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "Bob", IsMuted: true})
	evt := store.Event{
		Kind:    store.EventNewMessage,
		Message: store.Message{ChatID: 2, Text: "hello there"},
	}
	maybeNotify(n, st, evt, 1, true)
	assert.Empty(t, n.calls)
}

func TestMaybeNotify_SilentForStaleCatchUp(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "Bob"})
	// A backlog message recovered via getDifference carries its original
	// (old) send time. It must not raise a notification (#123).
	evt := store.Event{
		Kind:    store.EventNewMessage,
		Message: store.Message{ChatID: 2, Text: "missed while idle", Date: time.Now().Add(-time.Minute)},
	}
	maybeNotify(n, st, evt, 1, true)
	assert.Empty(t, n.calls)
}

func TestMaybeNotify_SendsForFreshMessage(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "Bob"})
	evt := store.Event{
		Kind:    store.EventNewMessage,
		Message: store.Message{ChatID: 2, Text: "live now", Date: time.Now()},
	}
	maybeNotify(n, st, evt, 1, true)
	require.Len(t, n.calls, 1)
	assert.Equal(t, "Bob", n.calls[0].title)
}

func TestMaybeNotify_SendsForFreshReaction_Group(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "Bob"})
	evt := store.Event{
		Kind:            store.EventReactionsUpdate,
		ChatID:          2,
		MsgID:           10,
		ReactionsUnread: true,
		ReactionEmoji:   "❤",
		ReactionDate:    time.Now(),
	}
	maybeNotify(n, st, evt, 1, true)
	require.Len(t, n.calls, 1)
	assert.Equal(t, "Bob", n.calls[0].title)
	assert.Contains(t, n.calls[0].body, "❤")
	assert.Contains(t, n.calls[0].body, "reacted")
}

func TestMaybeNotify_SendsForFreshReaction_DM(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "Bob"})
	// 1:1 reactions arrive as a hidden edit carrying the reacted (outgoing) message.
	evt := store.Event{
		Kind: store.EventEditMessage,
		Message: store.Message{
			ChatID: 2, ID: 10, IsOut: true, HasUnreadReactions: true,
		},
		ReactionEmoji: "👍",
		ReactionDate:  time.Now(),
	}
	maybeNotify(n, st, evt, 1, true)
	require.Len(t, n.calls, 1)
	assert.Equal(t, "Bob", n.calls[0].title)
	assert.Contains(t, n.calls[0].body, "👍")
}

func TestMaybeNotify_SilentForReactionInOpenChat(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 1, Title: "Alice"})
	evt := store.Event{
		Kind: store.EventReactionsUpdate, ChatID: 1, ReactionsUnread: true,
		ReactionEmoji: "❤", ReactionDate: time.Now(),
	}
	maybeNotify(n, st, evt, 1, true)
	assert.Empty(t, n.calls)
}

func TestMaybeNotify_SilentForReactionInMutedChat(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "Bob", IsMuted: true})
	evt := store.Event{
		Kind: store.EventReactionsUpdate, ChatID: 2, ReactionsUnread: true,
		ReactionEmoji: "❤", ReactionDate: time.Now(),
	}
	maybeNotify(n, st, evt, 1, true)
	assert.Empty(t, n.calls)
}

func TestMaybeNotify_SilentForStaleReaction(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "Bob"})
	// A reaction recovered via getDifference carries its original (old) date.
	evt := store.Event{
		Kind: store.EventReactionsUpdate, ChatID: 2, ReactionsUnread: true,
		ReactionEmoji: "❤", ReactionDate: time.Now().Add(-time.Minute),
	}
	maybeNotify(n, st, evt, 1, true)
	assert.Empty(t, n.calls)
}

func TestMaybeNotify_SilentForRealEdit(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "Bob"})
	edited := time.Now()
	// A genuine text edit (no unread reactions) must not notify.
	evt := store.Event{
		Kind: store.EventEditMessage,
		Message: store.Message{
			ChatID: 2, ID: 10, Text: "edited text", EditDate: &edited,
		},
	}
	maybeNotify(n, st, evt, 1, true)
	assert.Empty(t, n.calls)
}

func TestMaybeNotify_SilentForArchivedChat(t *testing.T) {
	n := &mockNotifier{}
	st := store.NewMemory()
	st.SetChat(store.Chat{ID: 2, Title: "Bob", IsArchived: true})
	evt := store.Event{
		Kind:    store.EventNewMessage,
		Message: store.Message{ChatID: 2, Text: "hello there"},
	}
	maybeNotify(n, st, evt, 1, true)
	assert.Empty(t, n.calls)
}
