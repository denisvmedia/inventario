package imageprocessor

import (
	"bytes"
	"encoding/binary"
	"image"
)

// Orientation is the EXIF orientation tag (TIFF tag 0x0112): how the stored
// pixels have to be turned to show the picture the right way up. A phone camera
// records the sensor's pixels as they came and writes this tag instead of
// rotating them, and every browser applies it when it displays the file.
//
// Re-encoding a decoded image drops the tag along with the rest of the EXIF
// block, so code that re-encodes has to apply it to the pixels first, or a photo
// taken in portrait comes out lying on its side.
type Orientation int

const (
	// OrientationNormal means the pixels are stored the right way up.
	OrientationNormal Orientation = 1
	// orientationMax is the highest value the tag defines.
	orientationMax Orientation = 8

	exifTagOrientation = 0x0112
	tiffTypeShort      = 3
)

var exifHeader = []byte("Exif\x00\x00")

// JPEGOrientation reads the EXIF orientation of a JPEG. It answers
// OrientationNormal for anything it cannot read: no EXIF block, a block without
// the tag, a value outside 1–8, or a malformed file. Turning a picture the wrong
// way is worse than leaving it as stored, so an unreadable tag is ignored rather
// than guessed at.
func JPEGOrientation(raw []byte) Orientation {
	if len(raw) < 4 || raw[0] != 0xFF || raw[1] != 0xD8 {
		return OrientationNormal
	}
	for i := 2; i+4 <= len(raw); {
		if raw[i] != 0xFF {
			return OrientationNormal
		}
		marker := raw[i+1]
		switch {
		case marker == 0xFF:
			// Fill byte before a marker.
			i++
			continue
		case marker == 0x01 || (marker >= 0xD0 && marker <= 0xD7):
			// Markers without a length field.
			i += 2
			continue
		case marker == 0xDA || marker == 0xD9:
			// Start of scan or end of image: EXIF lives before the image data.
			return OrientationNormal
		}
		segmentLen := int(binary.BigEndian.Uint16(raw[i+2:]))
		end := i + 2 + segmentLen
		if segmentLen < 2 || end > len(raw) {
			return OrientationNormal
		}
		if payload := raw[i+4 : end]; marker == 0xE1 && bytes.HasPrefix(payload, exifHeader) {
			return tiffOrientation(payload[len(exifHeader):])
		}
		i = end
	}
	return OrientationNormal
}

// tiffOrientation reads the orientation tag from IFD0 of a TIFF structure, the
// format an EXIF block uses.
func tiffOrientation(tiff []byte) Orientation {
	if len(tiff) < 8 {
		return OrientationNormal
	}
	var order binary.ByteOrder
	switch string(tiff[:2]) {
	case "II":
		order = binary.LittleEndian
	case "MM":
		order = binary.BigEndian
	default:
		return OrientationNormal
	}
	if order.Uint16(tiff[2:]) != 42 {
		return OrientationNormal
	}

	size := uint64(len(tiff))
	ifd := uint64(order.Uint32(tiff[4:]))
	if ifd+2 > size {
		return OrientationNormal
	}
	entries := uint64(order.Uint16(tiff[ifd:]))
	for k := range entries {
		entry := ifd + 2 + k*12
		if entry+12 > size {
			return OrientationNormal
		}
		if order.Uint16(tiff[entry:]) != exifTagOrientation {
			continue
		}
		// A single SHORT sits in the first two bytes of the value field.
		if order.Uint16(tiff[entry+2:]) != tiffTypeShort {
			return OrientationNormal
		}
		value := Orientation(order.Uint16(tiff[entry+8:]))
		if value < OrientationNormal || value > orientationMax {
			return OrientationNormal
		}
		return value
	}
	return OrientationNormal
}

// Orient returns src turned the way o says, so that it displays the right way
// up without the tag. Orientations 5 to 8 swap the width and the height.
// OrientationNormal, or a value outside 1–8, returns src unchanged.
func Orient(src image.Image, o Orientation) image.Image {
	if o <= OrientationNormal || o > orientationMax {
		return src
	}
	bounds := src.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	outW, outH := w, h
	if o >= 5 {
		outW, outH = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	for y := range outH {
		for x := range outW {
			sx, sy := orientedSource(o, x, y, w, h)
			dst.Set(x, y, src.At(bounds.Min.X+sx, bounds.Min.Y+sy))
		}
	}
	return dst
}

// orientedSource maps a pixel of the turned image back to the stored pixel it
// shows. w and h are the stored image's dimensions.
func orientedSource(o Orientation, x, y, w, h int) (sx, sy int) {
	switch o {
	case 2: // mirrored left to right
		return w - 1 - x, y
	case 3: // turned 180°
		return w - 1 - x, h - 1 - y
	case 4: // mirrored top to bottom
		return x, h - 1 - y
	case 5: // mirrored along the top-left to bottom-right diagonal
		return y, x
	case 6: // needs a quarter turn clockwise
		return y, h - 1 - x
	case 7: // mirrored along the other diagonal
		return w - 1 - y, h - 1 - x
	case 8: // needs a quarter turn counter-clockwise
		return w - 1 - y, x
	default:
		return x, y
	}
}
