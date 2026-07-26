package statedir_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/sorokin-vladimir/tele/internal/statedir"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestMigrate_MovesSessionAndDatabase(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "config")
	state := filepath.Join(root, "state")
	require.NoError(t, os.MkdirAll(state, 0o700))

	write(t, filepath.Join(legacy, "session.json"), "session-payload")
	write(t, filepath.Join(legacy, "state.db"), "db-payload")
	write(t, filepath.Join(legacy, "state.db-wal"), "wal-payload")

	moved, err := statedir.Migrate(state, legacy, zap.NewNop())
	require.NoError(t, err)
	assert.True(t, moved)

	got, err := os.ReadFile(filepath.Join(state, "session.json"))
	require.NoError(t, err)
	assert.Equal(t, "session-payload", string(got))

	got, err = os.ReadFile(filepath.Join(state, "state.db"))
	require.NoError(t, err)
	assert.Equal(t, "db-payload", string(got))

	got, err = os.ReadFile(filepath.Join(state, "state.db-wal"))
	require.NoError(t, err)
	assert.Equal(t, "wal-payload", string(got))

	// Sources are gone: this is a move, not a copy.
	_, err = os.Stat(filepath.Join(legacy, "session.json"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(legacy, "state.db"))
	assert.True(t, os.IsNotExist(err))
}

func TestMigrate_NoopWhenNothingToMigrate(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "config")
	state := filepath.Join(root, "state")
	require.NoError(t, os.MkdirAll(legacy, 0o700))
	require.NoError(t, os.MkdirAll(state, 0o700))

	moved, err := statedir.Migrate(state, legacy, zap.NewNop())
	require.NoError(t, err)
	assert.False(t, moved)

	entries, err := os.ReadDir(state)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestMigrate_NoopWhenAlreadyMigrated(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "config")
	state := filepath.Join(root, "state")

	write(t, filepath.Join(state, "session.json"), "current")
	write(t, filepath.Join(legacy, "session.json"), "stale")

	moved, err := statedir.Migrate(state, legacy, zap.NewNop())
	require.NoError(t, err)
	assert.False(t, moved)

	got, err := os.ReadFile(filepath.Join(state, "session.json"))
	require.NoError(t, err)
	assert.Equal(t, "current", string(got))

	// The legacy copy is left strictly alone.
	got, err = os.ReadFile(filepath.Join(legacy, "session.json"))
	require.NoError(t, err)
	assert.Equal(t, "stale", string(got))
}

func TestMigrate_NoopWhenDirsAreTheSame(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "session.json"), "payload")

	moved, err := statedir.Migrate(dir, dir, zap.NewNop())
	require.NoError(t, err)
	assert.False(t, moved)

	got, err := os.ReadFile(filepath.Join(dir, "session.json"))
	require.NoError(t, err)
	assert.Equal(t, "payload", string(got))
}

// A crash after the database moved but before the session did must be
// completable by simply running again.
func TestMigrate_ResumesAfterPartialMove(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "config")
	state := filepath.Join(root, "state")

	write(t, filepath.Join(state, "state.db"), "db-payload")
	write(t, filepath.Join(legacy, "session.json"), "session-payload")

	moved, err := statedir.Migrate(state, legacy, zap.NewNop())
	require.NoError(t, err)
	assert.True(t, moved)

	got, err := os.ReadFile(filepath.Join(state, "session.json"))
	require.NoError(t, err)
	assert.Equal(t, "session-payload", string(got))
}
