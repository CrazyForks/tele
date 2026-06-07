package media_test

import (
	"image"
	"image/color"
	"testing"

	"github.com/sorokin-vladimir/tele/internal/ui/media"
	"github.com/stretchr/testify/require"
)

func TestCircleCrop_CornersTransparentCenterOpaque(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 20, 20))
	for y := 0; y < 20; y++ {
		for x := 0; x < 20; x++ {
			src.Set(x, y, color.RGBA{R: 255, A: 255})
		}
	}

	out := media.CircleCrop(src)

	require.Equal(t, src.Bounds().Dx(), out.Bounds().Dx())
	require.Equal(t, src.Bounds().Dy(), out.Bounds().Dy())

	_, _, _, aCorner := out.At(0, 0).RGBA()
	require.Equal(t, uint32(0), aCorner, "corner must be transparent")

	_, _, _, aCenter := out.At(10, 10).RGBA()
	require.Greater(t, aCenter, uint32(0), "center must stay opaque")
}
