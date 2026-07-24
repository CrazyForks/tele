package components

import (
	"github.com/sorokin-vladimir/tele/internal/store"
	"github.com/sorokin-vladimir/tele/internal/ui/media"
)

const (
	mosaicGap         = 1    // blank columns between tiles / blank rows between grid rows
	mosaicMinTileCols = 6    // fall back to the vertical stack below this tile width
	mosaicMaxCropFrac = 0.20 // max fraction of an axis a tile may crop before it letterboxes
)

// mosaicCols is the grid column count for nPreview previewable parts: 2 for up to
// four, 3 for five or more, never more than the number of parts.
func mosaicCols(nPreview int) int {
	c := 2
	if nPreview >= 5 {
		c = 3
	}
	if c > nPreview {
		c = nPreview
	}
	if c < 1 {
		c = 1
	}
	return c
}

// tileWidths splits contentW into cols tile widths with mosaicGap columns between
// them; the division remainder goes to the leftmost tiles so the row fills exactly.
func tileWidths(contentW, cols int) []int {
	if cols < 1 {
		return nil
	}
	avail := contentW - (cols-1)*mosaicGap
	if avail < cols {
		avail = cols // guard: at least 1 column per tile
	}
	base, rem := avail/cols, avail%cols
	ws := make([]int, cols)
	for i := range ws {
		ws[i] = base
		if i < rem {
			ws[i]++
		}
	}
	return ws
}

// tileRowsFor is the tile height (rows) that renders roughly square in pixels:
// tileW cols wide is tileW*cellW px, and tileRows*cellH px tall; square means
// tileRows = tileW*cellW/cellH. Floored at 2 rows.
func (ml *MessageList) tileRowsFor(tileW int) int {
	cw, ch := media.CellPx()
	if cw <= 0 || ch <= 0 {
		r := tileW / 2 // cells are ~2x taller than wide when the size is unknown
		if r < 2 {
			r = 2
		}
		return r
	}
	r := int(float64(tileW)*cw/ch + 0.5)
	if r < 2 {
		r = 2
	}
	return r
}

// mosaicUsesGrid reports whether an album should render as a grid rather than the
// vertical stack: at least two previewable parts and a tile wide enough to read.
func mosaicUsesGrid(nPreview, tileW0 int) bool {
	return nPreview >= 2 && tileW0 >= mosaicMinTileCols
}

type tileMode int

const (
	tileCover   tileMode = iota // fill the tile, crop the overflow (a centered window)
	tileContain                 // fit the whole image, pad the remainder (letterbox)
)

// tileGeom is one tile's render + transmit geometry. In cover mode the image is
// transmitted at CoverCols x CoverRows and the visible window is the centered
// TileW x TileRows sub-rectangle at (HOff, VOff). In contain mode the whole image
// is transmitted at FitCols x FitRows and centered in the tile with (PadLeft,
// PadTop) blank padding.
type tileGeom struct {
	TileW, TileRows                   int
	Mode                              tileMode
	CoverCols, CoverRows, HOff, VOff  int
	FitCols, FitRows, PadLeft, PadTop int
}

// transmitBox is the cell box the image is transmitted (and encoded) at.
func (g tileGeom) transmitBox() (cols, rows int) {
	if g.Mode == tileContain {
		return g.FitCols, g.FitRows
	}
	return g.CoverCols, g.CoverRows
}

// coverWindow computes how an imgW x imgH image fills a tileW x tileRows tile.
// It covers (crops the overflowing axis, centered) unless that would crop more
// than mosaicMaxCropFrac of the axis, in which case it contains (fits whole,
// centered, padded).
func coverWindow(imgW, imgH, tileW, tileRows int) tileGeom {
	aspect := media.CellAspect()
	natural := media.PhotoRows(imgW, imgH, tileW, aspect) // rows if scaled to tileW cols
	g := tileGeom{TileW: tileW, TileRows: tileRows, Mode: tileCover}

	switch {
	case natural >= tileRows:
		// Cover by width; crop vertically. Crop fraction = (natural-tileRows)/natural.
		if float64(natural-tileRows)/float64(natural) > mosaicMaxCropFrac {
			return containWindow(imgW, imgH, tileW, tileRows)
		}
		g.CoverCols, g.CoverRows = tileW, natural
		g.HOff, g.VOff = 0, (natural-tileRows)/2
	default:
		// Cover by height; crop horizontally. coverCols scales width up to fill.
		coverCols := int(float64(tileW)*float64(tileRows)/float64(natural) + 0.5)
		if coverCols < tileW {
			coverCols = tileW
		}
		if float64(coverCols-tileW)/float64(coverCols) > mosaicMaxCropFrac {
			return containWindow(imgW, imgH, tileW, tileRows)
		}
		g.CoverCols, g.CoverRows = coverCols, tileRows
		g.HOff, g.VOff = (coverCols-tileW)/2, 0
	}
	return g
}

