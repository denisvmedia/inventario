package services

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"runtime"
	"testing"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // Register file driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// newTestContext creates a context with test user for testing
func newTestContext() context.Context {
	// Create a test user with generated UUID
	testUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-" + generateTestID()},
			TenantID: "test-tenant-id",
		},
	}
	return appctx.WithUser(context.Background(), testUser)
}

// generateTestID generates a simple test ID
func generateTestID() string {
	return "12345678-1234-1234-1234-123456789012" // Fixed UUID for consistent testing
}

func TestFileService_DeleteFileWithPhysical(t *testing.T) {
	c := qt.New(t)
	ctx := newTestContext()

	// Create temporary directory for test files
	tempDir := c.TempDir()
	var uploadLocation string
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	} else {
		uploadLocation = "file://" + tempDir + "?create_dir=1"
	}

	factorySet := memory.NewFactorySet()
	registrySet, err := factorySet.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)

	// Create file service
	service := NewFileService(factorySet, uploadLocation)

	// Create a test physical file
	testFilePath := "test-file.txt"
	testContent := []byte("test content")

	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	err = b.WriteAll(ctx, testFilePath, testContent, nil)
	c.Assert(err, qt.IsNil)

	// Verify file exists
	exists, err := b.Exists(ctx, testFilePath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)

	// Create file entity
	fileEntity := models.FileEntity{
		Title:       "Test File",
		Description: "Test description",
		Type:        models.FileTypeDocument,
		Tags:        []string{"test"},
		File: &models.File{
			Path:         "test-file",
			OriginalPath: testFilePath,
			Ext:          ".txt",
			MIMEType:     "text/plain",
		},
	}

	createdFile, err := registrySet.FileRegistry.Create(ctx, fileEntity)
	c.Assert(err, qt.IsNil)

	// Test successful deletion
	err = service.DeleteFileWithPhysical(ctx, createdFile.ID)
	c.Assert(err, qt.IsNil)

	// Verify file entity is deleted
	_, err = registrySet.FileRegistry.Get(ctx, createdFile.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify physical file is deleted
	exists, err = b.Exists(ctx, testFilePath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
}

// TestFileService_SharedBlobKey_DeleteKeepsTheOtherRowsBytes is the #2241
// regression test, and it is about the rows ALREADY IN DEPLOYED DATABASES.
//
// New uploads carry a UUID and cannot collide. The rows written before that
// fix can, forever: their key is `<sanitized-name>-<unix SECONDS><ext>`, so two
// members of two different groups who uploaded `receipt.jpg` in the same second
// have two DISTINCT file rows pointing at ONE blob. No code change can
// un-collide them.
//
// Every blob delete goes by key, and `files` has no soft-delete and no trash, so
// before the guard, deleting either row destroyed the other, still-live file's
// bytes — irreversibly, with the surviving row left pointing at nothing.
//
// The blob may only die with its LAST owner.
func TestFileService_SharedBlobKey_DeleteKeepsTheOtherRowsBytes(t *testing.T) {
	c := qt.New(t)
	ctx := newTestContext()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	}

	factorySet := memory.NewFactorySet()
	registrySet, err := factorySet.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)
	service := NewFileService(factorySet, uploadLocation)

	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	// The one blob both rows point at — a legacy, second-granularity key.
	const shared = "t/test-tenant-id/files/receipt-1783824560.jpg"
	c.Assert(b.WriteAll(ctx, shared, []byte("the only copy of the user's receipt"), nil), qt.IsNil)

	mk := func(title string) *models.FileEntity {
		c.Helper()
		f, err := registrySet.FileRegistry.Create(ctx, models.FileEntity{
			Title: title,
			Type:  models.FileTypeImage,
			File: &models.File{
				Path: title, OriginalPath: shared, Ext: ".jpg", MIMEType: "image/jpeg",
			},
		})
		c.Assert(err, qt.IsNil)
		return f
	}
	first := mk("receipt-a")
	second := mk("receipt-b")

	// Deleting the FIRST row must not touch the bytes: the second row is still
	// live and still points at them.
	c.Assert(service.DeleteFileWithPhysical(ctx, first.ID), qt.IsNil)

	_, err = registrySet.FileRegistry.Get(ctx, first.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	exists, err := b.Exists(ctx, shared)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue,
		qt.Commentf("deleting one row destroyed the other live file's only copy — this is #2241"))

	// And the survivor is not merely a row: its bytes are readable.
	got, err := b.ReadAll(ctx, shared)
	c.Assert(err, qt.IsNil)
	c.Assert(string(got), qt.Equals, "the only copy of the user's receipt")

	// The LAST owner takes the blob with it — the guard must not leak the blob
	// forever, it must only defer.
	c.Assert(service.DeleteFileWithPhysical(ctx, second.ID), qt.IsNil)

	exists, err = b.Exists(ctx, shared)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse,
		qt.Commentf("the last owner is gone, so nothing references the blob and it must not be leaked"))
}

