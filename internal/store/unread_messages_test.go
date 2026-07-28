package store_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/store"
)

func TestApplyUnreadMessage_CountsFreshInboundID(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1, Title: "A"})

	assert.True(t, s.ApplyUnreadMessage(1, 100))
	c, _ := s.GetChat(1)
	assert.Equal(t, 1, c.UnreadCount)

	assert.True(t, s.ApplyUnreadMessage(1, 200))
	c, _ = s.GetChat(1)
	assert.Equal(t, 2, c.UnreadCount)
}

// A daemon may replay the same update on attach or reconnect (#183); counting
// twice would inflate what a CLI reports.
func TestApplyUnreadMessage_SameIDCountsOnce(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1})
	require.True(t, s.ApplyUnreadMessage(1, 100))

	assert.False(t, s.ApplyUnreadMessage(1, 100))
	c, _ := s.GetChat(1)
	assert.Equal(t, 1, c.UnreadCount)
}

// getDifference catch-up delivers messages already read on another client.
func TestApplyUnreadMessage_AtOrBelowReadPointerIsNoop(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1, ReadInboxMaxID: 100})

	assert.False(t, s.ApplyUnreadMessage(1, 100))
	assert.False(t, s.ApplyUnreadMessage(1, 50))
	c, _ := s.GetChat(1)
	assert.Equal(t, 0, c.UnreadCount)

	assert.True(t, s.ApplyUnreadMessage(1, 101))
	c, _ = s.GetChat(1)
	assert.Equal(t, 1, c.UnreadCount)
}

func TestApplyUnreadMessage_UnknownChatNoop(t *testing.T) {
	s := store.NewMemory()
	assert.False(t, s.ApplyUnreadMessage(42, 1))
}

// SetChat carries the authoritative dialog-list count; session tracking must not
// be added on top of a number that already includes those messages.
func TestApplyUnreadMessage_SetChatRebasesAndDropsTracking(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1})
	require.True(t, s.ApplyUnreadMessage(1, 100))

	s.SetChat(domain.Chat{ID: 1, UnreadCount: 7})
	c, _ := s.GetChat(1)
	assert.Equal(t, 7, c.UnreadCount)

	// The tracked set was dropped, so the same ID counts again on top of 7.
	assert.True(t, s.ApplyUnreadMessage(1, 100))
	c, _ = s.GetChat(1)
	assert.Equal(t, 8, c.UnreadCount)
}

func TestApplyUnreadMessage_AddsOnTopOfServerBaseline(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1, UnreadCount: 3, ReadInboxMaxID: 10})

	require.True(t, s.ApplyUnreadMessage(1, 11))
	require.True(t, s.ApplyUnreadMessage(1, 12))
	c, _ := s.GetChat(1)
	assert.Equal(t, 5, c.UnreadCount)
}

// On restart the persisted count becomes the baseline, so counting resumes from
// it instead of restarting at zero.
func TestApplyUnreadMessage_ResumesFromPersistedCountAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	log := zap.NewNop()

	s, err := store.NewSQLite(path, log)
	require.NoError(t, err)
	s.SetChat(domain.Chat{ID: 1, Title: "A", UnreadCount: 4, ReadInboxMaxID: 10})
	require.NoError(t, s.Close())

	s2, err := store.NewSQLite(path, log)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s2.Close() })

	c, ok := s2.GetChat(1)
	require.True(t, ok)
	require.Equal(t, 4, c.UnreadCount)

	require.True(t, s2.ApplyUnreadMessage(1, 11))
	c, _ = s2.GetChat(1)
	assert.Equal(t, 5, c.UnreadCount)
}

// Regression: messages load lazily, so a chat not opened this session has an
// empty in-memory slice. The old message-scan recompute zeroed its count on any
// read event; the tracked set must survive instead.
func TestUpdateChatReadMaxID_KeepsUnreadAbovePointerWithoutLoadedMessages(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1, ReadInboxMaxID: 10})
	require.True(t, s.ApplyUnreadMessage(1, 11))
	require.True(t, s.ApplyUnreadMessage(1, 12))
	require.True(t, s.ApplyUnreadMessage(1, 13))

	// Read up to 12 on another client; 13 stays unread. No messages were ever
	// appended to this chat.
	require.True(t, s.UpdateChatReadMaxID(1, 12))

	c, _ := s.GetChat(1)
	assert.Equal(t, 12, c.ReadInboxMaxID)
	assert.Equal(t, 1, c.UnreadCount)
}

func TestUpdateChatReadMaxID_ClearsCountWhenPointerPassesEverything(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1, ReadInboxMaxID: 10})
	require.True(t, s.ApplyUnreadMessage(1, 11))
	require.True(t, s.ApplyUnreadMessage(1, 12))

	require.True(t, s.UpdateChatReadMaxID(1, 12))
	c, _ := s.GetChat(1)
	assert.Equal(t, 0, c.UnreadCount)
}

// Pruning must remove the IDs, not just the count: a message re-delivered after
// being read stays out of the count.
func TestUpdateChatReadMaxID_PrunedIDsDoNotCountAgain(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1, ReadInboxMaxID: 10})
	require.True(t, s.ApplyUnreadMessage(1, 11))
	require.True(t, s.UpdateChatReadMaxID(1, 11))

	assert.False(t, s.ApplyUnreadMessage(1, 11))
	c, _ := s.GetChat(1)
	assert.Equal(t, 0, c.UnreadCount)
}

// An advancing pointer makes the dialog-list baseline stale: its messages are
// identified by a number only, and some of them have now been read.
func TestUpdateChatReadMaxID_DropsStaleServerBaseline(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1, UnreadCount: 5, ReadInboxMaxID: 10})
	require.True(t, s.ApplyUnreadMessage(1, 20))
	c, _ := s.GetChat(1)
	require.Equal(t, 6, c.UnreadCount)

	require.True(t, s.UpdateChatReadMaxID(1, 15))

	c, _ = s.GetChat(1)
	assert.Equal(t, 1, c.UnreadCount, "only the locally observed message above 15 survives")
}

func TestUpdateChatReadMaxID_NoAdvanceLeavesCountUntouched(t *testing.T) {
	s := store.NewMemory()
	s.SetChat(domain.Chat{ID: 1, ReadInboxMaxID: 10})
	require.True(t, s.ApplyUnreadMessage(1, 11))

	assert.False(t, s.UpdateChatReadMaxID(1, 10))
	c, _ := s.GetChat(1)
	assert.Equal(t, 1, c.UnreadCount)
	assert.Equal(t, 10, c.ReadInboxMaxID)
}
