package statedir

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// os.Rename succeeds inside a single temp directory, so the copy fallback is
// never reached by the Migrate tests. Exercise it directly.
func TestCopyFile_CopiesContentWithTightPermissions(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	require.NoError(t, os.WriteFile(src, []byte("payload"), 0o600))

	require.NoError(t, copyFile(src, dst))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, "payload", string(got))

	if runtime.GOOS == "windows" {
		return // Windows approximates Unix mode bits
	}
	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestCopyFile_FailsOnMissingSource(t *testing.T) {
	dir := t.TempDir()
	assert.Error(t, copyFile(filepath.Join(dir, "absent"), filepath.Join(dir, "dst")))
}
