package services_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	_ "go.5x5.cz/inventario/internal/fileblob" // registers the file:// driver
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// avatarFixture is a tenant with one user and a local upload bucket.
type avatarFixture struct {
	factorySet *registry.FactorySet
	svc        *services.AvatarService
	location   string
	userID     string
}

func newAvatarFixture(c *qt.C) *avatarFixture {
	c.Helper()

	factorySet := memory.NewFactorySet()
	ctx := context.Background()
	const tenantID = "avatar-tenant"

	_, err := factorySet.TenantRegistry.Create(ctx, models.Tenant{
		EntityID: models.EntityID{ID: tenantID},
		Name:     "Avatar Tenant",
		Slug:     "avatar-tenant",
		Status:   models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	user, err := factorySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               "member@example.com",
		Name:                "Avatar Member",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	location := "file://" + c.TempDir() + "?create_dir=1"
	return &avatarFixture{
		factorySet: factorySet,
		svc:        services.NewAvatarService(factorySet, location),
		location:   location,
		userID:     user.ID,
	}
}

func (f *avatarFixture) avatarPath(c *qt.C) *string {
	c.Helper()
	user, err := f.factorySet.UserRegistry.Get(context.Background(), f.userID)
	c.Assert(err, qt.IsNil)
	return user.AvatarPath
}

func (f *avatarFixture) exists(c *qt.C, key string) bool {
	c.Helper()
	b, err := blob.OpenBucket(context.Background(), f.location)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	ok, err := b.Exists(context.Background(), key)
	c.Assert(err, qt.IsNil)
	return ok
}

// pngBytes encodes a w×h PNG, so the test uploads something that is not already
// the stored format.
func pngBytes(c *qt.C, w, h int) []byte {
	c.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 128, A: 255})
		}
	}
	var buf bytes.Buffer
	c.Assert(png.Encode(&buf, img), qt.IsNil)
	return buf.Bytes()
}

// Whatever arrives is stored as a square JPEG. That is not only about bandwidth:
// re-encoding is what drops the EXIF block a phone camera writes, GPS included.
func TestAvatarService_NormalisesWhatItStores(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)

	key, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(pngBytes(c, 1200, 800)))
	c.Assert(err, qt.IsNil)
	c.Assert(strings.HasPrefix(key, "t/avatar-tenant/avatars/"+f.userID+"/1-"), qt.IsTrue, qt.Commentf("got %s", key))
	c.Assert(strings.HasSuffix(key, ".jpg"), qt.IsTrue, qt.Commentf("got %s", key))
	c.Assert(f.avatarPath(c), qt.IsNotNil)
	c.Assert(*f.avatarPath(c), qt.Equals, key)

	reader, contentType, err := f.svc.Open(context.Background(), f.userID)
	c.Assert(err, qt.IsNil)
	defer reader.Close()
	c.Assert(contentType, qt.Equals, "image/jpeg")

	stored, err := io.ReadAll(reader)
	c.Assert(err, qt.IsNil)

	decoded, format, err := image.Decode(bytes.NewReader(stored))
	c.Assert(err, qt.IsNil)
	c.Assert(format, qt.Equals, "jpeg", qt.Commentf("a PNG went in; a JPEG has to come out"))
	c.Assert(decoded.Bounds().Dx(), qt.Equals, 512)
	c.Assert(decoded.Bounds().Dy(), qt.Equals, 512)
}

// A replacement is written under a new key, which is what makes a browser fetch
// the new picture without any cache-header plumbing, and the old object goes.
func TestAvatarService_ReplacementBumpsTheRevisionAndCleansUp(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)
	ctx := context.Background()

	first, err := f.svc.Store(ctx, f.userID, bytes.NewReader(pngBytes(c, 300, 300)))
	c.Assert(err, qt.IsNil)
	second, err := f.svc.Store(ctx, f.userID, bytes.NewReader(pngBytes(c, 300, 300)))
	c.Assert(err, qt.IsNil)

	c.Assert(second, qt.Not(qt.Equals), first)
	c.Assert(second, qt.Contains, "/2-")
	c.Assert(f.exists(c, second), qt.IsTrue)
	c.Assert(f.exists(c, first), qt.IsFalse, qt.Commentf("the replaced image was left behind"))

	third, err := f.svc.Store(ctx, f.userID, bytes.NewReader(pngBytes(c, 300, 300)))
	c.Assert(err, qt.IsNil)
	c.Assert(third, qt.Contains, "/3-")
}

