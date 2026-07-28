package store

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newFileStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLite(filepath.Join(t.TempDir(), "state.db"), zap.NewNop())
	require.NoError(t, err)
	return s
}

func TestMarkMsgDirty_SkipsOptimisticNegativeID(t *testing.T) {
	s := newFileStore(t)
	defer func() { _ = s.Close() }()
	s.SetChat(domain.Chat{ID: 1, Peer: domain.Peer{ID: 1, Type: domain.PeerUser}})

	// An optimistic outgoing message with a negative sentinel id, plus a progress
	// tick — neither should be queued for persistence.
	s.AppendMessage(domain.Message{ID: -100, ChatID: 1, Text: "sending", Date: time.Unix(1, 0)})
	s.UpdateLocalMediaProgress(-100, 0.5)

	s.mu.Lock()
	dirty := len(s.dirtyMsgs[1])
	s.mu.Unlock()
	assert.Equal(t, 0, dirty, "negative-id messages must never be queued for persistence")
}

func TestLoadMessages_DoesNotQueueWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")

	s, err := NewSQLite(path, zap.NewNop())
	require.NoError(t, err)
	s.SetChat(domain.Chat{ID: 2, Peer: domain.Peer{ID: 2, Type: domain.PeerUser}})
	s.SetMessages(2, []domain.Message{{ID: 1, ChatID: 2, Text: "hi", Date: time.Unix(1, 0)}})
	require.NoError(t, s.Close())

	s2, err := NewSQLite(path, zap.NewNop())
	require.NoError(t, err)
	defer func() { _ = s2.Close() }()
	s2.LoadMessages(2)

	s2.mu.Lock()
	dirty := len(s2.dirtyMsgs[2])
	del := len(s2.deletedMsgs[2])
	s2.mu.Unlock()
	assert.Equal(t, 0, dirty, "loading from disk must not queue upserts")
	assert.Equal(t, 0, del, "loading a within-cap tail must not queue deletes")
}
