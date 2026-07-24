package components

import (
	"strconv"
	"strings"

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
	imgCount, fileRows := 0, 0
	for _, gm := range media {
		if !ml.isImageMediaPart(gm.Msg) && gm.Msg.Document != nil {
			fileRows++
		} else {
			imgCount++
		}
	}
	if imgCount == 0 {
		return 0
	}
	overhead := 2 + len(media) + fileRows // borders + one badge per part + file rows
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

// isImageMediaPart reports whether an album part renders as inline art (photo,
// video thumbnail, animation) rather than a compact file row. Document media
// without a cached thumbnail (generic files) render as file rows.
func (ml *MessageList) isImageMediaPart(msg store.Message) bool {
	if msg.Photo != nil {
		return true
	}
	if id, ok := ml.PreviewImageID(msg); ok {
		if _, has := ml.cachedImage(id); has {
			return true
		}
	}
	return false
}

// groupFileRow renders one document album part as a single labelled row:
// the media placeholder glyph, the file name, and a human size when known.
func (ml *MessageList) groupFileRow(msg store.Message, m bubbleMetrics) string {
	name := ""
	if msg.Document != nil && msg.Document.FileName != "" {
		name = msg.Document.FileName
	} else if msg.Media != nil && msg.Media.FileName != "" {
		name = msg.Media.FileName
	}
	label := strings.TrimSpace(placeholderFor(msg.Media) + " " + name)
	if msg.Media != nil && msg.Media.Size > 0 {
		label += "  " + humanSize(msg.Media.Size)
	}
	return labelLine(label, m.actualW, m.b, m.bs)
}

// badgeRow renders the "[n]" index badge that precedes each album part, matching
// the labels used by the open picker.
func (ml *MessageList) badgeRow(index int, m bubbleMetrics) string {
	return labelLine("["+strconv.Itoa(index)+"]", m.actualW, m.b, m.bs)
}
