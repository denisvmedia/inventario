package imageprocessor

import (
	"image"
	"image/jpeg"
	"os"

	"golang.org/x/image/draw"
)

// ImageProcessor is a service that provides image processing functionality.
type ImageProcessor struct {
	scaler draw.Scaler
}

// New creates a new ImageProcessor with the given scaler algorithm.
func New(algo draw.Scaler) *ImageProcessor {
	return &ImageProcessor{
		scaler: algo,
	}
}

// NewDefault creates a new ImageProcessor with the default scaler algorithm.
func NewDefault() *ImageProcessor {
	return New(draw.CatmullRom)
}

// CreateThumbnail creates a thumbnail of the given image with the given maximum size.
func (p *ImageProcessor) CreateThumbnail(src image.Image, maxSize int) image.Image {
	srcBounds := src.Bounds()
	srcW, srcH := srcBounds.Dx(), srcBounds.Dy()

	// Detect scale factor
	scale := float64(maxSize) / float64(srcW)
	if srcH > srcW {
		scale = float64(maxSize) / float64(srcH)
	}

	// If image is already smaller than max size
	if scale >= 1.0 {
		return src
	}

	newW := int(float64(srcW) * scale)
	newH := int(float64(srcH) * scale)

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	p.scaler.Scale(dst, dst.Bounds(), src, srcBounds, draw.Over, nil)

	return dst
}

// SaveThumbnail creates a thumbnail of the given image and saves it to the given file.
// All thumbnails are saved as JPEG files regardless of the original format.
func (p *ImageProcessor) SaveThumbnail(src image.Image, maxSize int, filename string) error {
	img := p.CreateThumbnail(src, maxSize)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Always save thumbnails as JPEG for consistency and smaller file sizes
	return jpeg.Encode(file, img, &jpeg.Options{Quality: 90})
}
