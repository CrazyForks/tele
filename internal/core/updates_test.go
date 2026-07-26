package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/config"
	"github.com/sorokin-vladimir/tele/internal/core/state"
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

func recvChange(t *testing.T, ch <-chan state.Change) (state.Change, bool) {
	t.Helper()
	select {
	case c := <-ch:
		return c, true
	case <-time.After(time.Second):
		return state.Change{}, false
	}
}

func TestUpdateLoop_AppliesAndPublishes(t *testing.T) {
	o, events, st := newTestOwner(t)
	st.SetChat(store.Chat{ID: 1, Title: "A"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go o.RunUpdates(ctx)

	events <- store.Event{Kind: store.EventNewMessage, Message: store.Message{ID: 5, ChatID: 1, Text: "hi"}}

	chg, got := recvChange(t, o.Changes())
	require.True(t, got, "the owner must publish a change")
	assert.Equal(t, state.ChangeNewMessage, chg.Kind)
	assert.Equal(t, int64(1), chg.ChatID)

	c, _ := st.GetChat(1)
	assert.Equal(t, 1, c.UnreadCount, "the owner applied the mutation, not the client")
}

// A read receipt that does not advance the pointer changes nothing, so no client
// is woken.
func TestUpdateLoop_NoopEventPublishesNothing(t *testing.T) {
	o, events, st := newTestOwner(t)
	st.SetChat(store.Chat{ID: 1, ReadInboxMaxID: 10})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go o.RunUpdates(ctx)

	events <- store.Event{Kind: store.EventReadInbox, ChatID: 1, ReadMaxID: 10}
	// Follow it with an event that does produce a change, so the assertion does
	// not merely observe a slow loop.
	events <- store.Event{Kind: store.EventReadInbox, ChatID: 1, ReadMaxID: 20}

	chg, got := recvChange(t, o.Changes())
	require.True(t, got)
	assert.Equal(t, state.ChangeReadInbox, chg.Kind)

	_, more := recvChange(t, o.Changes())
	assert.False(t, more, "the no-op receipt must not have produced a change")
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
