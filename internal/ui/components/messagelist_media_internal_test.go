package components

import (
	"strings"
	"testing"

	"github.com/sorokin-vladimir/tele/internal/store"
)

func TestRenderUploadBar(t *testing.T) {
	bar := renderUploadBar(0.6, 20)
	if !strings.Contains(bar, "60%") {
		t.Fatalf("bar missing percent: %q", bar)
	}
}

func TestUploadStatusLineFailed(t *testing.T) {
	s := uploadStatusLine(&store.LocalMedia{UploadState: store.UploadFailed}, 20)
	if !strings.Contains(strings.ToLower(s), "failed") {
		t.Fatalf("failed status missing: %q", s)
	}
}

func TestUploadStatusLineUploading(t *testing.T) {
	s := uploadStatusLine(&store.LocalMedia{UploadProgress: 0.4}, 20)
	if !strings.Contains(s, "40%") {
		t.Fatalf("uploading status missing percent: %q", s)
	}
}

func TestLocalMediaLabel_Photo(t *testing.T) {
	got := localMediaLabel(&store.LocalMedia{Kind: store.MediaPhoto, FileName: "pic.jpg"})
	if !strings.HasPrefix(got, "🖼") || !strings.Contains(got, "pic.jpg") {
		t.Fatalf("photo label want 🖼 + name, got %q", got)
	}
}

func TestLocalMediaLabel_File(t *testing.T) {
	got := localMediaLabel(&store.LocalMedia{Kind: store.MediaFile, FileName: "report.pdf"})
	if !strings.HasPrefix(got, "📎") || !strings.Contains(got, "report.pdf") {
		t.Fatalf("file label want 📎 + name, got %q", got)
	}
}

func TestVideoOverlayLabel_GIF(t *testing.T) {
	if got := videoOverlayLabel(&store.MediaRef{Kind: store.MediaGIF}); got != "GIF" {
		t.Fatalf("GIF overlay want \"GIF\", got %q", got)
	}
}

func TestVideoOverlayLabel_PhotoEmpty(t *testing.T) {
	if got := videoOverlayLabel(&store.MediaRef{Kind: store.MediaPhoto}); got != "" {
		t.Fatalf("photo must have no overlay label, got %q", got)
	}
}

func TestOverlayLabelFor_GIFLoadingSpinner(t *testing.T) {
	ml := NewMessageList(20, 40)
	gif := store.Message{
		Media:    &store.MediaRef{Kind: store.MediaGIF},
		Document: &store.DocumentRef{ID: 5, ThumbSize: "m"},
	}
	if got := ml.overlayLabelFor(gif); got != "GIF" {
		t.Fatalf("idle GIF want \"GIF\", got %q", got)
	}
	ml.SetGifLoading(5, "⠋")
	if got := ml.overlayLabelFor(gif); got != "⠋ GIF" {
		t.Fatalf("loading GIF want \"⠋ GIF\", got %q", got)
	}
	ml.SetGifLoading(99, "⠋") // a different gif is loading
	if got := ml.overlayLabelFor(gif); got != "GIF" {
		t.Fatalf("non-loading GIF want \"GIF\", got %q", got)
	}
}

func TestSelectedMessageGIF(t *testing.T) {
	ml := NewMessageList(20, 40)
	ml.SetMessages([]store.Message{{
		ID:       1,
		Media:    &store.MediaRef{Kind: store.MediaGIF},
		Document: &store.DocumentRef{ID: 55, ThumbSize: "m"},
	}})
	ref, ok := ml.SelectedMessageGIF()
	if !ok || ref.ID != 55 {
		t.Fatalf("got (id=%d, ok=%v), want (55, true)", ref.ID, ok)
	}
}

func TestSelectedMessageGIF_NotAGif(t *testing.T) {
	ml := NewMessageList(20, 40)
	ml.SetMessages([]store.Message{{
		ID:    1,
		Media: &store.MediaRef{Kind: store.MediaPhoto},
		Photo: &store.PhotoRef{ID: 9},
	}})
	if _, ok := ml.SelectedMessageGIF(); ok {
		t.Fatal("photo selection must not report a GIF")
	}
}

func TestPreviewImageID_GIFWithThumb(t *testing.T) {
	ml := NewMessageList(20, 40)
	msg := store.Message{
		Media:    &store.MediaRef{Kind: store.MediaGIF},
		Document: &store.DocumentRef{ID: 777, ThumbSize: "m"},
	}
	id, ok := ml.PreviewImageID(msg)
	if !ok || id != 777 {
		t.Fatalf("GIF with thumb: got (id=%d, ok=%v), want (777, true)", id, ok)
	}
}

func TestPreviewImageID_GIFWithoutThumb(t *testing.T) {
	ml := NewMessageList(20, 40)
	msg := store.Message{
		Media:    &store.MediaRef{Kind: store.MediaGIF},
		Document: &store.DocumentRef{ID: 777}, // no ThumbSize
	}
	if _, ok := ml.PreviewImageID(msg); ok {
		t.Fatal("GIF without a thumb must have no inline preview")
	}
}

func TestLocalMediaLabel_Video(t *testing.T) {
	got := localMediaLabel(&store.LocalMedia{Kind: store.MediaVideo, FileName: "clip.mp4"})
	if got != "🎥 clip.mp4" {
		t.Fatalf("video label want '🎥 clip.mp4', got %q", got)
	}
}

func TestLocalMediaLabel_VideoNoName(t *testing.T) {
	got := localMediaLabel(&store.LocalMedia{Kind: store.MediaVideo})
	if got != "🎥 video" {
		t.Fatalf("nameless video label want '🎥 video', got %q", got)
	}
}

func TestPlaceholderFor_FileWithNameAndSize(t *testing.T) {
	got := placeholderFor(&store.MediaRef{Kind: store.MediaFile, FileName: "report.pdf", Size: 1300000})
	if !strings.Contains(got, "report.pdf") || !strings.Contains(got, "MB") {
		t.Fatalf("file placeholder want name + size, got %q", got)
	}
}

func TestPlaceholderFor_FileNoName(t *testing.T) {
	got := placeholderFor(&store.MediaRef{Kind: store.MediaFile})
	if got != "📎 file" {
		t.Fatalf("nameless file placeholder want '📎 file', got %q", got)
	}
}
