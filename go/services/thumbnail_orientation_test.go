package services_test

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// jpegFile writes raw as a JPEG blob and creates its file row.
func (fx fileLinkFixture) jpegFile(c *qt.C, name string, raw []byte) *models.FileEntity {
	c.Helper()
	key := "files/" + name
	if raw != nil {
		fx.writeBlob(c, key, raw)
	}
	now := time.Now()
	file, err := fx.set.FileRegistry.Create(fx.ctx, models.FileEntity{
		Title:     name,
		Type:      models.FileTypeImage,
		Category:  models.FileCategoryImages,
		CreatedAt: now,
		UpdatedAt: now,
		File:      &models.File{Path: name, OriginalPath: key, Ext: ".jpg", MIMEType: "image/jpeg"},
	})
	c.Assert(err, qt.IsNil)
	return file
}

func (fx fileLinkFixture) writeBlob(c *qt.C, key string, raw []byte) {
	c.Helper()
	b, err := blob.OpenBucket(fx.ctx, fx.uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	c.Assert(b.WriteAll(fx.ctx, key, raw, nil), qt.IsNil)
}

func (fx fileLinkFixture) readBlob(c *qt.C, key string) []byte {
	c.Helper()
	b, err := blob.OpenBucket(fx.ctx, fx.uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	raw, err := b.ReadAll(fx.ctx, key)
	c.Assert(err, qt.IsNil)
	return raw
}

// writeUnturnedThumbnails stores thumbnails of raw's pixels as they are
// stored, without its EXIF orientation applied.
func (fx fileLinkFixture) writeUnturnedThumbnails(c *qt.C, svc *services.FileService, file *models.FileEntity, raw []byte) {
	c.Helper()
	img, err := jpeg.Decode(bytes.NewReader(raw))
	c.Assert(err, qt.IsNil)
	var buf bytes.Buffer
	c.Assert(jpeg.Encode(&buf, img, nil), qt.IsNil)
	for _, key := range svc.GetThumbnailPaths(file.TenantID, file.ID) {
		fx.writeBlob(c, key, buf.Bytes())
	}
}

// quadrants names the dominant color of each quarter of a JPEG: top-left,
// top-right, bottom-left, bottom-right.
func quadrants(c *qt.C, raw []byte) [4]string {
	c.Helper()
	img, err := jpeg.Decode(bytes.NewReader(raw))
	c.Assert(err, qt.IsNil)
	b := img.Bounds()
	at := func(x, y int) string { return dominantColor(img.At(b.Min.X+x, b.Min.Y+y)) }
	w, h := b.Dx(), b.Dy()
	return [4]string{at(w/4, h/4), at(3*w/4, h/4), at(w/4, 3*h/4), at(3*w/4, 3*h/4)}
}

func dominantColor(px color.Color) string {
	r, g, b, _ := px.RGBA()
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

// phone-photo.jpg stores red, green, blue and white quadrants (top-left,
// top-right, bottom-left, bottom-right) with Orientation=6: a browser turns
// it a quarter clockwise, so blue shows top-left.
var (
	storedQuadrants = [4]string{"red", "green", "blue", "white"}
	turnedQuadrants = [4]string{"blue", "red", "white", "green"}
)

func TestFileService_GenerateThumbnails_AppliesTheExifOrientation(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkFixture(c)
	svc := services.NewFileService(fx.factorySet, fx.uploadLocation)
	file := fx.jpegFile(c, "phone.jpg", readFixture(c, "phone-photo.jpg"))

	c.Assert(svc.GenerateThumbnails(fx.ctx, file), qt.IsNil)

	for size, key := range svc.GetThumbnailPaths(file.TenantID, file.ID) {
		c.Check(quadrants(c, fx.readBlob(c, key)), qt.Equals, turnedQuadrants, qt.Commentf("thumbnail %s", size))
	}
	got, err := fx.set.FileRegistry.Get(fx.ctx, file.ID)
	c.Assert(err, qt.IsNil)
	c.Check(got.ThumbnailsOriented, qt.IsTrue)
}

// Thumbnails written before the orientation was applied are made again for
// a turned photo; every JPEG that could be read is marked, and a second run
// changes nothing.
func TestBackfillThumbnailOrientation(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkFixture(c)
	svc := services.NewFileService(fx.factorySet, fx.uploadLocation)
	phone := readFixture(c, "phone-photo.jpg")

	turned := fx.jpegFile(c, "turned.jpg", phone)
	fx.writeUnturnedThumbnails(c, svc, turned, phone)
	noThumbnails := fx.jpegFile(c, "no-thumbnails.jpg", phone)

	var plain bytes.Buffer
	c.Assert(jpeg.Encode(&plain, image.NewRGBA(image.Rect(0, 0, 32, 16)), nil), qt.IsNil)
	upright := fx.jpegFile(c, "upright.jpg", plain.Bytes())
	fx.writeUnturnedThumbnails(c, svc, upright, plain.Bytes())
	uprightSmall := svc.GetThumbnailPaths(upright.TenantID, upright.ID)["small"]
	sentinel := []byte("not rewritten")
	fx.writeBlob(c, uprightSmall, sentinel)

	missing := fx.jpegFile(c, "missing.jpg", nil)

	c.Assert(quadrants(c, fx.readBlob(c, svc.GetThumbnailPaths(turned.TenantID, turned.ID)["small"])), qt.Equals, storedQuadrants)

	services.BackfillThumbnailOrientation(fx.ctx, fx.factorySet, fx.uploadLocation)

	turnedSmall := svc.GetThumbnailPaths(turned.TenantID, turned.ID)["small"]
	c.Check(quadrants(c, fx.readBlob(c, turnedSmall)), qt.Equals, turnedQuadrants)
	c.Check(fx.readBlob(c, uprightSmall), qt.DeepEquals, sentinel, qt.Commentf("an upright photo keeps its thumbnails"))
	for _, key := range svc.GetThumbnailPaths(noThumbnails.TenantID, noThumbnails.ID) {
		c.Check(fx.blobExists(c, key), qt.IsFalse, qt.Commentf("a photo without thumbnails gets them on its first view"))
	}

	wantMarked := map[string]bool{turned.ID: true, noThumbnails.ID: true, upright.ID: true, missing.ID: false}
	for id, want := range wantMarked {
		got, err := fx.set.FileRegistry.Get(fx.ctx, id)
		c.Assert(err, qt.IsNil)
		c.Check(got.ThumbnailsOriented, qt.Equals, want, qt.Commentf("file %s", got.Title))
	}

	fx.writeBlob(c, turnedSmall, sentinel)
	services.BackfillThumbnailOrientation(fx.ctx, fx.factorySet, fx.uploadLocation)
	c.Check(fx.readBlob(c, turnedSmall), qt.DeepEquals, sentinel, qt.Commentf("a second run writes nothing"))
}
