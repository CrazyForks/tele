package ui_test

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/gotd/td/tgerr"
	"github.com/sorokin-vladimir/tele/internal/store"
	"github.com/sorokin-vladimir/tele/internal/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openDocumentCmd must stream the document straight to a temp file (never
// buffering it) and hand that path to the OS launcher.
func TestOpenDocumentCmd_StreamsToTempFile(t *testing.T) {
	tmpDir := t.TempDir()
	const body = "the entire document body"

	client := &mockTGClient{
		downloadDocFileFunc: func(dst io.Writer) error {
			_, err := io.WriteString(dst, body)
			return err
		},
	}

	var opened string
	restore := ui.SetOpenPathForTest(func(name string) { opened = name })
	defer restore()

	ref := store.DocumentRef{ID: 7, FileName: "clip.mp4"}
	msg := ui.OpenDocumentCmdForTest(client, store.Peer{ID: 1}, 99, ref, tmpDir)()

	assert.Nil(t, msg, "successful open without refresh returns no message")
	require.NotEmpty(t, opened, "OS launcher must be invoked with the temp path")
	assert.Equal(t, tmpDir, filepath.Dir(opened), "temp file lives in tmpDir")
	assert.Equal(t, ".mp4", filepath.Ext(opened))

	data, err := os.ReadFile(opened)
	require.NoError(t, err)
	assert.Equal(t, body, string(data))
}

// On FILE_REFERENCE_EXPIRED the partial first attempt must be discarded: the
// file is truncated before the retry so the result is exactly the retry's data.
func TestOpenDocumentCmd_TruncatesOnRetry(t *testing.T) {
	tmpDir := t.TempDir()
	const fresh = "fresh"

	calls := 0
	client := &mockTGClient{
		downloadDocFileFunc: func(dst io.Writer) error {
			calls++
			if calls == 1 {
				// Simulate a partial write before the reference expires.
				_, _ = io.WriteString(dst, "stale-partial-bytes")
				return &tgerr.Error{Code: 400, Type: "FILE_REFERENCE_EXPIRED"}
			}
			_, err := io.WriteString(dst, fresh)
			return err
		},
		refreshFunc: func(int) (store.Message, error) {
			return store.Message{Document: &store.DocumentRef{ID: 7, FileName: "clip.mp4"}}, nil
		},
	}

	var opened string
	restore := ui.SetOpenPathForTest(func(name string) { opened = name })
	defer restore()

	ref := store.DocumentRef{ID: 7, FileName: "clip.mp4"}
	msg := ui.OpenDocumentCmdForTest(client, store.Peer{ID: 1}, 99, ref, tmpDir)()

	assert.Equal(t, 2, calls, "must retry once after refresh")
	_, isErr := msg.(ui.StatusErrMsg)
	assert.False(t, isErr, "retry succeeds, so no error status")
	require.NotEmpty(t, opened)

	data, err := os.ReadFile(opened)
	require.NoError(t, err)
	assert.Equal(t, fresh, string(data), "stale partial bytes must be truncated away")
}