// A group purge sweeps blobs while the rows it is purging are STILL IN THE
// TABLE (the GroupPurger removes them afterwards). So the shared-key guard has
// to distinguish "this key is referenced by a row I am deleting" from "this key
// is referenced by somebody else's row" — and get it right in BOTH directions:
//
//   - a purged group's own blobs must actually be cleaned up (a naive
//     "is anyone pointing at this?" check sees the row itself and leaks
//     every blob in the installation, forever);
//   - a blob shared with a row in ANOTHER group must survive the purge.
func TestFileService_DeletePhysicalFilesForGroup_SharedBlobKey(t *testing.T) {
	c := qt.New(t)
	ctx := newTestContext()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	}

	factorySet := memory.NewFactorySet()
	service := NewFileService(factorySet, uploadLocation)
	fileReg := factorySet.FileRegistryFactory.CreateServiceRegistry()

	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	const doomed = "t/tenant-1/files/doomed-1783824560.jpg"
	const shared = "t/tenant-1/files/receipt-1783824560.jpg"
	c.Assert(b.WriteAll(ctx, doomed, []byte("purge me"), nil), qt.IsNil)
	c.Assert(b.WriteAll(ctx, shared, []byte("keep me"), nil), qt.IsNil)

	mk := func(groupID, key string) {
		c.Helper()
		_, err := fileReg.Create(ctx, models.FileEntity{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID: "tenant-1", GroupID: groupID, CreatedByUserID: "user-1",
			},
			Title: "f", Type: models.FileTypeImage,
			File: &models.File{Path: "f", OriginalPath: key, Ext: ".jpg", MIMEType: "image/jpeg"},
		})
		c.Assert(err, qt.IsNil)
	}
	mk("group-1", doomed) // only group-1 points at this
	mk("group-1", shared) // ...and group-1 shares this one with group-2
	mk("group-2", shared)

	c.Assert(service.DeletePhysicalFilesForGroup(ctx, "tenant-1", "group-1"), qt.IsNil)

	exists, err := b.Exists(ctx, doomed)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse,
		qt.Commentf("the purged group's own blob was leaked — the guard mistook the row for somebody else's"))

	exists, err = b.Exists(ctx, shared)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue,
		qt.Commentf("purging group-1 destroyed group-2's live file bytes"))
}

func TestFileService_DeleteFileWithPhysical_FileNotFound(t *testing.T) {
	c := qt.New(t)
	ctx := newTestContext()

	// Create temporary directory for test files
	tempDir := c.TempDir()
	var uploadLocation string
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	} else {
		uploadLocation = "file://" + tempDir + "?create_dir=1"
	}

	factorySet := memory.NewFactorySet()

	// Create file service
	service := NewFileService(factorySet, uploadLocation)

	// Test deletion of non-existent file entity
	err := service.DeleteFileWithPhysical(ctx, "non-existent-id")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestFileService_DeletePhysicalFile(t *testing.T) {
	c := qt.New(t)
	ctx := newTestContext()

	// Create temporary directory for test files
	tempDir := c.TempDir()
	var uploadLocation string
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	} else {
		uploadLocation = "file://" + tempDir + "?create_dir=1"
	}

	// Create test registry set
	factorySet := memory.NewFactorySet()

	// Create file service
	service := NewFileService(factorySet, uploadLocation)

	// Create a test physical file
	testFilePath := "test-physical-file.txt"
	testContent := []byte("test content")

	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	err = b.WriteAll(ctx, testFilePath, testContent, nil)
	c.Assert(err, qt.IsNil)

	// Verify file exists
	exists, err := b.Exists(ctx, testFilePath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)

	// Test successful physical file deletion
	err = service.DeletePhysicalFile(ctx, testFilePath)
	c.Assert(err, qt.IsNil)

	// Verify physical file is deleted
	exists, err = b.Exists(ctx, testFilePath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
}

