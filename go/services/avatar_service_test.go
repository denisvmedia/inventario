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
	c.Assert(strings.HasPrefix(key, "avatars/"+f.userID+"/1-"), qt.IsTrue, qt.Commentf("got %s", key))
	c.Assert(strings.HasSuffix(key, ".jpg"), qt.IsTrue, qt.Commentf("got %s", key))
	c.Assert(f.avatarPath(c), qt.Not(qt.IsNil))
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
	c.Assert(strings.Contains(second, "/2-"), qt.IsTrue, qt.Commentf("got %s", second))
	c.Assert(f.exists(c, second), qt.IsTrue)
	c.Assert(f.exists(c, first), qt.IsFalse, qt.Commentf("the replaced image was left behind"))

	third, err := f.svc.Store(ctx, f.userID, bytes.NewReader(pngBytes(c, 300, 300)))
	c.Assert(err, qt.IsNil)
	c.Assert(strings.Contains(third, "/3-"), qt.IsTrue, qt.Commentf("got %s", third))
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
		c.Assert(len(padded), qt.Equals, limit+1)
		_, err := f.svc.Store(context.Background(), f.userID, bytes.NewReader(padded))
		c.Assert(err, qt.ErrorIs, services.ErrAvatarTooLarge)
	})

	c.Run("exactly at the limit is accepted", func(c *qt.C) {
		// Trailing bytes after IEND are ignored, so this is a decodable image
		// of exactly the maximum size.
		padded := append(append([]byte(nil), body...), bytes.Repeat([]byte{0}, limit-len(body))...)
		c.Assert(len(padded), qt.Equals, limit)
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

	c.Assert(len(keys), qt.Equals, racers,
		qt.Commentf("%d uploads produced %d distinct keys: %v", racers, len(keys), keys))

	// Whichever row update landed last decides, and it has to point at an object
	// that is actually there.
	final := f.avatarPath(c)
	c.Assert(final, qt.Not(qt.IsNil))
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
