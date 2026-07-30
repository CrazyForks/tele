package ui

// The client decodes the media the owner hands it as a file, so the decoders
// have to be registered here rather than in internal/tg (#196). Losing one of
// these imports compiles cleanly and fails at runtime: image.Decode answers
// "unknown format" and that media silently stops rendering.
// TestFetchStickerCmd_DecodesAWebpFile guards the WEBP one.
import (
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)
