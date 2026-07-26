package notices_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/notices"
	"github.com/sorokin-vladimir/tele/internal/store"
)

// The SQLite implementation is the one that ships, so it is exercised against a
// real database rather than trusted to match the in-memory one.
func TestSQLiteSeen_PersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")

	st, err := store.NewSQLite(path, zap.NewNop())
	require.NoError(t, err)

	seen := notices.NewSQLiteSeen(st.DB())
	assert.False(t, seen.IsSeen("some-notice"))
	seen.MarkSeen("some-notice")
	assert.True(t, seen.IsSeen("some-notice"))
	assert.False(t, seen.IsSeen("other-notice"))
	require.NoError(t, st.Close())

	reopened, err := store.NewSQLite(path, zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = reopened.Close() })

	assert.True(t, notices.NewSQLiteSeen(reopened.DB()).IsSeen("some-notice"),
		"a dismissed notice must stay dismissed after a restart")
}

func TestSQLiteSeen_MarkIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	st, err := store.NewSQLite(path, zap.NewNop())
	require.NoError(t, err)
	t.Cleanup(func() { _ = st.Close() })

	seen := notices.NewSQLiteSeen(st.DB())
	seen.MarkSeen("dup")
	seen.MarkSeen("dup")
	assert.True(t, seen.IsSeen("dup"))
}
