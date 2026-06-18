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
