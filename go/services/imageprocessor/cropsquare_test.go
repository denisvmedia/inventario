package imageprocessor_test

import (
	"image"
	"image/color"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services/imageprocessor"
)

// fill returns a w×h image whose left half is red and right half is blue, so a
// crop's horizontal position is visible in the output.
func fill(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{R: 255, A: 255}
			if x >= w/2 {
				c = color.RGBA{B: 255, A: 255}
			}
			img.Set(x, y, c)
		}
	}
	return img
}

// An avatar is rendered in a square tile, so the crop has to be square whatever
// the source aspect ratio is.
func TestCropSquare_IsAlwaysSquare(t *testing.T) {
	c := qt.New(t)

	for _, tc := range []struct {
		name             string
		w, h, size, want int
	}{
		{"landscape", 1000, 400, 512, 400},
		{"portrait", 400, 1000, 512, 400},
		{"already square and larger", 800, 800, 512, 512},
		{"landscape larger than the target", 2000, 1500, 512, 512},
		{"smaller than the target is not upscaled", 64, 64, 512, 64},
		{"one pixel", 1, 1, 512, 1},
	} {
		c.Run(tc.name, func(c *qt.C) {
			out := imageprocessor.NewDefault().CropSquare(fill(tc.w, tc.h), tc.size)
			b := out.Bounds()
			c.Assert(b.Dx(), qt.Equals, tc.want)
			c.Assert(b.Dy(), qt.Equals, tc.want)
		})
	}
}

// The crop is centered, which is what makes a portrait photo usable as an
// avatar. A top-left crop would cut faces out of landscape photos.
func TestCropSquare_IsCentered(t *testing.T) {
	c := qt.New(t)

	// 400 wide, 100 tall: the square window is 100 wide, centered at x=150..250,
	// which lies entirely in the red left half (x < 200) only at its left edge.
	// Sampling both edges of the output shows where the window sat.
	out := imageprocessor.NewDefault().CropSquare(fill(400, 100), 100)
	left := out.At(out.Bounds().Min.X+2, out.Bounds().Min.Y+50)
	right := out.At(out.Bounds().Max.X-3, out.Bounds().Min.Y+50)

	lr, _, lb, _ := left.RGBA()
	rr, _, rb, _ := right.RGBA()
	c.Assert(lr > lb, qt.IsTrue, qt.Commentf("left edge should still be in the red half, got %v", left))
	c.Assert(rb > rr, qt.IsTrue, qt.Commentf("right edge should be in the blue half, got %v", right))
}

// A non-positive size is a caller bug rather than a request to produce a
// zero-sized image; returning the source keeps it from panicking in the scaler.
func TestCropSquare_NonPositiveSizeReturnsTheSource(t *testing.T) {
	c := qt.New(t)

	src := fill(10, 10)
	for _, size := range []int{0, -1} {
		c.Assert(imageprocessor.NewDefault().CropSquare(src, size), qt.Equals, src)
	}
}