func TestFileService_DeletePhysicalFile_NonExistent(t *testing.T) {
	c := qt.New(t)
	ctx := newTestContext()

	// Create temporary directory for test files
	tempDir := c.TempDir()
	var uploadLocation string
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	} else {
		uploadLocation = "file://" + tempDir + "?create_dir=1"
	}

	// Create test registry set
	factorySet := memory.NewFactorySet()

	// Create file service
	service := NewFileService(factorySet, uploadLocation)

	// Test deletion of non-existent physical file (should not error)
	err := service.DeletePhysicalFile(ctx, "non-existent-file.txt")
	c.Assert(err, qt.IsNil)
}

func TestFileService_DeleteLinkedFiles(t *testing.T) {
	c := qt.New(t)
	ctx := newTestContext()

	// Create temporary directory for test files
	tempDir := c.TempDir()
	var uploadLocation string
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	} else {
		uploadLocation = "file://" + tempDir + "?create_dir=1"
	}

	// Create test registry set
	factorySet := memory.NewFactorySet()
	registrySet, err := factorySet.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)

	// Create file service
	service := NewFileService(factorySet, uploadLocation)

	// Create test physical files
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	testFiles := []string{"linked-file-1.txt", "linked-file-2.txt"}
	for _, filePath := range testFiles {
		err = b.WriteAll(ctx, filePath, []byte("test content"), nil)
		c.Assert(err, qt.IsNil)
	}

	// Create linked file entities
	entityType := "commodity"
	entityID := "test-commodity-id"
	var createdFiles []*models.FileEntity

	for _, filePath := range testFiles {
		fileEntity := models.FileEntity{
			Title:            "Test File",
			Description:      "Test description",
			Type:             models.FileTypeDocument,
			Tags:             []string{"test"},
			LinkedEntityType: entityType,
			LinkedEntityID:   entityID,
			LinkedEntityMeta: "images",
			File: &models.File{
				Path:         filepath.Base(filePath),
				OriginalPath: filePath,
				Ext:          ".txt",
				MIMEType:     "text/plain",
			},
		}

		createdFile, err := registrySet.FileRegistry.Create(ctx, fileEntity)
		c.Assert(err, qt.IsNil)
		createdFiles = append(createdFiles, createdFile)
	}

	// Test deletion of linked files
	err = service.DeleteLinkedFiles(ctx, entityType, entityID)
	c.Assert(err, qt.IsNil)

	// Verify all file entities are deleted
	for _, file := range createdFiles {
		_, err = registrySet.FileRegistry.Get(ctx, file.ID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	}

	// Verify all physical files are deleted
	for _, filePath := range testFiles {
		exists, err := b.Exists(ctx, filePath)
		c.Assert(err, qt.IsNil)
		c.Assert(exists, qt.IsFalse)
	}
}