// Removal clears the row and the object, and removing nothing is not an error —
// the frontend can call it without first checking.
func TestAvatarService_RemoveIsIdempotent(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)
	ctx := context.Background()

	c.Assert(f.svc.Remove(ctx, f.userID), qt.IsNil)

	key, err := f.svc.Store(ctx, f.userID, bytes.NewReader(pngBytes(c, 300, 300)))
	c.Assert(err, qt.IsNil)
	c.Assert(f.svc.Remove(ctx, f.userID), qt.IsNil)

	c.Assert(f.avatarPath(c), qt.IsNil)
	c.Assert(f.exists(c, key), qt.IsFalse)

	_, _, err = f.svc.Open(ctx, f.userID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	c.Assert(f.svc.Remove(ctx, f.userID), qt.IsNil)
}

// The type is decided by sniffing the bytes, so a file that claims to be an
// image and is not gets refused.
func TestAvatarService_RefusesWhatIsNotAnImage(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)

	for _, tc := range []struct {
		name string
		body []byte
	}{
		{"plain text", []byte(strings.Repeat("not an image at all. ", 100))},
		{"an SVG, which is markup that can carry script", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`)},
		{"a PDF", append([]byte("%PDF-1.7\n"), bytes.Repeat([]byte("x"), 600)...)},
		{"empty", nil},
	} {
		c.Run(tc.name, func(c *qt.C) {
			_, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(tc.body))
			c.Assert(err, qt.ErrorIs, services.ErrAvatarNotAnImage)
			c.Assert(f.avatarPath(c), qt.IsNil)
		})
	}
}

// A GIF is a real image that image.Decode would happily read. It is refused
// because the profile page promises JPG or PNG, which is the only thing the
// content sniff decides that the decoder would not.
func TestAvatarService_RefusesAFormatOutsideTheAllowlist(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)

	var buf bytes.Buffer
	c.Assert(gif.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 64, 64)), nil), qt.IsNil)

	_, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(buf.Bytes()))
	c.Assert(err, qt.ErrorIs, services.ErrAvatarNotAnImage)
	c.Assert(f.avatarPath(c), qt.IsNil)
}

// Sniffing only reads the first bytes, so a file with an image header and
// garbage after it passes the type check and must still be refused — by failing
// to decode.
func TestAvatarService_RefusesATruncatedImage(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)

	full := pngBytes(c, 400, 400)
	truncated := full[:len(full)/3]

	_, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(truncated))
	c.Assert(err, qt.ErrorIs, services.ErrAvatarNotAnImage)
	c.Assert(f.avatarPath(c), qt.IsNil)
}

// The cap is what stops a hostile body being read into memory. The boundary is
// worth pinning: a body exactly at the limit is a legitimate upload.
func TestAvatarService_SizeCapBoundary(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)
	limit := int(services.AvatarMaxUploadBytes)

	// A small PNG padded to an exact length. PNG decoding stops at IEND, so the
	// padding is ignored and the image stays decodable — which leaves the size
	// check as the only thing that can refuse it.
	body := pngBytes(c, 8, 8)

	c.Run("one byte over the limit is refused", func(c *qt.C) {
		padded := append(append([]byte(nil), body...), bytes.Repeat([]byte{0}, limit+1-len(body))...)
		c.Assert(padded, qt.HasLen, limit+1)
		_, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(padded))
		c.Assert(err, qt.ErrorIs, services.ErrAvatarTooLarge)
	})

	c.Run("exactly at the limit is accepted", func(c *qt.C) {
		// Trailing bytes after IEND are ignored, so this is a decodable image
		// of exactly the maximum size.
		padded := append(append([]byte(nil), body...), bytes.Repeat([]byte{0}, limit-len(body))...)
		c.Assert(padded, qt.HasLen, limit)
		_, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(padded))
		c.Assert(err, qt.IsNil)
	})
}

// The byte cap does not bound the decoded image: a small file of one flat colour
// can declare enormous dimensions, and decoding it allocates width x height x 4
// bytes before anything is scaled down. The header has to be read first.
func TestAvatarService_RefusesADecompressionBomb(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)

	// A PNG whose IHDR declares 30000x30000 (900 megapixels, ~3.6 GB decoded)
	// and which stops right after it. Go verifies the IHDR CRC, so the chunk has
	// to be well-formed — the point is that a VALID header with absurd
	// dimensions is refused before the pixels are allocated, which is also why
	// the truncation after it is never reached.
	ihdr := []byte{
		0x00, 0x00, 0x75, 0x30, // width  = 30000
		0x00, 0x00, 0x75, 0x30, // height = 30000
		0x08, 0x02, 0x00, 0x00, 0x00, // 8-bit truecolour
	}
	chunk := append([]byte("IHDR"), ihdr...)
	crc := crc32.ChecksumIEEE(chunk)

	body := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}
	body = append(body, 0x00, 0x00, 0x00, 0x0D) // IHDR length
	body = append(body, chunk...)
	crcBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(crcBytes, crc)
	body = append(body, crcBytes...)
	// Pad past the sniff length so the content is recognised as a PNG.
	body = append(body, bytes.Repeat([]byte{0}, 1024)...)

	_, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(body))
	c.Assert(err, qt.ErrorIs, services.ErrAvatarTooLarge,
		qt.Commentf("a declared 900-megapixel image was not refused from its header"))
	c.Assert(f.avatarPath(c), qt.IsNil)
}

// Two uploads racing for the same user must not land on the same key: both read
// the same revision, and without a per-upload component they would compute the
// same name and overwrite each other.
func TestAvatarService_ConcurrentUploadsGetDistinctKeys(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)
	body := pngBytes(c, 200, 200)

	const racers = 6
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		keys = map[string]int{}
	)
	for range racers {
		wg.Go(func() {
			key, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(body))
			mu.Lock()
			defer mu.Unlock()
			c.Check(err, qt.IsNil)
			keys[key]++
		})
	}
	wg.Wait()

	c.Assert(keys, qt.HasLen, racers,
		qt.Commentf("%d uploads produced %d distinct keys: %v", racers, len(keys), keys))

	// Whichever row update landed last decides, and it has to point at an object
	// that is actually there.
	final := f.avatarPath(c)
	c.Assert(final, qt.IsNotNil)
	c.Assert(f.exists(c, *final), qt.IsTrue)
}

// A JPEG carrying an EXIF block must not come out still carrying it: the block
// can hold the GPS coordinates of where the photo was taken, and group members
// can fetch this file.
func TestAvatarService_DropsExifMetadata(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)

	// A minimal JPEG with an APP1/Exif segment spliced in after SOI. The marker
	// is what an EXIF parser looks for, so its absence downstream is the check.
	var plain bytes.Buffer
	c.Assert(jpeg.Encode(&plain, image.NewRGBA(image.Rect(0, 0, 600, 600)), nil), qt.IsNil)
	original := plain.Bytes()

	exif := []byte("Exif\x00\x00II*\x00\x08\x00\x00\x00GPSDATA-51.5074-0.1278")
	segLen := len(exif) + 2
	c.Assert(segLen <= 0xFFFF, qt.IsTrue, qt.Commentf("the EXIF fixture does not fit one JPEG segment"))
	segment := append([]byte{0xFF, 0xE1, byte(segLen >> 8 & 0xFF), byte(segLen & 0xFF)}, exif...)
	withExif := append(append(append([]byte(nil), original[:2]...), segment...), original[2:]...)
	c.Assert(bytes.Contains(withExif, []byte("GPSDATA")), qt.IsTrue, qt.Commentf("the fixture itself has no EXIF payload"))

	_, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(withExif))
	c.Assert(err, qt.IsNil)

	reader, _, err := f.svc.Open(context.Background(), f.userID)
	c.Assert(err, qt.IsNil)
	defer reader.Close()
	stored, err := io.ReadAll(reader)
	c.Assert(err, qt.IsNil)

	c.Assert(bytes.Contains(stored, []byte("GPSDATA")), qt.IsFalse,
		qt.Commentf("the EXIF payload survived into the stored avatar"))
	c.Assert(bytes.Contains(stored, []byte("Exif")), qt.IsFalse)
}

// readFixture loads one of the images under testdata. They were written by real
// tools, not by this package, so a test against them checks the parsing against
// what a camera or an encoder produces:
//
//   - phone-photo.jpg: 256x256, red/green/blue/white quadrants (top-left,
//     top-right, bottom-left, bottom-right as stored), with an EXIF block
//     written by exiftool: Make=ExamplePhone, Orientation=6 and a GPS position.
//   - static.webp: a 96x64 lossy WebP from cwebp.
//   - animated.webp: two frames, from img2webp.
//   - animated.png: a two-frame APNG, from ImageMagick.
func readFixture(c *qt.C, name string) []byte {
	c.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	c.Assert(err, qt.IsNil)
	return raw
}

// storedAvatar uploads raw and returns the bytes that were stored.
func (f *avatarFixture) storedAvatar(c *qt.C, raw []byte) []byte {
	c.Helper()
	_, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(raw))
	c.Assert(err, qt.IsNil)
	reader, _, err := f.svc.Open(context.Background(), f.userID)
	c.Assert(err, qt.IsNil)
	defer reader.Close()
	stored, err := io.ReadAll(reader)
	c.Assert(err, qt.IsNil)
	return stored
}

// jpegAppSegments returns the marker of every APPn segment before the image
// data. EXIF is APP1; a JPEG with none of them carries no metadata block.
func jpegAppSegments(c *qt.C, raw []byte) []byte {
	c.Helper()
	c.Assert(len(raw) > 4 && raw[0] == 0xFF && raw[1] == 0xD8, qt.IsTrue, qt.Commentf("not a JPEG"))
	var markers []byte
	for i := 2; i+4 <= len(raw); {
		c.Assert(raw[i], qt.Equals, byte(0xFF), qt.Commentf("lost sync at %d", i))
		marker := raw[i+1]
		if marker == 0xDA {
			break
		}
		if marker >= 0xE1 && marker <= 0xEF {
			markers = append(markers, marker)
		}
		i += 2 + int(binary.BigEndian.Uint16(raw[i+2:]))
	}
	return markers
}

// A photo straight off a phone carries where it was taken. Group members can
// fetch the avatar, so the position must not survive the upload, and neither
// may anything else in the EXIF block.
func TestAvatarService_DropsTheGPSPositionOfAPhonePhoto(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)
	original := readFixture(c, "phone-photo.jpg")

	// The fixture really has what the test claims it removes: an APP1 EXIF
	// segment, the camera make, and a GPS IFD pointer (tag 0x8825).
	c.Assert(jpegAppSegments(c, original), qt.Contains, byte(0xE1))
	c.Assert(bytes.Contains(original, []byte("Exif\x00\x00")), qt.IsTrue)
	c.Assert(bytes.Contains(original, []byte("ExamplePhone")), qt.IsTrue)
	c.Assert(bytes.Contains(original, []byte{0x88, 0x25}) || bytes.Contains(original, []byte{0x25, 0x88}), qt.IsTrue)

	stored := f.storedAvatar(c, original)

	c.Assert(jpegAppSegments(c, stored), qt.HasLen, 0, qt.Commentf("an APPn metadata segment survived"))
	c.Assert(bytes.Contains(stored, []byte("Exif")), qt.IsFalse)
	c.Assert(bytes.Contains(stored, []byte("ExamplePhone")), qt.IsFalse)
}

// The same photo is stored sideways with Orientation=6, which a browser turns
// a quarter clockwise when it shows the original. The stored avatar has no tag
// left to turn by, so the pixels have to be turned already.
func TestAvatarService_AppliesTheExifOrientation(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)

	stored := f.storedAvatar(c, readFixture(c, "phone-photo.jpg"))
	img, _, err := image.Decode(bytes.NewReader(stored))
	c.Assert(err, qt.IsNil)
	c.Assert(img.Bounds().Dx(), qt.Equals, 256)

	// Turned a quarter clockwise, the stored bottom-left (blue) is now the
	// top-left, and so on round the picture.
	dominant := func(x, y int) string {
		r, g, b, _ := img.At(x, y).RGBA()
		switch {
		case r > 0xC000 && g > 0xC000 && b > 0xC000:
			return "white"
		case r > g && r > b:
			return "red"
		case g > r && g > b:
			return "green"
		default:
			return "blue"
		}
	}
	c.Assert(dominant(64, 64), qt.Equals, "blue")
	c.Assert(dominant(192, 64), qt.Equals, "red")
	c.Assert(dominant(64, 192), qt.Equals, "white")
	c.Assert(dominant(192, 192), qt.Equals, "green")
}

func TestAvatarService_AcceptsAStillWebP(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)

	stored := f.storedAvatar(c, readFixture(c, "static.webp"))
	img, format, err := image.Decode(bytes.NewReader(stored))
	c.Assert(err, qt.IsNil)
	c.Assert(format, qt.Equals, "jpeg")
	// 96x64 is cropped to its shorter side and not enlarged.
	c.Assert(img.Bounds().Dx(), qt.Equals, 64)
	c.Assert(img.Bounds().Dy(), qt.Equals, 64)
}

// Animated pictures are refused rather than flattened to their first frame:
// whoever uploads one expects it to move, and should hear that it will not.
func TestAvatarService_RefusesAnimation(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)

	c.Run("animated WebP", func(c *qt.C) {
		_, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(readFixture(c, "animated.webp")))
		c.Assert(err, qt.ErrorIs, services.ErrAvatarAnimated)
	})

	// An APNG sniffs as its own media type, so it is refused at the allowlist.
	c.Run("animated PNG", func(c *qt.C) {
		_, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(readFixture(c, "animated.png")))
		c.Assert(err, qt.ErrorIs, services.ErrAvatarNotAnImage)
	})

	c.Assert(f.avatarPath(c), qt.IsNil, qt.Commentf("a refused upload must not set an avatar"))
}

// A profile photo belongs to the user, not to a group, so it must not count
// toward any group's storage total. It is never written as a file row, which
// is what the storage total sums.
func TestAvatarService_WritesNoFileRow(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)

	_, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(pngBytes(c, 300, 300)))
	c.Assert(err, qt.IsNil)

	files := f.factorySet.FileRegistryFactory.CreateServiceRegistry()
	rows, err := files.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(rows, qt.HasLen, 0)

	usage, err := services.NewStorageUsageService(files).GetUsage(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(usage.UsedBytes, qt.Equals, int64(0))
}

func TestAvatarService_DeleteStored(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)

	key, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(pngBytes(c, 100, 100)))
	c.Assert(err, qt.IsNil)

	// An object outside the avatar prefix is left alone.
	b, err := blob.OpenBucket(context.Background(), f.location)
	c.Assert(err, qt.IsNil)
	c.Assert(b.WriteAll(context.Background(), "files/other.txt", []byte("x"), nil), qt.IsNil)
	c.Assert(b.Close(), qt.IsNil)
	f.svc.DeleteStored(context.Background(), "files/other.txt")
	c.Assert(f.exists(c, "files/other.txt"), qt.IsTrue)

	f.svc.DeleteStored(context.Background(), key)
	c.Assert(f.exists(c, key), qt.IsFalse)
}

// Photos uploaded before avatars were kept per tenant sit under a flat
// avatars/ key; deleting one works the same, and a tenant's other objects are
// left alone.
func TestAvatarService_DeleteStored_KeyLayouts(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)

	b, err := blob.OpenBucket(context.Background(), f.location)
	c.Assert(err, qt.IsNil)
	for _, key := range []string{"avatars/u1/1-abcdef012345.jpg", "t/avatar-tenant/files/other.txt"} {
		c.Assert(b.WriteAll(context.Background(), key, []byte("x"), nil), qt.IsNil)
	}
	c.Assert(b.Close(), qt.IsNil)

	f.svc.DeleteStored(context.Background(), "t/avatar-tenant/files/other.txt")
	c.Assert(f.exists(c, "t/avatar-tenant/files/other.txt"), qt.IsTrue)

	f.svc.DeleteStored(context.Background(), "avatars/u1/1-abcdef012345.jpg")
	c.Assert(f.exists(c, "avatars/u1/1-abcdef012345.jpg"), qt.IsFalse)
}

// A tenant move copies the photo into the new tenant's storage under the next
// revision, so its signed URL changes too, and leaves the original and the
// user row to the caller.
func TestAvatarService_CopyToTenant(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)
	ctx := context.Background()

	key, err := f.svc.Store(ctx, f.userID, bytes.NewReader(pngBytes(c, 100, 100)))
	c.Assert(err, qt.IsNil)
	user, err := f.factorySet.UserRegistry.Get(ctx, f.userID)
	c.Assert(err, qt.IsNil)

	copied, err := f.svc.CopyToTenant(ctx, user, "other-tenant")
	c.Assert(err, qt.IsNil)
	c.Assert(copied, qt.IsNotNil)
	c.Assert(strings.HasPrefix(*copied, "t/other-tenant/avatars/"+f.userID+"/2-"), qt.IsTrue, qt.Commentf("got %s", *copied))
	c.Assert(f.exists(c, *copied), qt.IsTrue)
	c.Assert(f.exists(c, key), qt.IsTrue)
	c.Assert(*f.avatarPath(c), qt.Equals, key)
}

func TestAvatarService_CopyToTenant_NoPhoto(t *testing.T) {
	c := qt.New(t)
	f := newAvatarFixture(c)
	user, err := f.factorySet.UserRegistry.Get(context.Background(), f.userID)
	c.Assert(err, qt.IsNil)

	copied, err := f.svc.CopyToTenant(context.Background(), user, "other-tenant")
	c.Assert(err, qt.IsNil)
	c.Assert(copied, qt.IsNil)
}

func TestAvatarVersion(t *testing.T) {
	c := qt.New(t)
	c.Assert(services.AvatarVersion("avatars/u1/3-a1b2c3d4e5f6.jpg"), qt.Equals, "3-a1b2c3d4e5f6")
}
