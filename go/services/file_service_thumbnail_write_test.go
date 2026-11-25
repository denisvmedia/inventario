package services_test

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"runtime"
	"testing"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	_ "go.5x5.cz/inventario/internal/fileblob" // Register file driver
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// A thumbnail whose blob cannot be stored is an error, so the job that asked
// for it fails instead of completing with a missing thumbnail. A directory
// at the thumbnail's key makes the driver's final rename fail on Close.
func TestFileService_GenerateThumbnails_FailedSaveIsAnError(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	dir := c.TempDir()
	uploadLocation := "file://" + dir + "?create_dir=1"
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + dir + "?create_dir=1"
	}
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	img := image.NewRGBA(image.Rect(0, 0, 40, 30))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	w, err := b.NewWriter(ctx, "photo.png", nil)
	c.Assert(err, qt.IsNil)
	c.Assert(png.Encode(w, img), qt.IsNil)
	c.Assert(w.Close(), qt.IsNil)

	service := services.NewFileService(memory.NewFactorySet(), uploadLocation)
	file := &models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: "file-1"},
			TenantID: "tenant-1",
		},
		Type: models.FileTypeImage,
		File: &models.File{Path: "photo", OriginalPath: "photo.png", Ext: ".png", MIMEType: "image/png"},
	}

	large := service.GetThumbnailPaths(file.TenantID, file.ID)["large"]
	c.Assert(b.WriteAll(ctx, large+"/blocker", []byte("x"), nil), qt.IsNil)

	err = service.GenerateThumbnails(ctx, file)
	c.Assert(err, qt.ErrorMatches, `.*failed to save thumbnail.*`)
}
