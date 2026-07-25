package ui_test

import (
	"errors"
	"image"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/store"
	"github.com/sorokin-vladimir/tele/internal/ui"
	"github.com/sorokin-vladimir/tele/internal/ui/screens"
)

// cmdTimeout bounds how long a command may take before runUntilIdle treats it as
// a timer rather than pipeline work. Mocked network work returns instantly; the
// auto-clear ticks scheduled for toasts take seconds and must not stall a test.
const cmdTimeout = 300 * time.Millisecond

// execCmd runs a command (recursing into batches) and returns the messages it
// produced. Commands that do not answer within cmdTimeout are timers and are
// dropped.
func execCmd(c tea.Cmd) []tea.Msg {
	if c == nil {
		return nil
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- c() }()
	select {
	case msg := <-done:
		batch, ok := msg.(tea.BatchMsg)
		if !ok {
			if msg == nil {
				return nil
			}
			return []tea.Msg{msg}
		}
		var out []tea.Msg
		for _, inner := range batch {
			out = append(out, execCmd(inner)...)
		}
		return out
	case <-time.After(cmdTimeout):
		return nil
	}
}

// runUntilIdle feeds every message the pipeline produces back into the model
// until no commands are left, so a test can assert on the settled state.
func runUntilIdle(t *testing.T, m ui.RootModel, cmd tea.Cmd) ui.RootModel {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for steps := 0; len(queue) > 0; steps++ {
		require.Less(t, steps, 200, "pipeline did not settle")
		c := queue[0]
		queue = queue[1:]
		for _, msg := range execCmd(c) {
			if msg == nil {
				continue
			}
			nm, next := m.Update(msg)
			m = nm.(ui.RootModel)
			if next != nil {
				queue = append(queue, next)
			}
		}
	}
	return m
}

// visibleText settles the toast slide animation and returns the rendered frame
// with ANSI styling stripped, so assertions can match toast copy.
func visibleText(m ui.RootModel) string {
	m.SettleToastsForTest()
	return xansi.Strip(m.View().Content)
}

// stageAndSend stages paths, then drives the send request and every follow-up
// message the pipeline emits until it goes quiet.
func stageAndSend(t *testing.T, m ui.RootModel, paths []string) ui.RootModel {
	t.Helper()
	for _, p := range paths {
		nm, _ := m.Update(screens.FileSelectedMsg{Path: p})
		m = nm.(ui.RootModel)
	}
	nm, cmd := m.Update(screens.SendMediaRequest{
		Peer:    store.Peer{ID: 1, Type: store.PeerUser},
		Caption: "look",
	})
	m = nm.(ui.RootModel)
	return runUntilIdle(t, m, cmd)
}

func TestAlbumSend_TwoPhotosCreateTwoSentinelsAndOneAlbum(t *testing.T) {
	mc := &mockTGClient{}
	m, st := newRootOnChat(t, mc)
	m = stageAndSend(t, m, []string{
		writeTempBytes(t, "a.png", pngBytes),
		writeTempBytes(t, "b.png", pngBytes),
	})

	assert.Equal(t, 1, mc.sendAlbumCalls, "two parts must go out as one album")
	require.Len(t, mc.lastSendAlbumParams.Items, 2)
	assert.Equal(t, "look", mc.lastSendAlbumParams.Items[0].Caption)
	assert.Equal(t, "", mc.lastSendAlbumParams.Items[1].Caption, "only the first part carries the caption")
	assert.Equal(t, 0, m.PendingAttachmentCount(), "the queue is cleared on send")
	assert.Len(t, st.Messages(1), 2, "one optimistic bubble per file")
}

func TestAlbumSend_SingleFileStillUsesSendMedia(t *testing.T) {
	mc := &mockTGClient{}
	m, _ := newRootOnChat(t, mc)
	_ = stageAndSend(t, m, []string{writeTempBytes(t, "a.png", pngBytes)})

	assert.Equal(t, 0, mc.sendAlbumCalls, "a lone file must keep the single-media path")
	assert.NotNil(t, mc.lastSendMediaParams.Media)
}

func TestAlbumSend_MixedTypesSendTwoAlbums(t *testing.T) {
	mc := &mockTGClient{}
	m, _ := newRootOnChat(t, mc)
	_ = stageAndSend(t, m, []string{
		writeTempBytes(t, "a.png", pngBytes),
		writeTempBytes(t, "b.png", pngBytes),
		writeTempBytes(t, "c.pdf", []byte("%PDF-1.4 test")),
		writeTempBytes(t, "d.pdf", []byte("%PDF-1.4 test")),
	})

	assert.Equal(t, 2, mc.sendAlbumCalls, "photos and documents cannot share an album")
}