// TestFileService_DeleteLinkedFiles_AreaAndLocation pins #2119 at the
// primitive level: DeleteLinkedFiles handles the 'area' and 'location' link
// types exactly like 'commodity' — file rows AND physical blobs are removed.
// The area case uses an image MIME so the canonical thumbnail-key cleanup is
// exercised for these link types too.
func TestFileService_DeleteLinkedFiles_AreaAndLocation(t *testing.T) {
	tests := []struct {
		name       string
		entityType string
		mimeType   string
		ext        string
	}{
		{
			name:       "area-linked image file",
			entityType: "area",
			mimeType:   "image/jpeg",
			ext:        ".jpg",
		},
		{
			name:       "location-linked document file",
			entityType: "location",
			mimeType:   "application/pdf",
			ext:        ".pdf",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := newTestContext()

			tempDir := c.TempDir()
			var uploadLocation string
			if runtime.GOOS == "windows" {
				uploadLocation = "file:///" + tempDir + "?create_dir=1"
			} else {
				uploadLocation = "file://" + tempDir + "?create_dir=1"
			}

			factorySet := memory.NewFactorySet()
			registrySet, err := factorySet.CreateUserRegistrySet(ctx)
			c.Assert(err, qt.IsNil)

			service := NewFileService(factorySet, uploadLocation)

			b, err := blob.OpenBucket(ctx, uploadLocation)
			c.Assert(err, qt.IsNil)
			defer b.Close()

			blobKey := "linked-" + tt.entityType + tt.ext
			c.Assert(b.WriteAll(ctx, blobKey, []byte("test content"), nil), qt.IsNil)

			entityID := "test-" + tt.entityType + "-id"
			createdFile, err := registrySet.FileRegistry.Create(ctx, models.FileEntity{
				Title:            "Linked File",
				Type:             models.FileTypeFromMIME(tt.mimeType),
				LinkedEntityType: tt.entityType,
				LinkedEntityID:   entityID,
				LinkedEntityMeta: "images",
				File: &models.File{
					Path:         "linked-" + tt.entityType,
					OriginalPath: blobKey,
					Ext:          tt.ext,
					MIMEType:     tt.mimeType,
				},
			})
			c.Assert(err, qt.IsNil)

			// For the image case, pre-write the canonical thumbnail blobs the
			// file would own so their cleanup is asserted too.
			thumbnailPaths := service.GetThumbnailPaths(createdFile.TenantID, createdFile.ID)
			if tt.mimeType == "image/jpeg" {
				for _, p := range thumbnailPaths {
					c.Assert(b.WriteAll(ctx, p, []byte("thumb"), nil), qt.IsNil)
				}
			}

			err = service.DeleteLinkedFiles(ctx, tt.entityType, entityID)
			c.Assert(err, qt.IsNil)

			// File row is gone.
			_, err = registrySet.FileRegistry.Get(ctx, createdFile.ID)
			c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

			// Physical blob is gone.
			exists, err := b.Exists(ctx, blobKey)
			c.Assert(err, qt.IsNil)
			c.Assert(exists, qt.IsFalse)

			// Thumbnail blobs are gone for the image case.
			if tt.mimeType == "image/jpeg" {
				for size, p := range thumbnailPaths {
					exists, err := b.Exists(ctx, p)
					c.Assert(err, qt.IsNil)
					c.Assert(exists, qt.IsFalse, qt.Commentf("thumbnail %s at %s should be deleted", size, p))
				}
			}
		})
	}
}

func TestFileService_DeleteLinkedFiles_NoFiles(t *testing.T) {
	c := qt.New(t)
	ctx := newTestContext()

	// Create temporary directory for test files
	tempDir := c.TempDir()
	var uploadLocation string
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	} else {
		uploadLocation = "file://" + tempDir + "?create_dir=1"
	}

	// Create test registry set
	factorySet := memory.NewFactorySet()

	// Create file service
	service := NewFileService(factorySet, uploadLocation)

	// Test deletion of linked files when no files exist (should not error)
	err := service.DeleteLinkedFiles(ctx, "commodity", "non-existent-id")
	c.Assert(err, qt.IsNil)
}