// containWindow fits the whole image inside the tile (no crop), centered with
// blank padding on the letterboxed axis.
func containWindow(imgW, imgH, tileW, tileRows int) tileGeom {
	aspect := media.CellAspect()
	natural := media.PhotoRows(imgW, imgH, tileW, aspect)
	g := tileGeom{TileW: tileW, TileRows: tileRows, Mode: tileContain}
	if natural >= tileRows {
		// Fit by height: scale down so height == tileRows, width shrinks.
		g.FitRows = tileRows
		g.FitCols = int(float64(tileW)*float64(tileRows)/float64(natural) + 0.5)
		if g.FitCols < 1 {
			g.FitCols = 1
		}
	} else {
		g.FitCols, g.FitRows = tileW, natural
	}
	g.PadLeft = (tileW - g.FitCols) / 2
	g.PadTop = (tileRows - g.FitRows) / 2
	if g.PadLeft < 0 {
		g.PadLeft = 0
	}
	if g.PadTop < 0 {
		g.PadTop = 0
	}
	return g
}

// previewParts is the album's previewable parts (photos, thumbnailed video/GIF),
// in album order. Metadata-derived (albumPartReservesPreview), stable across load.
func (ml *MessageList) previewParts(parts []store.Message) []groupMedia {
	var out []groupMedia
	for _, gm := range groupMediaParts(parts) {
		if ml.albumPartReservesPreview(gm.Msg) {
			out = append(out, gm)
		}
	}
	return out
}

// mosaicContentW is the content width the album tiles are laid out in — the same
// three-quarter-viewport budget the album caption uses.
func (ml *MessageList) mosaicContentW() int {
	return ml.albumContentW()
}

// mosaicOverheadRows are the album's non-tile rows: two borders, one blank row
// between grid rows, a badge row per non-previewable file part, and the caption.
func (ml *MessageList) mosaicOverheadRows(parts []store.Message, nRows int) int {
	fileCount := 0
	for _, gm := range groupMediaParts(parts) {
		if !ml.albumPartReservesPreview(gm.Msg) {
			fileCount++
		}
	}
	h := 2 + (nRows - 1) + fileCount
	if c := albumCaption(parts); c != "" {
		h += 1 + wrappedLineCount(c, albumCaptionEntities(parts), ml.albumContentW())
	}
	return h
}

// mosaicTileRows is the shared tile height: the square-ish height, capped so the
// grid fits the viewport (mirrors the stack fill-pane logic).
func (ml *MessageList) mosaicTileRows(tileW, nRows, overheadRows int) int {
	square := ml.tileRowsFor(tileW)
	budget := (ml.viewHeight - overheadRows) / nRows
	if budget < 2 {
		budget = 2
	}
	if square < budget {
		return square
	}
	return budget
}

// albumTileGeom returns the tile geometry for the album part identified by
// partMsgID, or ok=false when the album does not grid or the part is not a
// previewable tile. Derived from metadata (previewable count, pane width, the
// part's grid index) plus the image's own dimensions, so it does not change as
// sibling parts load in.
func (ml *MessageList) albumTileGeom(parts []store.Message, partMsgID, imgW, imgH int) (tileGeom, bool) {
	prev := ml.previewParts(parts)
	cols := mosaicCols(len(prev))
	ws := tileWidths(ml.mosaicContentW(), cols)
	if len(ws) == 0 || !mosaicUsesGrid(len(prev), ws[len(ws)-1]) {
		return tileGeom{}, false
	}
	idx := -1
	for i, gm := range prev {
		if gm.Msg.ID == partMsgID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return tileGeom{}, false
	}
	nRows := (len(prev) + cols - 1) / cols
	overhead := ml.mosaicOverheadRows(parts, nRows)
	col := idx % cols
	tileW := ws[col]
	tileRows := ml.mosaicTileRows(minWidth(ws), nRows, overhead)
	return coverWindow(imgW, imgH, tileW, tileRows), true
}

// msgIDForPreviewID returns the message ID of the album part whose preview image
// is cached under id, or 0 if none.
func (ml *MessageList) msgIDForPreviewID(parts []store.Message, id int64) int {
	for _, p := range parts {
		if pid, ok := ml.PreviewImageID(p); ok && pid == id {
			return p.ID
		}
	}
	return 0
}

// minWidth returns the smallest tile column width; every tile uses one tileRows
// derived from the narrowest column so all tiles fit the shared budget.
func minWidth(ws []int) int {
	m := ws[0]
	for _, w := range ws {
		if w < m {
			m = w
		}
	}
	return m
}
