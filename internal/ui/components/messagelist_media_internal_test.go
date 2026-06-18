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
