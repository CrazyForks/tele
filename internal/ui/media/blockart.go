package media

import (
	"fmt"
	"image"
	"strings"

	colorful "github.com/lucasb-eyer/go-colorful"
	"github.com/nfnt/resize"
)

// RenderBlockArt scales img to cols columns wide and renders it as ANSI half-block art.
// Each returned string is one terminal line (no trailing newline).
// Two image rows map to one terminal row: top pixel → background, bottom pixel → foreground, char = '▄'.
// Odd image heights duplicate the last row.
func RenderBlockArt(img image.Image, cols int) []string {
	if cols <= 0 {
		return nil
	}
	b := img.Bounds()
	srcW, srcH := b.Dx(), b.Dy()
	if srcW == 0 || srcH == 0 {
		return nil
	}
	// Half-blocks (▄) give 2 image rows per terminal row, which already compensates
	// for the ~2:1 terminal cell aspect ratio — no additional scaling needed.
	targetH := uint(cols) * uint(srcH) / uint(srcW)
	if targetH == 0 {
		targetH = 2
	}
	scaled := resize.Resize(uint(cols), targetH, img, resize.Bilinear)
	sb := scaled.Bounds()
	width := sb.Dx()
	height := sb.Dy()

	termRows := (height + 1) / 2
	lines := make([]string, 0, termRows)

	for row := 0; row < termRows; row++ {
		y0 := sb.Min.Y + row*2
		y1 := y0 + 1
		if y1 >= sb.Min.Y+height {
			y1 = y0
		}

		var line strings.Builder
		for x := sb.Min.X; x < sb.Min.X+width; x++ {
			topC, _ := colorful.MakeColor(scaled.At(x, y0))
			botC, _ := colorful.MakeColor(scaled.At(x, y1))
			fmt.Fprintf(&line, "\x1b[48;2;%d;%d;%dm\x1b[38;2;%d;%d;%dm▄",
				clamp(int(topC.R*255), 0, 255),
				clamp(int(topC.G*255), 0, 255),
				clamp(int(topC.B*255), 0, 255),
				clamp(int(botC.R*255), 0, 255),
				clamp(int(botC.G*255), 0, 255),
				clamp(int(botC.B*255), 0, 255),
			)
		}
		line.WriteString("\x1b[0m")
		lines = append(lines, line.String())
	}
	return lines
}

// PhotoTermLines returns the number of terminal lines RenderBlockArt produces
// for an image of imgW×imgH pixels scaled to cols columns.
func PhotoTermLines(imgW, imgH, cols int) int {
	if imgW == 0 || cols == 0 {
		return 1
	}
	targetH := cols * imgH / imgW
	if targetH == 0 {
		targetH = 2
	}
	return (targetH + 1) / 2
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
