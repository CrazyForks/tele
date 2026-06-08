package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/store"
)

func newTestSQLite(t *testing.T) *store.SQLiteStore {
	t.Helper()
	s, err := store.NewSQLite(filepath.Join(t.TempDir(), "state.db"), zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestSQLite_SetChat_PersistsSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	log := zap.NewNop()

	s, err := store.NewSQLite(path, log)
	require.NoError(t, err)
	s.SetChat(store.Chat{
		ID:    42,
		Title: "Hello",
		Peer:  store.Peer{ID: 42, Type: store.PeerUser, AccessHash: 999},
	})
	_ = s.Close()

	s2, err := store.NewSQLite(path, log)
	require.NoError(t, err)
	defer func() { _ = s2.Close() }()

	chat, ok := s2.GetChat(42)
	assert.True(t, ok)
	assert.Equal(t, "Hello", chat.Title)
	assert.Equal(t, int64(999), chat.Peer.AccessHash)
}

func TestSQLite_LastMessage_PersistsSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	log := zap.NewNop()

	now := time.Unix(1700000000, 0).UTC()
	s, err := store.NewSQLite(path, log)
	require.NoError(t, err)
	s.SetChat(store.Chat{
		ID:    1,
		Title: "C",
		Peer:  store.Peer{ID: 1, Type: store.PeerUser},
		LastMessage: &store.Message{
			ID:     55,
			ChatID: 1,
			Text:   "hey",
			Date:   now,
		},
	})
	_ = s.Close()

	s2, err := store.NewSQLite(path, log)
	require.NoError(t, err)
	defer func() { _ = s2.Close() }()

	chat, ok := s2.GetChat(1)
	assert.True(t, ok)
	require.NotNil(t, chat.LastMessage)
	assert.Equal(t, 55, chat.LastMessage.ID)
	assert.Equal(t, "hey", chat.LastMessage.Text)
	assert.True(t, chat.LastMessage.Date.Equal(now))
}

func TestSQLite_FolderFilters_PersistsSurvivesReopen(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	log := zap.NewNop()

	filters := []store.FolderFilter{
		{ID: 1, Title: "Work", Emoji: "💼", Groups: true},
		{ID: 2, Title: "Personal", Contacts: true, ExcludeMuted: true},
	}

	s, err := store.NewSQLite(path, log)
	require.NoError(t, err)
	s.SetFolderFilters(filters)
	_ = s.Close()

	s2, err := store.NewSQLite(path, log)
	require.NoError(t, err)
	defer func() { _ = s2.Close() }()

	got := s2.FolderFilters()
	require.Len(t, got, 2)
	assert.Equal(t, 1, got[0].ID)
	assert.Equal(t, "Work", got[0].Title)
	assert.True(t, got[0].Groups)
	assert.Equal(t, 2, got[1].ID)
	assert.True(t, got[1].Contacts)
	assert.True(t, got[1].ExcludeMuted)
}

func TestSQLite_FolderFilters_EmptyWhenNotSet(t *testing.T) {
	s := newTestSQLite(t)
	assert.Nil(t, s.FolderFilters())
}

func TestSQLite_Chats_OrderMatchesMemory(t *testing.T) {
	s := newTestSQLite(t)
	now := time.Now()
	s.SetChat(store.Chat{ID: 1, Title: "A", LastMessage: &store.Message{Date: now.Add(-1 * time.Minute)}})
	s.SetChat(store.Chat{ID: 2, Title: "B", LastMessage: &store.Message{Date: now}})
	s.SetChat(store.Chat{ID: 3, Title: "Pinned", Pinned: true})

	chats := s.Chats()
	require.Len(t, chats, 3)
	assert.Equal(t, int64(3), chats[0].ID) // pinned first
	assert.Equal(t, int64(2), chats[1].ID) // newest
	assert.Equal(t, int64(1), chats[2].ID)
}

func TestSQLite_Chats_ReordersAfterAppendMessage(t *testing.T) {
	s := newTestSQLite(t)
	now := time.Now()
	s.SetChat(store.Chat{ID: 1, Title: "A", LastMessage: &store.Message{Date: now}})
	s.SetChat(store.Chat{ID: 2, Title: "B", LastMessage: &store.Message{Date: now.Add(-1 * time.Hour)}})

	// A is newest, so it leads initially.
	require.Equal(t, int64(1), s.Chats()[0].ID)

	// A newer message in B must move it to the top on the next read.
	s.AppendMessage(store.Message{ID: 9, ChatID: 2, Date: now.Add(1 * time.Hour)})
	assert.Equal(t, int64(2), s.Chats()[0].ID)
}

func TestSQLite_Chats_ReflectsFreshUnreadAndOnlineWithoutReorder(t *testing.T) {
	s := newTestSQLite(t)
	now := time.Now()
	s.SetChat(store.Chat{ID: 1, Title: "A", LastMessage: &store.Message{Date: now}})
	s.SetChat(store.Chat{ID: 2, Title: "B", LastMessage: &store.Message{Date: now.Add(-1 * time.Hour)}})

	// Prime the order cache.
	require.Equal(t, int64(1), s.Chats()[0].ID)

	// Mutations that do not affect ordering must still be reflected in the
	// cached view (the cache stores order only; field values are read fresh).
	s.IncrementChatUnread(1)
	s.UpdateChatOnline(1, true)

	chats := s.Chats()
	require.Equal(t, int64(1), chats[0].ID) // order unchanged
	assert.Equal(t, 1, chats[0].UnreadCount)
	assert.True(t, chats[0].Online)
}

func TestSQLite_UpdateChatOnline_ReturnsTrueOnFlip(t *testing.T) {
	s := newTestSQLite(t)
	s.SetChat(store.Chat{ID: 1, Title: "Alice"})
	assert.True(t, s.UpdateChatOnline(1, true))
}

func TestSQLite_UpdateChatOnline_ReturnsFalseWhenUnchanged(t *testing.T) {
	s := newTestSQLite(t)
	s.SetChat(store.Chat{ID: 1, Title: "Alice", Online: true})
	assert.False(t, s.UpdateChatOnline(1, true))
}

func TestSQLite_UpdateChatOnline_ReturnsFalseWhenMissing(t *testing.T) {
	s := newTestSQLite(t)
	assert.False(t, s.UpdateChatOnline(999, true))
}

func TestSQLite_UpdateChatReadMaxID_ReturnsTrueWhenAdvanced(t *testing.T) {
	s := newTestSQLite(t)
	s.SetChat(store.Chat{ID: 1, Title: "Alice", ReadInboxMaxID: 5})
	assert.True(t, s.UpdateChatReadMaxID(1, 10))
}

func TestSQLite_UpdateChatReadMaxID_ReturnsFalseWhenNotAdvanced(t *testing.T) {
	s := newTestSQLite(t)
	s.SetChat(store.Chat{ID: 1, Title: "Alice", ReadInboxMaxID: 10})
	assert.False(t, s.UpdateChatReadMaxID(1, 10))
}

func TestSQLite_UpdateChatReadMaxID_ReturnsFalseWhenMissing(t *testing.T) {
	s := newTestSQLite(t)
	assert.False(t, s.UpdateChatReadMaxID(999, 10))
}
