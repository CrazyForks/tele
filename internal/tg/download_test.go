package tg

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/gotd/td/tgerr"
	"github.com/stretchr/testify/assert"

	"github.com/sorokin-vladimir/tele/internal/store"
)

func TestIsFileReferenceExpired(t *testing.T) {
	expired := &tgerr.Error{Code: 400, Type: "FILE_REFERENCE_EXPIRED"}
	assert.True(t, IsFileReferenceExpired(expired))
	assert.False(t, IsFileReferenceExpired(errors.New("boom")))
	assert.False(t, IsFileReferenceExpired(nil))
}

// DownloadDocumentImage must fail cleanly (not panic) when the client is not
// connected; the decode path itself is exercised end-to-end by manual testing.
func TestDownloadDocumentImage_NotConnected(t *testing.T) {
	c := &GotdClient{}
	_, err := c.DownloadDocumentImage(context.Background(), store.DocumentRef{ID: 1})
	assert.Error(t, err)
}

// DownloadDocumentToFile must fail cleanly (not panic) when the client is not
// connected; the streaming path is exercised end-to-end by manual testing.
func TestDownloadDocumentToFile_NotConnected(t *testing.T) {
	c := &GotdClient{}
	err := c.DownloadDocumentToFile(context.Background(), store.DocumentRef{ID: 1}, io.Discard)
	assert.Error(t, err)
}
