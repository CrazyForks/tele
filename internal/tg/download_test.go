package tg

import (
	"context"
	"io"
	"testing"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/sorokin-vladimir/tele/internal/telerr"
	"github.com/stretchr/testify/assert"
)

// A document with no thumbnail has no thumbnail location to build, so the
// request must be refused before it reaches Telegram.
func TestDownloadDocumentThumbToFile_RefusesADocumentWithoutAThumb(t *testing.T) {
	c := &GotdClient{}

	err := c.DownloadDocumentThumbToFile(context.Background(), domain.DocumentRef{ID: 7}, io.Discard)

	assert.Equal(t, telerr.NotFound, telerr.Of(err))
}

// DownloadPhotoToFile must fail cleanly (not panic) when the client is not
// connected; the streaming path is exercised end-to-end by manual testing.
func TestDownloadPhotoToFile_NotConnected(t *testing.T) {
	c := &GotdClient{}
	err := c.DownloadPhotoToFile(context.Background(), domain.PhotoRef{ID: 1}, io.Discard)
	assert.Error(t, err)
}

// DownloadDocumentToFile must fail cleanly (not panic) when the client is not
// connected; the streaming path is exercised end-to-end by manual testing.
func TestDownloadDocumentToFile_NotConnected(t *testing.T) {
	c := &GotdClient{}
	err := c.DownloadDocumentToFile(context.Background(), domain.DocumentRef{ID: 1}, io.Discard)
	assert.Error(t, err)
}
