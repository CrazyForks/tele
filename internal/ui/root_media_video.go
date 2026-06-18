package ui

import (
	"context"
	"os"

	"github.com/gotd/td/tg"

	"github.com/sorokin-vladimir/tele/internal/media"
	internaltg "github.com/sorokin-vladimir/tele/internal/tg"
)

// videoBuildMediaCtx returns the context-aware media builder for an outbound
// video. path is the local source file (captured for ffprobe/ffmpeg). Inside the
// upload goroutine it probes duration/dimensions (ffprobe) and generates +
// uploads a thumbnail frame (ffmpeg). ffmpeg/ffprobe are optional: any missing
// binary or error degrades gracefully — we still send the video, just with zero
// metadata and/or no client thumbnail, and let Telegram fill the gaps
// server-side.
func videoBuildMediaCtx(path, name, mime string) func(ctx context.Context, client internaltg.Client, main tg.InputFileClass) (tg.InputMediaClass, error) {
	return func(ctx context.Context, client internaltg.Client, main tg.InputFileClass) (tg.InputMediaClass, error) {
		var meta media.VideoMeta
		if probed, err := media.ProbeVideo(ctx, path); err == nil {
			meta = probed
		}
		thumb := uploadVideoThumb(ctx, client, path)
		return internaltg.BuildInputMediaUploadedVideo(
			main, name, mime, meta.Duration, meta.Width, meta.Height, thumb,
		), nil
	}
}

// uploadVideoThumb extracts a thumbnail frame from the source video and uploads
// it, returning the uploaded InputFile or nil on any failure (no ffmpeg, extract
// error, or upload error) — the caller then sends without a client thumbnail.
func uploadVideoThumb(ctx context.Context, client internaltg.Client, path string) tg.InputFileClass {
	tmp, err := os.CreateTemp("", "tele-thumb-*.jpg")
	if err != nil {
		return nil
	}
	tmpPath := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := media.ExtractThumbnail(ctx, path, tmpPath); err != nil {
		return nil
	}
	thumb, err := client.UploadFile(ctx, internaltg.UploadParams{Path: tmpPath})
	if err != nil {
		return nil
	}
	return thumb
}
