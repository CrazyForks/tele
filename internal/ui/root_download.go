package ui

import (
	"context"
	"image"
	"io"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/sorokin-vladimir/tele/internal/store"
	internaltg "github.com/sorokin-vladimir/tele/internal/tg"
	"github.com/sorokin-vladimir/tele/internal/ui/components"
	"github.com/sorokin-vladimir/tele/internal/ui/media"
)

// downloadWithRefresh runs download(ref); on a FILE_REFERENCE_EXPIRED error it
// refreshes the message's media refs once via RefreshMessage and retries with the
// fresh ref. On a successful retry it returns the refreshed message so the caller
// can persist the new ref.
func downloadWithRefresh[T any, R any](
	ctx context.Context,
	client internaltg.Client,
	peer store.Peer,
	msgID int,
	ref R,
	download func(R) (T, error),
	pickRef func(store.Message) (R, bool),
) (result T, refreshed *store.Message, err error) {
	result, err = download(ref)
	if err == nil {
		return result, nil, nil
	}
	if !internaltg.IsFileReferenceExpired(err) {
		return result, nil, err
	}
	msg, rerr := client.RefreshMessage(ctx, peer, msgID)
	if rerr != nil {
		return result, nil, err
	}
	newRef, ok := pickRef(msg)
	if !ok {
		return result, nil, err
	}
	result, err = download(newRef)
	if err != nil {
		return result, nil, err
	}
	return result, &msg, nil
}

func downloadPhotoCmd(ctx context.Context, client internaltg.Client, peer store.Peer, msgID int, ref store.PhotoRef) tea.Cmd {
	return func() tea.Msg {
		img, refreshed, err := downloadWithRefresh(ctx, client, peer, msgID, ref,
			func(r store.PhotoRef) (image.Image, error) {
				return client.DownloadPhoto(ctx, r)
			},
			func(m store.Message) (store.PhotoRef, bool) {
				if m.Photo == nil {
					return store.PhotoRef{}, false
				}
				return *m.Photo, true
			},
		)
		if err != nil {
			return StatusErrMsg{Text: "photo download failed: " + err.Error(), Sev: components.SeverityWarning}
		}
		ready := PhotoReadyMsg{PhotoID: ref.ID, Image: img}
		if refreshed != nil {
			return refreshedBatch(ready, mediaRefRefreshedMsg{chatID: peer.ID, msgID: msgID, photo: refreshed.Photo})
		}
		return ready
	}
}

// DownloadPhotoCmdForTest exposes downloadPhotoCmd for tests.
func DownloadPhotoCmdForTest(c internaltg.Client, peer store.Peer, msgID int, ref store.PhotoRef) tea.Cmd {
	return downloadPhotoCmd(context.Background(), c, peer, msgID, ref)
}

// HistoryChunkMsgForTest builds a historyChunkMsg for tests.
func HistoryChunkMsgForTest(chatID int64, msgs []store.Message) tea.Msg {
	return historyChunkMsg{chatID: chatID, messages: msgs}
}

// refreshedBatch emits both the ready image and the store-update message after a
// successful refresh+retry.
func refreshedBatch(ready, refreshed tea.Msg) tea.Msg {
	return tea.BatchMsg{
		func() tea.Msg { return ready },
		func() tea.Msg { return refreshed },
	}
}

// currentPeer returns the peer of the currently open chat, or the zero peer.
func (m RootModel) currentPeer() store.Peer {
	if m.st != nil {
		if chat, ok := m.st.GetChat(m.currentChatID); ok {
			return chat.Peer
		}
	}
	return store.Peer{}
}

