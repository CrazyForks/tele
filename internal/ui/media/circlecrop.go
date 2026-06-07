package media

import (
	"image"
	"image/color"
)

// CircleCrop returns an RGBA copy of src with everything outside the largest
// inscribed circle made fully transparent. Used to render round video messages
// (кружки) as circles; transparency is composited by the terminal over the
// bubble background (Kitty graphics).
func CircleCrop(src image.Image) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	out := image.NewRGBA(image.Rect(0, 0, w, h))

	d := w
	if h < d {
		d = h
	}
	r := float64(d) / 2
	cx := float64(w) / 2
	cy := float64(h) / 2

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			if dx*dx+dy*dy <= r*r {
				out.Set(x, y, src.At(b.Min.X+x, b.Min.Y+y))
			} else {
				out.Set(x, y, color.RGBA{})
			}
		}
	}
	return out
}
