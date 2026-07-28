package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/config"
	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/core/state"
	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/store"
)

type nopNotifier struct{}

func (nopNotifier) Notify(string, string) error { return nil }

// newTestOwner builds an owner over an in-memory store with no Telegram client:
// the update loop is fed directly through the returned channel.
func newTestOwner(t *testing.T) (*Owner, chan store.Event, store.Store) {
	t.Helper()
	st := store.NewMemory()
	s := state.New(st)
	events := make(chan store.Event, 8)
	o := New(&config.Config{}, zap.NewNop(), s, nil, nopNotifier{})
	o.events = events
	return o, events, st
}

func recvDelta(t *testing.T, ch <-chan project.Delta) (project.Delta, bool) {
	t.Helper()
	select {
	case d := <-ch:
		return d, true
	case <-time.After(time.Second):
		return project.Delta{}, false
	}
}

func TestUpdateLoop_AppliesAndPublishes(t *testing.T) {
	o, events, st := newTestOwner(t)
	st.SetChat(domain.Chat{ID: 1, Title: "A"})
	o.Subscribe(project.ChatListWindow{Limit: 10})
	_, _ = recvDelta(t, o.Deltas()) // the subscription's initial Reset
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go o.RunUpdates(ctx)

	events <- store.Event{Kind: store.EventNewMessage, Message: domain.Message{ID: 5, ChatID: 1, Text: "hi"}}

	d, got := recvDelta(t, o.Deltas())
	require.True(t, got, "the owner must publish a delta")
	require.NotNil(t, d.ChatList)
	assert.Equal(t, 1, d.ChatList.Row.Unread)

	c, _ := st.GetChat(1)
	assert.Equal(t, 1, c.UnreadCount, "the owner applied the mutation, not the client")
}

// A read receipt that does not advance the pointer changes nothing, so no client
// is woken.
func TestUpdateLoop_NoopEventPublishesNothing(t *testing.T) {
	o, events, st := newTestOwner(t)
	st.SetChat(domain.Chat{ID: 1, ReadInboxMaxID: 10, UnreadCount: 3})
	o.Subscribe(project.ChatListWindow{Limit: 10})
	_, _ = recvDelta(t, o.Deltas())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go o.RunUpdates(ctx)

	events <- store.Event{Kind: store.EventReadInbox, ChatID: 1, ReadMaxID: 10}
	// Follow it with an event that does produce a change, so the assertion does
	// not merely observe a slow loop.
	events <- store.Event{Kind: store.EventReadInbox, ChatID: 1, ReadMaxID: 20}

	d, got := recvDelta(t, o.Deltas())
	require.True(t, got)
	require.NotNil(t, d.ChatList)
	assert.Equal(t, project.ChatListRow, d.ChatList.Kind)
	assert.Zero(t, d.ChatList.Row.Unread, "the advancing receipt cleared the count")

	_, more := recvDelta(t, o.Deltas())
	assert.False(t, more, "the no-op receipt must not have produced a delta")
}

// A client that subscribes to a window nowhere near a change hears nothing. This
// is the presence-storm fix: presence streams for every online contact.
func TestUpdateLoop_ChangeOutsideEveryWindowPublishesNothing(t *testing.T) {
	o, events, st := newTestOwner(t)
	// Distinct last-message times: chats with equal sort keys come out of the
	// store in map order, which would make "is chat 3 in the window" a coin toss.
	for i := 1; i <= 30; i++ {
		st.SetChat(domain.Chat{
			ID:          int64(i),
			Title:       "chat",
			Peer:        domain.Peer{ID: int64(i)},
			LastMessage: &domain.Message{ID: i, Date: time.Unix(int64(1000-i), 0)},
		})
	}
	o.Subscribe(project.ChatListWindow{Offset: 0, Limit: 5})
	_, _ = recvDelta(t, o.Deltas())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go o.RunUpdates(ctx)

	events <- store.Event{Kind: store.EventUserPresence, ChatID: 25, Online: true}
	events <- store.Event{Kind: store.EventUserPresence, ChatID: 3, Online: true}

	d, got := recvDelta(t, o.Deltas())
	require.True(t, got)
	require.NotNil(t, d.ChatList)
	assert.Equal(t, int64(3), d.ChatList.Row.ID, "only the in-window chat produced a delta")

	_, more := recvDelta(t, o.Deltas())
	assert.False(t, more)
}

func TestUpdateLoop_StopsOnContextCancel(t *testing.T) {
	o, _, _ := newTestOwner(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { o.RunUpdates(ctx); close(done) }()

	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("update loop did not stop on context cancellation")
	}
}
