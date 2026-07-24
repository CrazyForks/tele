package components

import "testing"

func TestMosaicCols(t *testing.T) {
	cases := []struct{ n, want int }{
		{2, 2}, {3, 2}, {4, 2}, {5, 3}, {9, 3}, {1, 1},
	}
	for _, c := range cases {
		if got := mosaicCols(c.n); got != c.want {
			t.Fatalf("mosaicCols(%d) = %d, want %d", c.n, got, c.want)
		}
	}
}

func TestTileWidthsSumToContent(t *testing.T) {
	// cols-1 gaps of mosaicGap plus the tile widths must exactly fill contentW.
	for _, tc := range []struct{ contentW, cols int }{{40, 2}, {41, 2}, {40, 3}, {50, 3}} {
		ws := tileWidths(tc.contentW, tc.cols)
		if len(ws) != tc.cols {
			t.Fatalf("tileWidths(%d,%d) len = %d", tc.contentW, tc.cols, len(ws))
		}
		sum := (tc.cols - 1) * mosaicGap
		for _, w := range ws {
			sum += w
		}
		if sum != tc.contentW {
			t.Fatalf("tileWidths(%d,%d) = %v sum(+gaps)=%d, want %d", tc.contentW, tc.cols, ws, sum, tc.contentW)
		}
	}
}

func TestCoverWindowPortraitCropsVertically(t *testing.T) {
	// A mild portrait (natural 12 rows vs tile 10, ~17% crop, within the cap):
	// cover by width, crop top/bottom.
	g := coverWindow(1000, 1200, 20, 10)
	if g.Mode != tileCover {
		t.Fatalf("mode = %v, want cover", g.Mode)
	}
	if g.CoverCols != 20 {
		t.Fatalf("CoverCols = %d, want 20 (cover by width)", g.CoverCols)
	}
	if g.CoverRows < g.TileRows {
		t.Fatalf("CoverRows = %d must be >= TileRows %d", g.CoverRows, g.TileRows)
	}
	if g.VOff <= 0 || g.HOff != 0 {
		t.Fatalf("offsets = (%d,%d), want vertical-only centering", g.HOff, g.VOff)
	}
	if g.VOff+g.TileRows > g.CoverRows {
		t.Fatalf("window %d..%d exceeds cover rows %d", g.VOff, g.VOff+g.TileRows, g.CoverRows)
	}
}

func TestCoverWindowExtremePortraitLetterboxes(t *testing.T) {
	// A 1:6 sliver would crop far more than the cap: it must contain (letterbox).
	g := coverWindow(200, 1200, 20, 10)
	if g.Mode != tileContain {
		t.Fatalf("mode = %v, want contain (letterbox past the crop cap)", g.Mode)
	}
	if g.FitCols > g.TileW || g.FitRows > g.TileRows {
		t.Fatalf("fit box (%d,%d) exceeds tile (%d,%d)", g.FitCols, g.FitRows, g.TileW, g.TileRows)
	}
	if g.PadLeft < 0 || g.PadTop < 0 {
		t.Fatalf("negative padding (%d,%d)", g.PadLeft, g.PadTop)
	}
}

func TestCoverWindowSquareNoCrop(t *testing.T) {
	// An image already near the tile aspect neither crops nor pads much.
	g := coverWindow(1000, 500, 20, 10)
	if g.HOff != 0 || g.VOff != 0 {
		t.Fatalf("square-ish image offset = (%d,%d), want 0,0", g.HOff, g.VOff)
	}
}

func TestTileRowsForIsSquarish(t *testing.T) {
	ml := NewMessageList(40, 80)
	r := ml.tileRowsFor(20)
	if r < 2 || r > 20 {
		t.Fatalf("tileRowsFor(20) = %d, want a sane square-ish height in [2,20]", r)
	}
}

func TestMosaicUsesGrid(t *testing.T) {
	if !mosaicUsesGrid(2, mosaicMinTileCols) {
		t.Fatalf("two parts with a readable tile should grid")
	}
	if mosaicUsesGrid(1, 20) {
		t.Fatalf("a single previewable part must not grid")
	}
	if mosaicUsesGrid(4, mosaicMinTileCols-1) {
		t.Fatalf("a tile below the minimum width must not grid")
	}
}

func TestTransmitBoxByMode(t *testing.T) {
	cover := coverWindow(1000, 1200, 20, 10) // mild portrait -> cover
	if c, r := cover.transmitBox(); c != cover.CoverCols || r != cover.CoverRows {
		t.Fatalf("cover transmitBox = (%d,%d), want (%d,%d)", c, r, cover.CoverCols, cover.CoverRows)
	}
	contain := coverWindow(200, 1200, 20, 10) // extreme -> contain
	if c, r := contain.transmitBox(); c != contain.FitCols || r != contain.FitRows {
		t.Fatalf("contain transmitBox = (%d,%d), want (%d,%d)", c, r, contain.FitCols, contain.FitRows)
	}
}
