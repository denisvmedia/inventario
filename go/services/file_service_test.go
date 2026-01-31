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
	// Set UserID to self-reference
	testUser.UserID = testUser.ID

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
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
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
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "test-file-" + tt.name},
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
				thumbnailPaths := service.GetThumbnailPaths(fileEntity.ID)
				c.Assert(thumbnailPaths, qt.HasLen, 2) // small and medium

				for sizeName, thumbnailPath := range thumbnailPaths {
					exists, err := b.Exists(ctx, thumbnailPath)
					c.Assert(err, qt.IsNil)
					c.Assert(exists, qt.IsTrue, qt.Commentf("Thumbnail %s should exist at %s", sizeName, thumbnailPath))
				}
			} else {
				// Check that no thumbnails were created
				thumbnailPaths := service.GetThumbnailPaths(fileEntity.ID)
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
		fileID   string
		expected map[string]string
	}{
		{
			name:   "File ID with thumbnails",
			fileID: "test-file-123",
			expected: map[string]string{
				"small":  "thumbnails/test-file-123_small.jpg",
				"medium": "thumbnails/test-file-123_medium.jpg",
			},
		},
		{
			name:   "Another file ID",
			fileID: "photo-456",
			expected: map[string]string{
				"small":  "thumbnails/photo-456_small.jpg",
				"medium": "thumbnails/photo-456_medium.jpg",
			},
		},
		{
			name:   "UUID file ID",
			fileID: "f47ac10b-58cc-4372-a567-0e02b2c3d479",
			expected: map[string]string{
				"small":  "thumbnails/f47ac10b-58cc-4372-a567-0e02b2c3d479_small.jpg",
				"medium": "thumbnails/f47ac10b-58cc-4372-a567-0e02b2c3d479_medium.jpg",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			result := service.GetThumbnailPaths(tt.fileID)
			c.Assert(result, qt.DeepEquals, tt.expected)
		})
	}
}
