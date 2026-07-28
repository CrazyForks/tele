package project_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/core/project"
	"github.com/sorokin-vladimir/tele/internal/domain"
)

func TestRegistry_SubscribeRepliesWithCurrentContents(t *testing.T) {
	r := &fakeReader{chats: chats(3)}
	g := project.NewRegistry(r)

	id, deltas := g.Subscribe(project.ChatListWindow{Limit: 10})

	require.NotZero(t, id)
	require.Len(t, deltas, 1)
	require.NotNil(t, deltas[0].ChatList)
	assert.Equal(t, project.ChatListReset, deltas[0].ChatList.Kind,
		"the first delta is a Reset, which is what makes resubscribing a resync")
	assert.Equal(t, id, deltas[0].Sub)
}

func TestRegistry_ResubscribeIsResync(t *testing.T) {
	r := &fakeReader{chats: chats(3)}
	g := project.NewRegistry(r)
	id1, _ := g.Subscribe(project.ChatListWindow{Limit: 10})
	g.Unsubscribe(id1)
	r.chats = chats(5)

	_, deltas := g.Subscribe(project.ChatListWindow{Limit: 10})

	require.Len(t, deltas, 1)
	assert.Len(t, deltas[0].ChatList.Rows, 5)
}

func TestRegistry_RefreshEmitsNothingWhenNothingChanged(t *testing.T) {
	r := &fakeReader{chats: chats(3)}
	g := project.NewRegistry(r)
	g.Subscribe(project.ChatListWindow{Limit: 10})

	assert.Empty(t, g.Refresh())
}

func TestRegistry_RefreshEmitsNothingForAChangeOutsideTheWindow(t *testing.T) {
	all := chats(50)
	r := &fakeReader{chats: all}
	g := project.NewRegistry(r)
	g.Subscribe(project.ChatListWindow{Offset: 0, Limit: 10})

	all[40].Online = true // a presence update for a chat nowhere near the screen
	r.chats = all

	assert.Empty(t, g.Refresh(),
		"presence streams continuously for every online contact; only the window pays")
}

func TestRegistry_RefreshEmitsARowForAnInWindowChange(t *testing.T) {
	all := chats(10)
	r := &fakeReader{chats: all}
	g := project.NewRegistry(r)
	id, _ := g.Subscribe(project.ChatListWindow{Offset: 0, Limit: 5})

	all[2].Online = true
	r.chats = all

	deltas := g.Refresh()

	require.Len(t, deltas, 1)
	assert.Equal(t, id, deltas[0].Sub)
	assert.Equal(t, project.ChatListRow, deltas[0].ChatList.Kind)
	assert.Equal(t, int64(3), deltas[0].ChatList.Row.ID)
}

func TestRegistry_RefreshFansOutToEverySubscription(t *testing.T) {
	all := chats(10)
	r := &fakeReader{chats: all, msgs: map[int64][]domain.Message{1: msgs(3)}}
	g := project.NewRegistry(r)
	listID, _ := g.Subscribe(project.ChatListWindow{Limit: 10})
	chatID, _ := g.Subscribe(project.ChatWindow{
		ChatID: 1, Anchor: project.Anchor{Kind: project.AnchorNewest}, Before: 10,
	})

	all[0].Title = "renamed"
	r.chats = all
	r.msgs[1] = msgs(4)

	deltas := g.Refresh()

	var sawList, sawChat bool
	for _, d := range deltas {
		if d.Sub == listID && d.ChatList != nil {
			sawList = true
		}
		if d.Sub == chatID && d.Chat != nil {
			sawChat = true
		}
	}
	assert.True(t, sawList)
	assert.True(t, sawChat)
}

func TestRegistry_MoveWindowReplacesTheWindowAndEmits(t *testing.T) {
	r := &fakeReader{chats: chats(50)}
	g := project.NewRegistry(r)
	id, _ := g.Subscribe(project.ChatListWindow{Offset: 0, Limit: 5})

	deltas := g.MoveWindow(id, project.ChatListWindow{Offset: 20, Limit: 5})

	require.Len(t, deltas, 1)
	require.Equal(t, project.ChatListReset, deltas[0].ChatList.Kind)
	assert.Equal(t, 20, deltas[0].ChatList.Offset)
	assert.Equal(t, int64(21), deltas[0].ChatList.Rows[0].ID)
}

func TestRegistry_MoveWindowToTheSameWindowEmitsNothing(t *testing.T) {
	r := &fakeReader{chats: chats(50)}
	g := project.NewRegistry(r)
	w := project.ChatListWindow{Offset: 0, Limit: 5}
	id, _ := g.Subscribe(w)

	assert.Empty(t, g.MoveWindow(id, w), "MoveWindow is idempotent")
}

func TestRegistry_UnsubscribedSubscriptionStopsEmitting(t *testing.T) {
	all := chats(10)
	r := &fakeReader{chats: all}
	g := project.NewRegistry(r)
	id, _ := g.Subscribe(project.ChatListWindow{Limit: 10})
	g.Unsubscribe(id)

	all[0].Online = true
	r.chats = all

	assert.Empty(t, g.Refresh())
}

func TestRegistry_MoveWindowOnAnUnknownSubscriptionIsANoOp(t *testing.T) {
	g := project.NewRegistry(&fakeReader{chats: chats(1)})

	assert.Empty(t, g.MoveWindow(project.SubID(99), project.ChatListWindow{Limit: 5}))
}

func TestRegistry_WindowReportsTheCurrentWindow(t *testing.T) {
	g := project.NewRegistry(&fakeReader{chats: chats(3)})
	id, _ := g.Subscribe(project.ChatListWindow{Offset: 0, Limit: 5})
	g.MoveWindow(id, project.ChatListWindow{Offset: 1, Limit: 5})

	w, ok := g.Window(id)

	require.True(t, ok)
	assert.Equal(t, project.ChatListWindow{Offset: 1, Limit: 5}, w)
}

func TestRegistry_TypingReachesOnlyTheSubscribedChat(t *testing.T) {
	r := &fakeReader{
		chats: chats(2),
		msgs:  map[int64][]domain.Message{1: msgs(2), 2: msgs(2)},
	}
	g := project.NewRegistry(r)
	sub1, _ := g.Subscribe(project.ChatWindow{
		ChatID: 1, Anchor: project.Anchor{Kind: project.AnchorNewest}, Before: 5,
	})
	g.Subscribe(project.ChatWindow{
		ChatID: 2, Anchor: project.Anchor{Kind: project.AnchorNewest}, Before: 5,
	})

	deltas := g.SetTyping(1, "Ada is typing")

	require.Len(t, deltas, 1)
	assert.Equal(t, sub1, deltas[0].Sub)
	assert.Equal(t, project.ChatTyping, deltas[0].Chat.Kind)
	assert.Equal(t, "Ada is typing", deltas[0].Chat.Typing)
}

func TestRegistry_ClearingTypingEmitsAnEmptyLabel(t *testing.T) {
	r := &fakeReader{chats: chats(1), msgs: map[int64][]domain.Message{1: msgs(2)}}
	g := project.NewRegistry(r)
	g.Subscribe(project.ChatWindow{
		ChatID: 1, Anchor: project.Anchor{Kind: project.AnchorNewest}, Before: 5,
	})
	g.SetTyping(1, "Ada is typing")

	deltas := g.SetTyping(1, "")

	require.Len(t, deltas, 1)
	assert.Equal(t, project.ChatTyping, deltas[0].Chat.Kind)
	assert.Empty(t, deltas[0].Chat.Typing)
}