// openDocumentCmd downloads a document in full and opens it in the OS default
// application (e.g. a video player). Runs async; the download may be large.
func openDocumentCmd(ctx context.Context, client internaltg.Client, peer store.Peer, msgID int, ref store.DocumentRef, tmpDir string) tea.Cmd {
	return func() tea.Msg {
		ext := filepath.Ext(ref.FileName)
		if ext == "" {
			ext = extFromMime(ref.MimeType)
		}
		f, err := createTempMediaFile(tmpDir, ext)
		if err != nil {
			return StatusErrMsg{Text: "open file failed: " + err.Error(), Sev: components.SeverityWarning}
		}
		name := f.Name()

		// Stream directly to disk; the whole file is never held in memory. On a
		// FILE_REFERENCE_EXPIRED retry the file is truncated so a partial first
		// attempt does not corrupt the result.
		_, refreshed, derr := downloadWithRefresh(ctx, client, peer, msgID, ref,
			func(r store.DocumentRef) (struct{}, error) {
				if _, serr := f.Seek(0, io.SeekStart); serr != nil {
					return struct{}{}, serr
				}
				if terr := f.Truncate(0); terr != nil {
					return struct{}{}, terr
				}
				return struct{}{}, client.DownloadDocumentToFile(ctx, r, f)
			},
			pickDocumentRef,
		)
		if derr != nil {
			_ = f.Close()
			_ = os.Remove(name)
			return StatusErrMsg{Text: "open file failed: " + derr.Error(), Sev: components.SeverityWarning}
		}
		if cerr := f.Close(); cerr != nil {
			_ = os.Remove(name)
			return StatusErrMsg{Text: "open file failed: " + cerr.Error(), Sev: components.SeverityWarning}
		}
		openPath(name)
		if refreshed != nil {
			return mediaRefRefreshedMsg{chatID: peer.ID, msgID: msgID, doc: refreshed.Document}
		}
		return nil
	}
}

// OpenDocumentCmdForTest exposes openDocumentCmd for tests.
func OpenDocumentCmdForTest(c internaltg.Client, peer store.Peer, msgID int, ref store.DocumentRef, tmpDir string) tea.Cmd {
	return openDocumentCmd(context.Background(), c, peer, msgID, ref, tmpDir)
}

// SetOpenPathForTest swaps the OS file launcher and returns a restore func.
func SetOpenPathForTest(fn func(string)) func() {
	prev := openPath
	openPath = fn
	return func() { openPath = prev }
}

// pickDocumentRef extracts a message's fresh document ref, used as the refresh
// selector for document downloads.
func pickDocumentRef(m store.Message) (store.DocumentRef, bool) {
	if m.Document == nil {
		return store.DocumentRef{}, false
	}
	return *m.Document, true
}

// extFromMime maps common video MIME types to a file extension so the OS picks
// the right player. Defaults to .mp4 (the usual Telegram video container).
func extFromMime(mime string) string {
	switch mime {
	case "video/quicktime":
		return ".mov"
	case "video/webm":
		return ".webm"
	case "video/x-matroska":
		return ".mkv"
	default:
		return ".mp4"
	}
}

func downloadVoiceCmd(ctx context.Context, client internaltg.Client, peer store.Peer, msgID int, ref store.DocumentRef) tea.Cmd {
	return func() tea.Msg {
		data, refreshed, err := downloadWithRefresh(ctx, client, peer, msgID, ref,
			func(r store.DocumentRef) ([]byte, error) {
				return client.DownloadDocument(ctx, r)
			},
			pickDocumentRef,
		)
		if err != nil {
			return StatusErrMsg{Text: "voice download failed: " + err.Error(), Sev: components.SeverityWarning}
		}
		if len(data) == 0 {
			return nil
		}
		ready := voicePlayReadyMsg{docID: ref.ID, data: data}
		if refreshed != nil {
			return refreshedBatch(ready, mediaRefRefreshedMsg{chatID: peer.ID, msgID: msgID, doc: refreshed.Document})
		}
		return ready
	}
}

func downloadVideoThumbCmd(ctx context.Context, client internaltg.Client, peer store.Peer, msgID int, ref store.DocumentRef, crop bool) tea.Cmd {
	return func() tea.Msg {
		img, refreshed, err := downloadWithRefresh(ctx, client, peer, msgID, ref,
			func(r store.DocumentRef) (image.Image, error) {
				return client.DownloadDocumentThumb(ctx, r)
			},
			func(m store.Message) (store.DocumentRef, bool) {
				if m.Document == nil {
					return store.DocumentRef{}, false
				}
				return *m.Document, true
			},
		)
		if err != nil || img == nil {
			if err != nil {
				return StatusErrMsg{Text: "video thumb download failed: " + err.Error(), Sev: components.SeverityWarning}
			}
			return nil
		}
		if crop {
			img = media.CircleCrop(img) // round video note → circle
		}
		// Reuse the photo-ready path; the cache is keyed by id (here the document id).
		ready := PhotoReadyMsg{PhotoID: ref.ID, Image: img}
		if refreshed != nil {
			return refreshedBatch(ready, mediaRefRefreshedMsg{chatID: peer.ID, msgID: msgID, doc: refreshed.Document})
		}
		return ready
	}
}

