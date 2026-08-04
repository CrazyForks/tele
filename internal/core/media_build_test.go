package core

import (
	"context"
	"errors"
	"testing"

	"github.com/gotd/td/tg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/domain"
)

// errRefused stands in for whatever Telegram refused with. The taxonomy is
// mapped in internal/tg, so the core only has to propagate.
var errRefused = errors.New("upload refused")

func TestUploadPart_AnImageBecomesAnUploadedPhoto(t *testing.T) {
	c := &stubClient{}
	o, _ := newCmdOwner(t, c)
	part := domain.OutboxMediaPart{Path: writeFile(t, "a.jpg", 4), Name: "a.jpg"}

	got, err := o.uploadPart(context.Background(), part, nil)

	require.NoError(t, err)
	assert.IsType(t, &tg.InputMediaUploadedPhoto{}, got)
	assert.Equal(t, []string{part.Path}, c.uploads())
}

func TestUploadPart_SendAsFileOverridesTheDetectedKind(t *testing.T) {
	c := &stubClient{}
	o, _ := newCmdOwner(t, c)
	part := domain.OutboxMediaPart{
		Path: writeFile(t, "a.jpg", 4), Name: "a.jpg", SendAs: domain.MediaFile,
	}

	got, err := o.uploadPart(context.Background(), part, nil)

	require.NoError(t, err)
	doc, ok := got.(*tg.InputMediaUploadedDocument)
	require.True(t, ok, "the user asked for a file, not a photo")
	assert.Equal(t, "image/jpeg", doc.MimeType)
	assert.Equal(t, "a.jpg", documentFileName(t, doc))
	assert.True(t, doc.ForceFile,
		"without ForceFile Telegram reinterprets an image as a photo, undoing the choice")
}

// Video is a document too, but a streamable one: ForceFile would make Telegram
// render it as an attachment instead of playing it inline.
func TestUploadPart_AVideoIsSentAsStreamableVideo(t *testing.T) {
	c := &stubClient{}
	o, _ := newCmdOwner(t, c)
	part := domain.OutboxMediaPart{Path: writeFile(t, "clip.mp4", 4), Name: "clip.mp4"}

	got, err := o.uploadPart(context.Background(), part, nil)

	require.NoError(t, err)
	doc, ok := got.(*tg.InputMediaUploadedDocument)
	require.True(t, ok, "got %T", got)
	assert.False(t, doc.ForceFile)
	assert.Equal(t, "video/mp4", doc.MimeType)
	var hasVideoAttr bool
	for _, a := range doc.Attributes {
		if _, ok := a.(*tg.DocumentAttributeVideo); ok {
			hasVideoAttr = true
		}
	}
	assert.True(t, hasVideoAttr, "a video needs its video attribute to play inline")
}

func TestUploadPart_AnUnknownTypeBecomesADocument(t *testing.T) {
	c := &stubClient{}
	o, _ := newCmdOwner(t, c)
	part := domain.OutboxMediaPart{Path: writeFile(t, "notes.txt", 4), Name: "notes.txt"}

	got, err := o.uploadPart(context.Background(), part, nil)

	require.NoError(t, err)
	assert.IsType(t, &tg.InputMediaUploadedDocument{}, got)
}

func TestUploadPart_ReportsProgress(t *testing.T) {
	c := &stubClient{}
	o, _ := newCmdOwner(t, c)
	part := domain.OutboxMediaPart{Path: writeFile(t, "a.jpg", 4), Name: "a.jpg"}

	var last int64
	_, err := o.uploadPart(context.Background(), part, func(sent, _ int64) { last = sent })

	require.NoError(t, err)
	assert.Equal(t, int64(100), last)
}

func TestUploadPart_AFailedUploadIsReturned(t *testing.T) {
	c := &stubClient{uploadErr: errRefused}
	o, _ := newCmdOwner(t, c)
	part := domain.OutboxMediaPart{Path: writeFile(t, "a.jpg", 4), Name: "a.jpg"}

	_, err := o.uploadPart(context.Background(), part, nil)

	require.ErrorIs(t, err, errRefused)
}

// documentFileName digs the file name out of a document's attributes, which is
// where Telegram keeps it.
func documentFileName(t *testing.T, doc *tg.InputMediaUploadedDocument) string {
	t.Helper()
	for _, a := range doc.Attributes {
		if fn, ok := a.(*tg.DocumentAttributeFilename); ok {
			return fn.FileName
		}
	}
	return ""
}