func TestFileService_ExportFileDeletion_Integration(t *testing.T) {
	c := qt.New(t)
	ctx := newTestContext()

	// Create temporary directory for test files
	tempDir := c.TempDir()
	var uploadLocation string
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	} else {
		uploadLocation = "file://" + tempDir + "?create_dir=1"
	}

	// Create test registry set
	factorySet := memory.NewFactorySet()
	registrySet, err := factorySet.CreateUserRegistrySet(ctx)
	c.Assert(err, qt.IsNil)

	// Create file service
	service := NewFileService(factorySet, uploadLocation)

	// Create a test physical file
	testFilePath := "export-test-file.xml"
	testContent := []byte("<export>test export content</export>")

	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	err = b.WriteAll(ctx, testFilePath, testContent, nil)
	c.Assert(err, qt.IsNil)

	// Verify file exists
	exists, err := b.Exists(ctx, testFilePath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)

	// Create file entity linked to an export
	fileEntity := models.FileEntity{
		Title:            "Test Export File",
		Description:      "Test export file description",
		Type:             models.FileTypeDocument,
		Tags:             []string{"export"},
		LinkedEntityType: "export",
		LinkedEntityID:   "test-export-id",
		LinkedEntityMeta: "xml-1.0",
		File: &models.File{
			Path:         "export-test-file",
			OriginalPath: testFilePath,
			Ext:          ".xml",
			MIMEType:     "application/xml",
		},
	}

	createdFile, err := registrySet.FileRegistry.Create(ctx, fileEntity)
	c.Assert(err, qt.IsNil)

	// Create export entity with file reference
	exportEntity := models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusCompleted,
		Description: "Test export",
		FileID:      &createdFile.ID,
	}

	createdExport, err := registrySet.ExportRegistry.Create(ctx, exportEntity)
	c.Assert(err, qt.IsNil)

	// Test that both export and file exist before deletion
	_, err = registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)

	_, err = registrySet.FileRegistry.Get(ctx, createdFile.ID)
	c.Assert(err, qt.IsNil)

	exists, err = b.Exists(ctx, testFilePath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)

	// Delete the file using the service (simulating what happens when export is deleted)
	err = service.DeleteFileWithPhysical(ctx, createdFile.ID)
	c.Assert(err, qt.IsNil)

	// Verify file entity is deleted
	_, err = registrySet.FileRegistry.Get(ctx, createdFile.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Verify physical file is deleted
	exists, err = b.Exists(ctx, testFilePath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)

	// Export should still exist (it's the service layer's responsibility to delete it)
	_, err = registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)
}

// createTestImage creates a test image with the given dimensions and color
func createTestImage(width, height int, c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestFileService_GenerateThumbnails(t *testing.T) {
	c := qt.New(t)
	ctx := newTestContext()

	// Create temporary directory for test files
	tempDir := c.TempDir()
	var uploadLocation string
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	} else {
		uploadLocation = "file://" + tempDir + "?create_dir=1"
	}

	// Create test registry set
	factorySet := memory.NewFactorySet()

	// Create file service
	service := NewFileService(factorySet, uploadLocation)

	tests := []struct {
		name                     string
		mimeType                 string
		filename                 string
		shouldGenerateThumbnails bool
	}{
		{
			name:                     "PNG image should generate thumbnails",
			mimeType:                 "image/png",
			filename:                 "test-image.png",
			shouldGenerateThumbnails: true,
		},
		{
			name:                     "JPEG image should generate thumbnails",
			mimeType:                 "image/jpeg",
			filename:                 "test-image.jpg",
			shouldGenerateThumbnails: true,
		},
		{
			name:                     "PDF should not generate thumbnails",
			mimeType:                 "application/pdf",
			filename:                 "test-document.pdf",
			shouldGenerateThumbnails: false,
		},
		{
			name:                     "WebP image should not generate thumbnails",
			mimeType:                 "image/webp",
			filename:                 "test-webp-image.webp",
			shouldGenerateThumbnails: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			b, err := blob.OpenBucket(ctx, uploadLocation)
			c.Assert(err, qt.IsNil)
			defer b.Close()

			if tt.shouldGenerateThumbnails {
				// Create a test image
				testImg := createTestImage(400, 300, color.RGBA{R: 255, G: 0, B: 0, A: 255})

				// Save the test image
				writer, err := b.NewWriter(ctx, tt.filename, nil)
				c.Assert(err, qt.IsNil)

				// Encode based on the expected format
				if tt.mimeType == "image/jpeg" {
					err = jpeg.Encode(writer, testImg, &jpeg.Options{Quality: 90})
				} else {
					err = png.Encode(writer, testImg)
				}
				c.Assert(err, qt.IsNil)
				writer.Close()
			} else {
				// Create a non-image file
				err = b.WriteAll(ctx, tt.filename, []byte("test content"), nil)
				c.Assert(err, qt.IsNil)
			}

			// Create a file entity for testing
			fileEntity := &models.FileEntity{
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
					EntityID: models.EntityID{ID: "test-file-" + tt.name},
					TenantID: "test-tenant",
				},
				Type: models.FileTypeImage,
				File: &models.File{
					Path:         tt.filename,
					OriginalPath: tt.filename,
					Ext:          ".jpg",
					MIMEType:     tt.mimeType,
				},
			}

			// Test thumbnail generation
			err = service.GenerateThumbnails(ctx, fileEntity)
			c.Assert(err, qt.IsNil)

			if tt.shouldGenerateThumbnails {
				// Check that thumbnails were created
				thumbnailPaths := service.GetThumbnailPaths(fileEntity.TenantID, fileEntity.ID)
				c.Assert(thumbnailPaths, qt.HasLen, 2) // small and medium

				for sizeName, thumbnailPath := range thumbnailPaths {
					exists, err := b.Exists(ctx, thumbnailPath)
					c.Assert(err, qt.IsNil)
					c.Assert(exists, qt.IsTrue, qt.Commentf("Thumbnail %s should exist at %s", sizeName, thumbnailPath))
				}
			} else {
				// Check that no thumbnails were created
				thumbnailPaths := service.GetThumbnailPaths(fileEntity.TenantID, fileEntity.ID)
				for _, thumbnailPath := range thumbnailPaths {
					exists, err := b.Exists(ctx, thumbnailPath)
					c.Assert(err, qt.IsNil)
					c.Assert(exists, qt.IsFalse, qt.Commentf("Thumbnail should not exist at %s", thumbnailPath))
				}
			}
		})
	}
}