// downloadStickerCmd downloads and decodes a static WEBP sticker (the full
// document) and feeds it through the inline-image cache keyed by document id.
func downloadStickerCmd(ctx context.Context, client internaltg.Client, peer store.Peer, msgID int, ref store.DocumentRef) tea.Cmd {
	return func() tea.Msg {
		img, refreshed, err := downloadWithRefresh(ctx, client, peer, msgID, ref,
			func(r store.DocumentRef) (image.Image, error) {
				return client.DownloadDocumentImage(ctx, r)
			},
			func(m store.Message) (store.DocumentRef, bool) {
				if m.Document == nil {
					return store.DocumentRef{}, false
				}
				return *m.Document, true
			},
		)
		if err != nil || img == nil {
			if err != nil {
				return StatusErrMsg{Text: "sticker download failed: " + err.Error(), Sev: components.SeverityWarning}
			}
			return nil
		}
		ready := PhotoReadyMsg{PhotoID: ref.ID, Image: img}
		if refreshed != nil {
			return refreshedBatch(ready, mediaRefRefreshedMsg{chatID: peer.ID, msgID: msgID, doc: refreshed.Document})
		}
		return ready
	}
}

// DownloadStickerCmdForTest exposes downloadStickerCmd for tests.
func DownloadStickerCmdForTest(c internaltg.Client, peer store.Peer, msgID int, ref store.DocumentRef) tea.Cmd {
	return downloadStickerCmd(context.Background(), c, peer, msgID, ref)
}

func downloadFullPhotoCmd(ctx context.Context, client internaltg.Client, peer store.Peer, msgID int, ref store.PhotoRef) tea.Cmd {
	fullRef := ref
	fullRef.ThumbSize = ref.FullThumbSize
	return func() tea.Msg {
		img, refreshed, err := downloadWithRefresh(ctx, client, peer, msgID, fullRef,
			func(r store.PhotoRef) (image.Image, error) {
				return client.DownloadPhoto(ctx, r)
			},
			func(m store.Message) (store.PhotoRef, bool) {
				if m.Photo == nil {
					return store.PhotoRef{}, false
				}
				r := *m.Photo
				r.ThumbSize = r.FullThumbSize
				return r, true
			},
		)
		if err != nil || img == nil {
			if err != nil {
				return StatusErrMsg{Text: "full photo download failed: " + err.Error(), Sev: components.SeverityWarning}
			}
			return nil
		}
		ready := FullPhotoReadyMsg{PhotoID: ref.ID, Image: img}
		if refreshed != nil {
			return refreshedBatch(ready, mediaRefRefreshedMsg{chatID: peer.ID, msgID: msgID, photo: refreshed.Photo})
		}
		return ready
	}
}

func (m RootModel) pendingDownloadCmds(msgs []store.Message) tea.Cmd {
	var cmds []tea.Cmd
	for _, msg := range msgs {
		var peer store.Peer
		if m.st != nil {
			if chat, ok := m.st.GetChat(msg.ChatID); ok {
				peer = chat.Peer
			}
		}
		if msg.Photo != nil {
			if _, ok := m.imageCache[msg.Photo.ID]; !ok {
				cmds = append(cmds, downloadPhotoCmd(m.ctx, m.tgClient, peer, msg.ID, *msg.Photo))
			}
			if m.cfg != nil && m.cfg.Photos.EagerFullQuality && msg.Photo.FullThumbSize != "" {
				if _, ok := m.fullImageCache[msg.Photo.ID]; !ok {
					cmds = append(cmds, downloadFullPhotoCmd(m.ctx, m.tgClient, peer, msg.ID, *msg.Photo))
				}
			}
		}
		// Video thumbnails reuse the inline-image cache, keyed by document id.
		if msg.Media != nil && msg.Media.Kind.IsVideo() && msg.Document != nil && msg.Document.ThumbSize != "" {
			if _, ok := m.imageCache[msg.Document.ID]; !ok {
				// Round video notes are cropped to a circle, but only in Kitty mode
				// (PNG alpha); block-art has no transparency, so keep it square there.
				crop := msg.Media.Kind == store.MediaVideoNote && m.imageMode == media.ModeKitty
				cmds = append(cmds, downloadVideoThumbCmd(m.ctx, m.tgClient, peer, msg.ID, *msg.Document, crop))
			}
		}
		// Static WEBP stickers render inline (Kitty only); decode the full document.
		if m.imageMode == media.ModeKitty && store.IsStaticSticker(msg.Media, msg.Document) {
			if _, ok := m.imageCache[msg.Document.ID]; !ok {
				cmds = append(cmds, downloadStickerCmd(m.ctx, m.tgClient, peer, msg.ID, *msg.Document))
			}
		}
	}
	return tea.Batch(cmds...)
}
