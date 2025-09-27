package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"math"
	"os"
	"path/filepath"
)

func main() {
	// Create output directory
	outputDir := "assets/placeholders"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("Error creating directory: %v\n", err)
		os.Exit(1)
	}

	// Generate small placeholder (150x150)
	if err := createAnimatedPlaceholder(150, filepath.Join(outputDir, "generating_small.gif")); err != nil {
		fmt.Printf("Error creating small placeholder: %v\n", err)
		os.Exit(1)
	}

	// Generate medium placeholder (300x300)
	if err := createAnimatedPlaceholder(300, filepath.Join(outputDir, "generating_medium.gif")); err != nil {
		fmt.Printf("Error creating medium placeholder: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Placeholder images generated successfully!")
	fmt.Println("Copy the 'assets' folder to your upload location.")
}

func createAnimatedPlaceholder(size int, filename string) error {
	const frames = 12 // Number of animation frames
	const delay = 8   // Delay between frames (in 1/100th of a second)

	// Create GIF structure
	anim := &gif.GIF{}

	// Generate frames for spinning loader
	for frame := 0; frame < frames; frame++ {
		// Create paletted image for GIF
		img := image.NewPaletted(image.Rect(0, 0, size, size), color.Palette{
			color.RGBA{240, 240, 240, 255}, // Background #f0f0f0
			color.RGBA{221, 221, 221, 255}, // Border #dddddd
			color.RGBA{102, 102, 102, 255}, // Loader #666666
			color.RGBA{153, 153, 153, 255}, // Loader light #999999
		})

		// Fill with background color
		draw.Draw(img, img.Bounds(), &image.Uniform{img.Palette[0]}, image.Point{}, draw.Src)

		// Draw border
		drawBorderPaletted(img, 1, 2) // Use palette index 1 for border

		// Calculate rotation angle for this frame
		angle := float64(frame) * 2 * math.Pi / float64(frames)

		// Draw rotating loader
		centerX := size / 2
		centerY := size / 2
		radius := size / 8 // Adjust loader size based on image size
		if radius < 12 {
			radius = 12
		}
		if radius > 24 {
			radius = 24
		}

		drawRotatingLoader(img, centerX, centerY, radius, angle, 2, 3) // Use palette indices 2,3

		// Add frame to animation
		anim.Image = append(anim.Image, img)
		anim.Delay = append(anim.Delay, delay)
	}

	// Save as animated GIF
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := gif.EncodeAll(file, anim); err != nil {
		return err
	}

	fmt.Printf("Created %s (%dx%d, %d frames)\n", filename, size, size, frames)
	return nil
}

func drawBorderPaletted(img *image.Paletted, colorIndex uint8, width int) {
	bounds := img.Bounds()

	// Top border
	for y := 0; y < width; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetColorIndex(x, y, colorIndex)
		}
	}

	// Bottom border
	for y := bounds.Max.Y - width; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			img.SetColorIndex(x, y, colorIndex)
		}
	}

	// Left border
	for x := 0; x < width; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			img.SetColorIndex(x, y, colorIndex)
		}
	}

	// Right border
	for x := bounds.Max.X - width; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			img.SetColorIndex(x, y, colorIndex)
		}
	}
}

func drawRotatingLoader(img *image.Paletted, centerX, centerY, radius int, rotation float64, darkIndex, lightIndex uint8) {
	// Draw rotating loader with multiple dots/lines
	const numDots = 8

	for i := 0; i < numDots; i++ {
		// Calculate angle for this dot
		angle := rotation + float64(i)*2*math.Pi/float64(numDots)

		// Calculate position
		dotRadius := float64(radius)
		x := centerX + int(dotRadius*math.Cos(angle))
		y := centerY + int(dotRadius*math.Sin(angle))

		// Determine opacity/color based on position (fade effect)
		// Dots closer to the "front" (angle 0) are darker
		normalizedAngle := math.Mod(angle-rotation, 2*math.Pi)
		if normalizedAngle < 0 {
			normalizedAngle += 2 * math.Pi
		}

		// Use different colors for fade effect
		var colorIndex uint8
		if normalizedAngle < math.Pi {
			colorIndex = darkIndex // Front dots are darker
		} else {
			colorIndex = lightIndex // Back dots are lighter
		}

		// Draw dot (small circle)
		dotSize := 2
		if radius > 20 {
			dotSize = 3
		}

		for dy := -dotSize; dy <= dotSize; dy++ {
			for dx := -dotSize; dx <= dotSize; dx++ {
				if dx*dx+dy*dy <= dotSize*dotSize {
					px, py := x+dx, y+dy
					if px >= 0 && px < img.Bounds().Max.X && py >= 0 && py < img.Bounds().Max.Y {
						img.SetColorIndex(px, py, colorIndex)
					}
				}
			}
		}
	}
}