func TestAlbumSend_UploadFailureMarksThatPartAndSendsTheRest(t *testing.T) {
	mc := &mockTGClient{failUploadForName: "b.png"}
	m, st := newRootOnChat(t, mc)
	m = stageAndSend(t, m, []string{
		writeTempBytes(t, "a.png", pngBytes),
		writeTempBytes(t, "b.png", pngBytes),
		writeTempBytes(t, "c.png", pngBytes),
	})

	require.Equal(t, 1, mc.sendAlbumCalls)
	assert.Len(t, mc.lastSendAlbumParams.Items, 2, "the failed part drops out of the album")

	var failed int
	for _, msg := range st.Messages(1) {
		if msg.LocalMedia != nil && msg.LocalMedia.UploadState == store.UploadFailed {
			failed++
		}
	}
	assert.Equal(t, 1, failed, "the failed part keeps a failed bubble")
	assert.Contains(t, visibleText(m), "2 of 3 sent", "a partial failure is surfaced")
}

func TestAlbumSend_ConfirmedPartsShareTheGroupedID(t *testing.T) {
	mc := &mockTGClient{sendAlbumIDs: []int{101, 102}}
	mc.refreshFunc = func(id int) (store.Message, error) {
		return store.Message{ID: id, ChatID: 1, GroupedID: 777}, nil
	}
	m, st := newRootOnChat(t, mc)
	_ = stageAndSend(t, m, []string{
		writeTempBytes(t, "a.png", pngBytes),
		writeTempBytes(t, "b.png", pngBytes),
	})

	msgs := st.Messages(1)
	require.Len(t, msgs, 2)
	for _, msg := range msgs {
		assert.Equal(t, int64(777), msg.GroupedID, "album parts must share the grouped ID so they collapse into one bubble")
		assert.Contains(t, []int{101, 102}, msg.ID, "sentinels must be replaced by the server IDs")
	}
}

func TestAlbumSend_SendFailureMarksEveryPartFailed(t *testing.T) {
	mc := &mockTGClient{sendAlbumErr: errors.New("boom")}
	m, st := newRootOnChat(t, mc)
	m = stageAndSend(t, m, []string{
		writeTempBytes(t, "a.png", pngBytes),
		writeTempBytes(t, "b.png", pngBytes),
	})

	for _, msg := range st.Messages(1) {
		require.NotNil(t, msg.LocalMedia)
		assert.Equal(t, store.UploadFailed, msg.LocalMedia.UploadState)
	}
	assert.Contains(t, visibleText(m), "album send failed")
}

func TestAlbumSend_FullSuccessShowsNoToast(t *testing.T) {
	mc := &mockTGClient{}
	mc.refreshFunc = func(id int) (store.Message, error) {
		return store.Message{ID: id, ChatID: 1, GroupedID: 5}, nil
	}
	m, _ := newRootOnChat(t, mc)
	m = stageAndSend(t, m, []string{
		writeTempBytes(t, "a.png", pngBytes),
		writeTempBytes(t, "b.png", pngBytes),
	})

	text := visibleText(m)
	assert.NotContains(t, text, "sent", "a clean send speaks through its bubbles")
	assert.NotContains(t, text, "failed")
	assert.False(t, m.StatusBarTransferActive(), "the transfer slot is released when the send ends")
}

func TestAlbumSend_CaptionShowsOnTheOptimisticBubble(t *testing.T) {
	m, st := newRootOnChat(t, &mockTGClient{})
	for _, n := range []string{"a.png", "b.png"} {
		nm, _ := m.Update(screens.FileSelectedMsg{Path: writeTempBytes(t, n, pngBytes)})
		m = nm.(ui.RootModel)
	}

	// Only the send request is dispatched: the assertion is about what the user
	// sees while the album is still uploading, before any server answer.
	m.Update(screens.SendMediaRequest{
		Peer:    store.Peer{ID: 1, Type: store.PeerUser},
		Caption: "look",
	})

	msgs := st.Messages(1)
	require.Len(t, msgs, 2)
	captions := 0
	for _, msg := range msgs {
		if msg.Text != "" {
			captions++
			assert.Equal(t, "look", msg.Text)
		}
	}
	assert.Equal(t, 1, captions, "the caption rides on the first part, as Telegram renders an album")
}

func TestAlbumSend_AdoptedPartsRequestTheirInlineImages(t *testing.T) {
	mc := &mockTGClient{sendAlbumIDs: []int{101, 102}}
	mc.refreshFunc = func(id int) (store.Message, error) {
		return store.Message{
			ID: id, ChatID: 1, GroupedID: 777,
			Photo: &store.PhotoRef{ID: int64(900 + id), ThumbSize: "x"},
			Media: &store.MediaRef{Kind: store.MediaPhoto},
		}, nil
	}
	var downloads int
	mc.downloadPhotoFunc = func() (image.Image, error) {
		downloads++
		return image.NewRGBA(image.Rect(0, 0, 2, 2)), nil
	}
	m, st := newRootOnChat(t, mc)
	_ = stageAndSend(t, m, []string{
		writeTempBytes(t, "a.png", pngBytes),
		writeTempBytes(t, "b.png", pngBytes),
	})

	for _, msg := range st.Messages(1) {
		require.NotNil(t, msg.Photo, "the server photo ref must be adopted")
		require.NotNil(t, msg.Media, "without a MediaRef the renderer never looks up a preview")
	}
	assert.Equal(t, 2, downloads, "each adopted part must fetch its inline image")
}