func TestFileService_GetThumbnailPaths(t *testing.T) {
	// Create test registry set
	factorySet := memory.NewFactorySet()
	service := NewFileService(factorySet, "/tmp/uploads")

	tests := []struct {
		name     string
		tenantID string
		fileID   string
		expected map[string]string
	}{
		{
			name:     "File ID with thumbnails",
			tenantID: "tenant-a",
			fileID:   "test-file-123",
			expected: map[string]string{
				"small":  "t/tenant-a/thumbnails/test-file-123_small.jpg",
				"medium": "t/tenant-a/thumbnails/test-file-123_medium.jpg",
			},
		},
		{
			name:     "Another file ID",
			tenantID: "tenant-a",
			fileID:   "photo-456",
			expected: map[string]string{
				"small":  "t/tenant-a/thumbnails/photo-456_small.jpg",
				"medium": "t/tenant-a/thumbnails/photo-456_medium.jpg",
			},
		},
		{
			name:     "UUID file ID",
			tenantID: "tenant-b",
			fileID:   "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			expected: map[string]string{
				"small":  "t/tenant-b/thumbnails/f47ac10b-58cc-4372-a567-0e02b2c3d479_small.jpg",
				"medium": "t/tenant-b/thumbnails/f47ac10b-58cc-4372-a567-0e02b2c3d479_medium.jpg",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := service.GetThumbnailPaths(tt.tenantID, tt.fileID)
			c.Assert(result, qt.DeepEquals, tt.expected)
		})
	}
}

// TestFileService_GetThumbnailPaths_AlwaysTenantPrefixed is the structural
// invariant that #1793 promises: no matter what file id you pass, the
// emitted thumbnail keys live inside the supplied tenant's namespace.
func TestFileService_GetThumbnailPaths_AlwaysTenantPrefixed(t *testing.T) {
	c := qt.New(t)
	factorySet := memory.NewFactorySet()
	service := NewFileService(factorySet, "/tmp/uploads")

	for _, fileID := range []string{"x", "../../escape", "tenant-x/y/z"} {
		paths := service.GetThumbnailPaths("safe-tenant", fileID)
		for size, p := range paths {
			c.Assert(p[:len("t/safe-tenant/")], qt.Equals, "t/safe-tenant/",
				qt.Commentf("size=%s fileID=%q must live under safe-tenant namespace, got %q", size, fileID, p))
		}
	}
}
