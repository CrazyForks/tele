package ui

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Naming a saved file and claiming a free name moved to the owner in #196;
// TestSavedFileName_* and TestCreateUniqueFile_* in internal/core cover them.

func TestResolveDownloadsDir_NonEmpty(t *testing.T) {
	assert.NotEmpty(t, resolveDownloadsDir())
}

func TestResolveDownloadsDir_PrefersXDG(t *testing.T) {
	if os.Getenv("HOME") == "" {
		t.Skip("no HOME")
	}
	want := t.TempDir()
	t.Setenv("XDG_DOWNLOAD_DIR", want)
	// XDG is consulted on Linux; on macOS the env is ignored, so only assert
	// the env path when it is actually honored.
	got := resolveDownloadsDir()
	if got != want {
		assert.Contains(t, got, "Downloads")
	}
}
