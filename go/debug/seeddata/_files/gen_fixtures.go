//go:build ignore

// gen_fixtures regenerates the small PDF fixtures bundled with the
// seed package, and (only when explicitly asked) synthetic colored-
// swatch placeholders for the photo-*.jpg slots. Run with
// `go run gen_fixtures.go` from this directory; the produced files
// are committed to the repo and loaded at seed time via //go:embed.
//
// Outputs by default:
//
//	invoice.pdf — minimal 1-page receipt PDF
//	manual.pdf  — minimal 1-page manual PDF
//
// Photos (photo-*.jpg) ship as real Pexels images — see SOURCES.md.
// To overwrite them with licensing-clean synthetic swatches (smaller
// but visually bland), pass `-photos` on the command line.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
)

type swatch struct {
	name string
	tint color.RGBA
}

func main() {
	overwritePhotos := flag.Bool("photos", false,
		"overwrite the committed photo-*.jpg files with synthetic colored swatches "+
			"(disabled by default to preserve the real Pexels photos — see SOURCES.md)")
	flag.Parse()

	if *overwritePhotos {
		swatches := []swatch{
			{"photo-livingroom.jpg", color.RGBA{R: 180, G: 120, B: 90, A: 255}}, // warm sofa
			{"photo-kitchen.jpg", color.RGBA{R: 200, G: 200, B: 215, A: 255}},   // muted steel
			{"photo-work.jpg", color.RGBA{R: 60, G: 80, B: 130, A: 255}},        // navy work
			{"photo-outdoor.jpg", color.RGBA{R: 95, G: 130, B: 90, A: 255}},     // moss green
			{"photo-bedroom.jpg", color.RGBA{R: 150, G: 140, B: 170, A: 255}},   // lavender
			{"photo-storage.jpg", color.RGBA{R: 120, G: 110, B: 100, A: 255}},   // taupe
		}

		for _, s := range swatches {
			if err := writeSwatchJPG(s.name, s.tint); err != nil {
				fmt.Fprintln(os.Stderr, "write", s.name, ":", err)
				os.Exit(1)
			}
			fmt.Println("wrote", s.name)
		}
	} else {
		fmt.Println("skipping photo-*.jpg (committed Pexels photos preserved; pass -photos to overwrite)")
	}

	if err := os.WriteFile("invoice.pdf", minimalPDF("INVOICE", "Item .......... 1,299.99 USD\nTax ............    99.99 USD\nTOTAL ......... 1,399.98 USD"), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "invoice:", err)
		os.Exit(1)
	}
	fmt.Println("wrote invoice.pdf")

	if err := os.WriteFile("manual.pdf", minimalPDF("OWNER MANUAL", "1. Plug into AC outlet.\n2. Press POWER for 3 seconds.\n3. Refer to safety leaflet."), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "manual:", err)
		os.Exit(1)
	}
	fmt.Println("wrote manual.pdf")
}

// writeSwatchJPG produces a 320x240 JPG with a subtle gradient over `tint`
// so the demo grid doesn't look like flat color tiles.
func writeSwatchJPG(name string, tint color.RGBA) error {
	const w, h = 320, 240
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		// Vertical gradient: top is darker, bottom is lighter.
		shift := int16(y*40/h) - 20
		r := clamp(int16(tint.R) + shift)
		g := clamp(int16(tint.G) + shift)
		b := clamp(int16(tint.B) + shift)
		for x := 0; x < w; x++ {
			// Horizontal noise for non-uniform-ness.
			j := int16((x ^ y) & 0x07)
			img.Set(x, y, color.RGBA{
				R: clamp(int16(r) - j),
				G: clamp(int16(g) - j),
				B: clamp(int16(b) - j),
				A: 255,
			})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 75}); err != nil {
		return err
	}
	return os.WriteFile(name, buf.Bytes(), 0o644)
}

func clamp(v int16) uint8 {
	switch {
	case v < 0:
		return 0
	case v > 255:
		return 255
	default:
		return uint8(v)
	}
}

// minimalPDF returns a hand-rolled 1-page PDF carrying the given title +
// body. Real PDFs are wildly more complex; we only need viewers to open
// it and render something plausible. The byte stream is constructed by
// hand because we don't want to depend on a PDF library just for the
// seed.
func minimalPDF(title, body string) []byte {
	// Build content stream: title at top, body lines below.
	var content bytes.Buffer
	content.WriteString("BT\n/F1 18 Tf\n72 720 Td\n(")
	content.WriteString(pdfEscape(title))
	content.WriteString(") Tj\nET\n")

	yPos := 690
	for _, line := range splitLines(body) {
		fmt.Fprintf(&content, "BT\n/F1 11 Tf\n72 %d Td\n(", yPos)
		content.WriteString(pdfEscape(line))
		content.WriteString(") Tj\nET\n")
		yPos -= 18
	}

	stream := content.Bytes()

	// Now assemble the PDF.
	var pdf bytes.Buffer
	offsets := []int{0, 0, 0, 0, 0, 0}

	pdf.WriteString("%PDF-1.4\n%\xff\xff\xff\xff\n")

	offsets[1] = pdf.Len()
	pdf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	offsets[2] = pdf.Len()
	pdf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	offsets[3] = pdf.Len()
	pdf.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>\nendobj\n")

	offsets[4] = pdf.Len()
	pdf.WriteString("4 0 obj\n<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>\nendobj\n")

	offsets[5] = pdf.Len()
	fmt.Fprintf(&pdf, "5 0 obj\n<< /Length %d >>\nstream\n", len(stream))
	pdf.Write(stream)
	pdf.WriteString("endstream\nendobj\n")

	xrefOffset := pdf.Len()
	pdf.WriteString("xref\n0 6\n0000000000 65535 f \n")
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", offsets[i])
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)
	return pdf.Bytes()
}

func pdfEscape(s string) string {
	var b bytes.Buffer
	for _, r := range s {
		switch r {
		case '(':
			b.WriteString(`\(`)
		case ')':
			b.WriteString(`\)`)
		case '\\':
			b.WriteString(`\\`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func splitLines(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}
