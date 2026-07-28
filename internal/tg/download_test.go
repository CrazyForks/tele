package tg

import (
	"context"
	"io"
	"testing"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/stretchr/testify/assert"
)

// DownloadDocumentImage must fail cleanly (not panic) when the client is not
// connected; the decode path itself is exercised end-to-end by manual testing.
func TestDownloadDocumentImage_NotConnected(t *testing.T) {
	c := &GotdClient{}
	_, err := c.DownloadDocumentImage(context.Background(), domain.DocumentRef{ID: 1})
	assert.Error(t, err)
}

// DownloadDocumentToFile must fail cleanly (not panic) when the client is not
// connected; the streaming path is exercised end-to-end by manual testing.
func TestDownloadDocumentToFile_NotConnected(t *testing.T) {
	c := &GotdClient{}
	err := c.DownloadDocumentToFile(context.Background(), domain.DocumentRef{ID: 1}, io.Discard)
	assert.Error(t, err)
}
