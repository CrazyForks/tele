package statedir_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/statedir"
)

func TestAcquire_CreatesDirectoryAndLockFile(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "state")

	l, err := statedir.Acquire(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = l.Release() })

	info, err := os.Stat(dir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())

	_, err = os.Stat(filepath.Join(dir, "tele.lock"))
	assert.NoError(t, err)
}

// flock and LockFileEx are held per open file description, so a second Acquire
// is denied even from the same process. That is what makes this testable
// without spawning a child.
func TestAcquire_SecondAcquireIsDenied(t *testing.T) {
	dir := t.TempDir()

	first, err := statedir.Acquire(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Release() })

	second, err := statedir.Acquire(dir)
	require.Error(t, err)
	assert.Nil(t, second)
	assert.True(t, errors.Is(err, statedir.ErrLocked), "want ErrLocked, got %v", err)
}

func TestAcquire_ErrorNamesTheHoldingPid(t *testing.T) {
	dir := t.TempDir()

	first, err := statedir.Acquire(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = first.Release() })

	_, err = statedir.Acquire(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("pid %d", os.Getpid()))
}

func TestRelease_AllowsReacquire(t *testing.T) {
	dir := t.TempDir()

	first, err := statedir.Acquire(dir)
	require.NoError(t, err)
	require.NoError(t, first.Release())

	second, err := statedir.Acquire(dir)
	require.NoError(t, err)
	assert.NoError(t, second.Release())
}

func TestRelease_IsIdempotent(t *testing.T) {
	dir := t.TempDir()

	l, err := statedir.Acquire(dir)
	require.NoError(t, err)

	require.NoError(t, l.Release())
	assert.NoError(t, l.Release())
}
