package store_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func openStore(t *testing.T, path string) *store.SQLiteStore {
	t.Helper()
	s, err := store.NewSQLite(path, zap.NewNop())
	require.NoError(t, err)
	return s
}

func TestSQLite_Messages_PersistAndReloadAfterReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")

	s := openStore(t, path)
	s.SetChat(domain.Chat{ID: 7, Peer: domain.Peer{ID: 7, Type: domain.PeerUser}})
	s.SetMessages(7, []domain.Message{
		{ID: 1, ChatID: 7, Text: "hello", Date: time.Unix(1000, 0)},
		{ID: 2, ChatID: 7, Text: "world", Date: time.Unix(2000, 0)},
	})
	require.NoError(t, s.Close()) // Close flushes pending write-behind

	s2 := openStore(t, path)
	defer func() { _ = s2.Close() }()
	s2.LoadMessages(7)
	got := s2.Messages(7)

	require.Len(t, got, 2)
	assert.Equal(t, "hello", got[0].Text)
	assert.Equal(t, "world", got[1].Text)
}

func TestSQLite_AppendMessage_PersistsSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")

	s := openStore(t, path)
	s.SetChat(domain.Chat{ID: 9, Peer: domain.Peer{ID: 9, Type: domain.PeerUser}})
	s.AppendMessage(domain.Message{ID: 5, ChatID: 9, Text: "appended", Date: time.Unix(3000, 0)})
	require.NoError(t, s.Close())

	s2 := openStore(t, path)
	defer func() { _ = s2.Close() }()
	s2.LoadMessages(9)
	got := s2.Messages(9)

	require.Len(t, got, 1)
	assert.Equal(t, "appended", got[0].Text)
}

func TestSQLite_CapTrim_DeletesOldestOnDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")

	s := openStore(t, path)
	s.SetChat(domain.Chat{ID: 3, Peer: domain.Peer{ID: 3, Type: domain.PeerUser}})
	// One past the cap so the oldest (id 1) is trimmed.
	msgs := make([]domain.Message, 0, store.MaxMessagesPerChat+1)
	for i := 1; i <= store.MaxMessagesPerChat+1; i++ {
		msgs = append(msgs, domain.Message{ID: i, ChatID: 3, Date: time.Unix(int64(i), 0)})
	}
	s.SetMessages(3, msgs)
	require.NoError(t, s.Close())

	s2 := openStore(t, path)
	defer func() { _ = s2.Close() }()
	s2.LoadMessages(3)
	got := s2.Messages(3)

	require.Len(t, got, store.MaxMessagesPerChat)
	assert.Equal(t, 2, got[0].ID, "oldest (id 1) should be trimmed and absent on disk")
}

func TestSQLite_MessageEdit_PersistsSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")

	// Seed and persist the original message.
	s := openStore(t, path)
	s.SetChat(domain.Chat{ID: 4, Peer: domain.Peer{ID: 4, Type: domain.PeerUser}})
	s.SetMessages(4, []domain.Message{{ID: 1, ChatID: 4, Text: "before", Date: time.Unix(10, 0)}})
	require.NoError(t, s.Close())

	// Reopen so the message is disk-loaded and clean (not already dirty), then
	// edit it — this only persists if the edit ops mark the message dirty.
	s2 := openStore(t, path)
	s2.SetChat(domain.Chat{ID: 4, Peer: domain.Peer{ID: 4, Type: domain.PeerUser}})
	s2.LoadMessages(4)
	s2.UpdateMessageText(4, 1, "after", nil, time.Unix(20, 0))
	s2.UpdateMessageReactions(4, 1, []domain.Reaction{{Emoji: "👍", Count: 2}})
	require.NoError(t, s2.Close())

	s3 := openStore(t, path)
	defer func() { _ = s3.Close() }()
	s3.LoadMessages(4)
	got := s3.Messages(4)

	require.Len(t, got, 1)
	assert.Equal(t, "after", got[0].Text)
	require.Len(t, got[0].Reactions, 1)
	assert.Equal(t, "👍", got[0].Reactions[0].Emoji)
	require.NotNil(t, got[0].EditDate)
}

func TestSQLite_UpdateMessageID_MovesRowOnDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")

	s := openStore(t, path)
	s.SetChat(domain.Chat{ID: 6, Peer: domain.Peer{ID: 6, Type: domain.PeerUser}})
	// Positive ids only: negative sentinels are never persisted, so exercise the
	// id-change path with two real ids.
	s.SetMessages(6, []domain.Message{{ID: 100, ChatID: 6, Text: "m", Date: time.Unix(1, 0)}})
	s.UpdateMessageID(6, 100, 200)
	require.NoError(t, s.Close())

	s2 := openStore(t, path)
	defer func() { _ = s2.Close() }()
	s2.LoadMessages(6)
	got := s2.Messages(6)

	require.Len(t, got, 1)
	assert.Equal(t, 200, got[0].ID, "old id row should be deleted, new id upserted")
}

func TestSQLite_RemoveMessage_DeletesOnDisk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")

	// Seed two messages and persist them.
	s := openStore(t, path)
	s.SetChat(domain.Chat{ID: 8, Peer: domain.Peer{ID: 8, Type: domain.PeerUser}})
	s.SetMessages(8, []domain.Message{
		{ID: 1, ChatID: 8, Text: "keep", Date: time.Unix(1, 0)},
		{ID: 2, ChatID: 8, Text: "drop", Date: time.Unix(2, 0)},
	})
	require.NoError(t, s.Close())

	// Reopen so both are disk-loaded and clean, then remove one.
	s2 := openStore(t, path)
	s2.SetChat(domain.Chat{ID: 8, Peer: domain.Peer{ID: 8, Type: domain.PeerUser}})
	s2.LoadMessages(8)
	s2.RemoveMessage(8, 2)
	require.NoError(t, s2.Close())

	s3 := openStore(t, path)
	defer func() { _ = s3.Close() }()
	s3.LoadMessages(8)
	got := s3.Messages(8)

	require.Len(t, got, 1)
	assert.Equal(t, 1, got[0].ID)
}
