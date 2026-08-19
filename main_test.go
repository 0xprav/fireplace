package main

import (
	"image"
	"image/color"
	"image/gif"
	"testing"
)

func TestCompositeFramesRetainsPixelsUnderTransparency(t *testing.T) {
	bounds := image.Rect(0, 0, 2, 1)

	base := image.NewPaletted(bounds, color.Palette{
		color.RGBA{R: 255, A: 255},
	})
	overlay := image.NewPaletted(bounds, color.Palette{
		color.Transparent,
		color.RGBA{B: 255, A: 255},
	})
	overlay.Pix = []uint8{0, 1}

	frames := compositeFrames(&gif.GIF{
		Image:  []*image.Paletted{base, overlay},
		Config: image.Config{Width: 2, Height: 1},
	})

	if got := color.RGBAModel.Convert(frames[1].At(0, 0)); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("transparent pixel did not retain the prior frame: got %v", got)
	}
	if got := color.RGBAModel.Convert(frames[1].At(1, 0)); got != (color.RGBA{B: 255, A: 255}) {
		t.Fatalf("opaque pixel did not replace the prior frame: got %v", got)
	}
}
