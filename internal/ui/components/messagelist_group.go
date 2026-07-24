package components

import (
	"strconv"

	"github.com/sorokin-vladimir/tele/internal/store"
)

// defaultAlbumImgW/H are the representative image dimensions used to size album
// previews before any part's bytes are cached. A 4:3 landscape keeps the
// pre-load reservation close to the typical photo footprint.
const (
	defaultAlbumImgW = 1024
	defaultAlbumImgH = 768
)

// groupMedia is one renderable/openable media part of an album, tagged with its
// 1-based display index. The index is stable across the bubble badges, the open
// picker labels, and the modal paging order.
type groupMedia struct {
	Index int
	Msg   store.Message
}

// hasRenderableMedia reports whether a message carries media the list draws
// inline: a photo or any document-backed media.
func hasRenderableMedia(msg store.Message) bool {
	return msg.Photo != nil || (msg.Media != nil && msg.Document != nil)
}

// groupMediaParts returns the album's media-bearing parts in order, numbered from
// 1. Text-only parts (an album can hold at most one caption-only message in
// practice, but guard anyway) are skipped.
func groupMediaParts(parts []store.Message) []groupMedia {
	out := make([]groupMedia, 0, len(parts))
	n := 0
	for _, p := range parts {
		if !hasRenderableMedia(p) {
			continue
		}
		n++
		out = append(out, groupMedia{Index: n, Msg: p})
	}
	return out
}

// albumContentW is the caption/text content width for an album bubble: the same
// three-quarter-viewport budget the single-message path uses.
func (ml *MessageList) albumContentW() int {
	w := ml.viewWidth*3/4 - 4
	if w < 4 {
		w = 4
	}
	return w
}

// albumImageRows returns the scaled art-row count for each image part of an
// album, chosen so the whole bubble — borders, one badge per part, any compact
// file rows, and the caption — fits the viewport height. A readability floor of 2
// rows applies. Both groupHeight and renderGroupBubble call this so they stay in
// lock-step.
func (ml *MessageList) albumImageRows(parts []store.Message) int {
	media := groupMediaParts(parts)
	imgCount := 0
	for _, gm := range media {
		if ml.albumPartHasPreview(gm.Msg) {
			imgCount++
		}
	}
	if imgCount == 0 {
		return 0
	}
	// borders + one badge per part + one blank line between adjacent parts. File
	// parts contribute only their badge (no preview rows), already counted here.
	overhead := 2 + len(media) + (len(media) - 1)
	if caption := albumCaption(parts); caption != "" {
		overhead += 1 + wrappedLineCount(caption, albumCaptionEntities(parts), ml.albumContentW())
	}
	budget := ml.viewHeight - overhead
	_, normal := ml.photoBox(defaultAlbumImgW, defaultAlbumImgH)
	rows := normal
	if per := budget / imgCount; per < rows {
		rows = per
	}
	if rows < 2 {
		rows = 2
	}
	return rows
}

// albumCaption returns the album's single caption. Telegram attaches the caption
// to one part (usually the first); return the first non-empty text in order.
func albumCaption(parts []store.Message) string {
	for _, p := range parts {
		if p.Text != "" {
			return p.Text
		}
	}
	return ""
}

// albumCaptionEntities returns the entities of the caption-bearing part so the
// caption wraps and styles identically to a normal message body.
func albumCaptionEntities(parts []store.Message) []store.MessageEntity {
	for _, p := range parts {
		if p.Text != "" {
			return p.Entities
		}
	}
	return nil
}

// albumPartHasPreview reports whether an album part draws an inline art preview
// (a photo, or any media whose thumbnail bytes are cached). Parts without a
// preview — generic files, or images not yet downloaded — are described entirely
// by their badge line and reserve no art rows.
func (ml *MessageList) albumPartHasPreview(msg store.Message) bool {
	if id, ok := ml.PreviewImageID(msg); ok {
		if _, has := ml.cachedImage(id); has {
			return true
		}
	}
	// A photo whose bytes are not cached yet still reserves a preview box so the
	// image can swap in at a stable height.
	return msg.Photo != nil
}

// albumBadgeText describes an album part next to its index: an icon plus type and
// context (a video's duration, a file's name and size), reusing the same labels
// the single-message placeholders use. Photos without a MediaRef fall back to a
// plain photo label rather than dereferencing a nil Media.
func albumBadgeText(msg store.Message) string {
	if msg.Media != nil {
		return placeholderFor(msg.Media)
	}
	if msg.Photo != nil {
		return "📷 photo"
	}
	return "📦 media"
}

// albumBadgeLabel is the full badge line content for a part: "[n] <type/context>".
func albumBadgeLabel(index int, msg store.Message) string {
	return "[" + strconv.Itoa(index) + "] " + albumBadgeText(msg)
}
