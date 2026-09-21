package imageprocessor_test

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"golang.org/x/image/draw"

	"go.5x5.cz/inventario/services/imageprocessor"
)

// solid builds an opaque image of the given size. The content does not matter
// to any assertion here — every one of them is about geometry or encoding.
func solid(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	return img
}

// The long side lands on maxSize and the short side follows it. Getting this
// backwards produces thumbnails that overflow their grid cell in one
// dimension, which is the kind of thing that looks like a CSS bug.
func TestCreateThumbnail_ScalesTheLongSide(t *testing.T) {
	c := qt.New(t)
	p := imageprocessor.NewDefault()

	landscape := p.CreateThumbnail(solid(800, 400), 200)
	c.Assert(landscape.Bounds().Dx(), qt.Equals, 200)
	c.Assert(landscape.Bounds().Dy(), qt.Equals, 100)

	portrait := p.CreateThumbnail(solid(400, 800), 200)
	c.Assert(portrait.Bounds().Dx(), qt.Equals, 100)
	c.Assert(portrait.Bounds().Dy(), qt.Equals, 200)

	square := p.CreateThumbnail(solid(500, 500), 250)
	c.Assert(square.Bounds().Dx(), qt.Equals, 250)
	c.Assert(square.Bounds().Dy(), qt.Equals, 250)
}

// Upscaling a small image would cost memory and add nothing — the source is
// returned as-is, not a resampled copy of the same size.
func TestCreateThumbnail_LeavesASmallImageAlone(t *testing.T) {
	c := qt.New(t)
	p := imageprocessor.NewDefault()

	src := solid(100, 80)
	got := p.CreateThumbnail(src, 200)
	c.Assert(got, qt.Equals, src)

	// Exactly at the limit counts as small enough.
	atLimit := solid(200, 150)
	c.Assert(p.CreateThumbnail(atLimit, 200), qt.Equals, atLimit)
}

// Thumbnails are JPEG whatever the source was, so the browser gets one format
// and the stored bytes stay small.
func TestSaveThumbnail_WritesJPEGFromAPNGSource(t *testing.T) {
	c := qt.New(t)
	p := imageprocessor.NewDefault()

	// Round-trip through PNG first so the source really is a decoded PNG
	// rather than the in-memory RGBA we built.
	pngPath := filepath.Join(t.TempDir(), "source.png")
	pngFile, err := os.Create(pngPath)
	c.Assert(err, qt.IsNil)
	c.Assert(png.Encode(pngFile, solid(600, 300)), qt.IsNil)
	c.Assert(pngFile.Close(), qt.IsNil)

	pngFile, err = os.Open(pngPath)
	c.Assert(err, qt.IsNil)
	defer pngFile.Close()
	src, format, err := image.Decode(pngFile)
	c.Assert(err, qt.IsNil)
	c.Assert(format, qt.Equals, "png")

	out := filepath.Join(t.TempDir(), "thumb.jpg")
	c.Assert(p.SaveThumbnail(src, 150, out), qt.IsNil)

	written, err := os.Open(out)
	c.Assert(err, qt.IsNil)
	defer written.Close()
	decoded, format, err := image.Decode(written)
	c.Assert(err, qt.IsNil)
	c.Assert(format, qt.Equals, "jpeg")
	c.Assert(decoded.Bounds().Dx(), qt.Equals, 150)
	c.Assert(decoded.Bounds().Dy(), qt.Equals, 75)
}

func TestSaveThumbnail_ReportsAnUnwritablePath(t *testing.T) {
	c := qt.New(t)
	p := imageprocessor.NewDefault()

	// A directory that does not exist: the worker treats the error as a
	// retryable job failure, so it has to come back rather than be swallowed.
	err := p.SaveThumbnail(solid(400, 400), 100, filepath.Join(t.TempDir(), "nope", "thumb.jpg"))
	c.Assert(err, qt.IsNotNil)
}

// New takes the scaler so a caller can trade quality for speed; the default is
// the high-quality one. Both have to produce a correctly-sized image.
func TestNew_HonorsTheSuppliedScaler(t *testing.T) {
	c := qt.New(t)

	fast := imageprocessor.New(draw.NearestNeighbor)
	got := fast.CreateThumbnail(solid(400, 200), 100)
	c.Assert(got.Bounds().Dx(), qt.Equals, 100)
	c.Assert(got.Bounds().Dy(), qt.Equals, 50)
}

func TestSaveThumbnail_EncodesAValidJPEGFile(t *testing.T) {
	c := qt.New(t)
	p := imageprocessor.NewDefault()

	out := filepath.Join(t.TempDir(), "thumb.jpg")
	c.Assert(p.SaveThumbnail(solid(1000, 500), 200, out), qt.IsNil)

	f, err := os.Open(out)
	c.Assert(err, qt.IsNil)
	defer f.Close()
	// jpeg.Decode rather than image.Decode: this asserts the encoder chose
	// JPEG, not merely that something decodable was written.
	img, err := jpeg.Decode(f)
	c.Assert(err, qt.IsNil)
	c.Assert(img.Bounds().Dx(), qt.Equals, 200)

	info, err := os.Stat(out)
	c.Assert(err, qt.IsNil)
	c.Assert(info.Size() > 0, qt.IsTrue)
}
