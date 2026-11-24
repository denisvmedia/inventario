package imageprocessor_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services/imageprocessor"
)

// plainJPEG encodes a small JPEG with no metadata.
func plainJPEG(c *qt.C) []byte {
	c.Helper()
	var buf bytes.Buffer
	c.Assert(jpeg.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil), qt.IsNil)
	return buf.Bytes()
}

// withExif splices an APP1 segment carrying a TIFF block with one IFD0 entry
// (tag, type, value) into a JPEG right after SOI.
func withExif(c *qt.C, order binary.AppendByteOrder, tag, typ, value uint16) []byte {
	c.Helper()
	tiff := make([]byte, 0, 32)
	if order == binary.LittleEndian {
		tiff = append(tiff, 'I', 'I')
	} else {
		tiff = append(tiff, 'M', 'M')
	}
	tiff = order.AppendUint16(tiff, 42)
	tiff = order.AppendUint32(tiff, 8) // IFD0 right after the header
	tiff = order.AppendUint16(tiff, 1) // one entry
	tiff = order.AppendUint16(tiff, tag)
	tiff = order.AppendUint16(tiff, typ)
	tiff = order.AppendUint32(tiff, 1)
	tiff = order.AppendUint16(tiff, value)
	tiff = append(tiff, 0, 0)          // value field padding
	tiff = order.AppendUint32(tiff, 0) // no next IFD

	// The APP1 length counts its own two bytes and the payload, which has a
	// fixed size: "Exif\0\0", the TIFF header, and an IFD with one entry.
	const app1Length = 2 + 6 + 8 + 2 + 12 + 4
	payload := append([]byte("Exif\x00\x00"), tiff...)
	c.Assert(len(payload)+2, qt.Equals, app1Length)
	segment := []byte{0xFF, 0xE1}
	segment = binary.BigEndian.AppendUint16(segment, app1Length)
	segment = append(segment, payload...)

	plain := plainJPEG(c)
	out := append([]byte(nil), plain[:2]...)
	out = append(out, segment...)
	return append(out, plain[2:]...)
}

// The fixture was written by exiftool with Orientation=6, the value a phone
// held upright records. Reading it back is the check that the parser agrees
// with a real EXIF writer, not only with the bytes the test builds itself.
func TestJPEGOrientation_ReadsWhatExiftoolWrote(t *testing.T) {
	c := qt.New(t)
	raw, err := os.ReadFile(filepath.Join("..", "testdata", "phone-photo.jpg"))
	c.Assert(err, qt.IsNil)
	c.Assert(imageprocessor.JPEGOrientation(raw), qt.Equals, imageprocessor.Orientation(6))
}

func TestJPEGOrientation(t *testing.T) {
	c := qt.New(t)
	cases := []struct {
		name string
		raw  func(c *qt.C) []byte
		want imageprocessor.Orientation
	}{
		{
			name: "no EXIF block",
			raw:  plainJPEG,
			want: imageprocessor.OrientationNormal,
		},
		{
			name: "little-endian block",
			raw:  func(c *qt.C) []byte { return withExif(c, binary.LittleEndian, 0x0112, 3, 8) },
			want: 8,
		},
		{
			name: "big-endian block",
			raw:  func(c *qt.C) []byte { return withExif(c, binary.BigEndian, 0x0112, 3, 3) },
			want: 3,
		},
		{
			name: "a block without the tag",
			raw:  func(c *qt.C) []byte { return withExif(c, binary.LittleEndian, 0x010F, 3, 6) },
			want: imageprocessor.OrientationNormal,
		},
		{
			name: "a value outside 1-8",
			raw:  func(c *qt.C) []byte { return withExif(c, binary.LittleEndian, 0x0112, 3, 9) },
			want: imageprocessor.OrientationNormal,
		},
		{
			name: "the tag with the wrong type",
			raw:  func(c *qt.C) []byte { return withExif(c, binary.LittleEndian, 0x0112, 4, 6) },
			want: imageprocessor.OrientationNormal,
		},
		{
			name: "a segment length past the end of the file",
			raw: func(c *qt.C) []byte {
				raw := withExif(c, binary.LittleEndian, 0x0112, 3, 6)
				return raw[:30]
			},
			want: imageprocessor.OrientationNormal,
		},
		{
			name: "not a JPEG",
			raw:  func(*qt.C) []byte { return []byte("\x89PNG\r\n\x1a\n") },
			want: imageprocessor.OrientationNormal,
		},
	}
	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(imageprocessor.JPEGOrientation(tc.raw(c)), qt.Equals, tc.want)
		})
	}
}

// Each stored pixel of a 3×2 image gets its own gray level, so the expected
// layouts below can be written as letters:
//
//	A B C
//	D E F
func TestOrient(t *testing.T) {
	c := qt.New(t)
	levels := map[byte]uint8{'A': 10, 'B': 40, 'C': 70, 'D': 100, 'E': 130, 'F': 160}
	src := image.NewGray(image.Rect(0, 0, 3, 2))
	for i, letter := range []byte("ABCDEF") {
		src.SetGray(i%3, i/3, color.Gray{Y: levels[letter]})
	}

	cases := []struct {
		orientation imageprocessor.Orientation
		want        []string
	}{
		{1, []string{"ABC", "DEF"}},
		{2, []string{"CBA", "FED"}},
		{3, []string{"FED", "CBA"}},
		{4, []string{"DEF", "ABC"}},
		{5, []string{"AD", "BE", "CF"}},
		{6, []string{"DA", "EB", "FC"}},
		{7, []string{"FC", "EB", "DA"}},
		{8, []string{"CF", "BE", "AD"}},
	}
	for _, tc := range cases {
		c.Run(fmt.Sprintf("orientation %d", tc.orientation), func(c *qt.C) {
			got := imageprocessor.Orient(src, tc.orientation)
			c.Assert(got.Bounds().Dy(), qt.Equals, len(tc.want))
			c.Assert(got.Bounds().Dx(), qt.Equals, len(tc.want[0]))
			for y, row := range tc.want {
				for x := range len(row) {
					gray := color.GrayModel.Convert(got.At(x, y)).(color.Gray)
					c.Assert(gray.Y, qt.Equals, levels[row[x]], qt.Commentf("pixel %d,%d", x, y))
				}
			}
		})
	}
}

func TestOrient_LeavesAnUnknownValueAlone(t *testing.T) {
	c := qt.New(t)
	src := image.NewGray(image.Rect(0, 0, 3, 2))
	c.Assert(imageprocessor.Orient(src, 0), qt.Equals, image.Image(src))
	c.Assert(imageprocessor.Orient(src, 9), qt.Equals, image.Image(src))
}
