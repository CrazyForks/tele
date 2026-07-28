package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/store"
)

func TestApplyUnreadMention_Idempotent(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1, Title: "A"})

	assert.True(t, s.ApplyUnreadMention(1, 100, true))
	c, _ := s.GetChat(1)
	assert.Equal(t, 1, c.UnreadMentionsCount)

	// Same message again: already tracked, no change.
	assert.False(t, s.ApplyUnreadMention(1, 100, true))
	c, _ = s.GetChat(1)
	assert.Equal(t, 1, c.UnreadMentionsCount)

	// Different message: +1.
	assert.True(t, s.ApplyUnreadMention(1, 200, true))
	c, _ = s.GetChat(1)
	assert.Equal(t, 2, c.UnreadMentionsCount)
}

func TestApplyUnreadMention_ClearDecrementsFloorZero(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1})
	s.ApplyUnreadMention(1, 100, true)

	assert.True(t, s.ApplyUnreadMention(1, 100, false))
	c, _ := s.GetChat(1)
	assert.Equal(t, 0, c.UnreadMentionsCount)

	// Clearing an untracked message: no change, no negative count.
	assert.False(t, s.ApplyUnreadMention(1, 999, false))
	c, _ = s.GetChat(1)
	assert.Equal(t, 0, c.UnreadMentionsCount)
}

func TestApplyUnreadMention_UnknownChatNoop(t *testing.T) {
	s := store.NewMemory()
	assert.False(t, s.ApplyUnreadMention(42, 1, true))
}

func TestSetChatMentionsRead_ClearsCountAndSet(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1, UnreadMentionsCount: 3})
	s.ApplyUnreadMention(1, 100, true) // count 4, tracked {100}

	s.SetChatMentionsRead(1)
	c, _ := s.GetChat(1)
	assert.Equal(t, 0, c.UnreadMentionsCount)

	// Set was cleared: re-adding the same message increments from 0.
	assert.True(t, s.ApplyUnreadMention(1, 100, true))
	c, _ = s.GetChat(1)
	assert.Equal(t, 1, c.UnreadMentionsCount)
}

func TestSetChat_ResetsMentionTracking(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1})
	s.ApplyUnreadMention(1, 100, true)

	// Dialog refresh overwrites the chat with a fresh server count.
	s.SetChat(domain.Chat{ID: 1, UnreadMentionsCount: 5})

	// Tracked set was cleared by SetChat, so re-adding 100 increments to 6.
	assert.True(t, s.ApplyUnreadMention(1, 100, true))
	c, _ := s.GetChat(1)
	assert.Equal(t, 6, c.UnreadMentionsCount)
}
