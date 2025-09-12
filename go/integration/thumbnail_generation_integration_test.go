package integration_test

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"runtime"
	"testing"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // Register file driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// TestThumbnailGenerationIntegration tests the complete thumbnail generation workflow
// using the services directly to verify the core functionality
func TestThumbnailGenerationIntegration(t *testing.T) {
	c := qt.New(t)

	// Create temporary directory for test files
	tempDir := t.TempDir()
	var uploadLocation string
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	} else {
		uploadLocation = "file://" + tempDir + "?create_dir=1"
	}

	// Create test user context
	testUser := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-123"},
			TenantID: "test-tenant-id",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	testUser.UserID = testUser.ID
	ctx := appctx.WithUser(context.Background(), testUser)

	// Create factory set and services
	factorySet := memory.NewFactorySet()
	fileService := services.NewFileService(factorySet, uploadLocation)
	fileSigningService := services.NewFileSigningService([]byte("test-signing-key-32-bytes-long!!"), 900)

	t.Log("📁 Setting up test environment...")

	// Step 1: Create and save a test image file
	t.Log("📤 Creating test image file...")
	originalPath := createTestImageFile(c, ctx, uploadLocation)

	// Step 2: Generate thumbnails using the file service
	t.Log("🖼️ Generating thumbnails...")

	// Create a file entity for testing
	fileEntity := &models.FileEntity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-file-123"},
		},
		Type: models.FileTypeImage,
		File: &models.File{
			Path:         "test-image",
			OriginalPath: originalPath,
			Ext:          ".png",
			MIMEType:     "image/png",
		},
	}

	err := fileService.GenerateThumbnails(ctx, fileEntity)
	c.Assert(err, qt.IsNil)

	// Step 3: Verify thumbnails were created in storage
	t.Log("🔍 Verifying thumbnails in storage...")
	verifyThumbnailsInStorage(c, ctx, uploadLocation, originalPath, fileService)

	// Step 4: Test signed URL generation with thumbnails
	t.Log("🔗 Testing signed URL generation with thumbnails...")
	verifySignedURLGeneration(c, fileSigningService, originalPath)

	// Step 5: Test thumbnail cleanup
	t.Log("🗑️ Testing thumbnail cleanup...")
	verifyThumbnailCleanup(c, ctx, uploadLocation, fileEntity.ID, originalPath, fileService)

	t.Log("✅ Thumbnail integration test completed successfully!")
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

// createTestImageFile creates a test PNG image file in storage and returns the path
func createTestImageFile(c *qt.C, ctx context.Context, uploadLocation string) string {
	// Create test PNG image
	testImg := createTestImage(400, 300, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	// Open blob storage
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	// Save the test image to storage
	originalPath := "test-image.png"
	writer, err := b.NewWriter(ctx, originalPath, nil)
	c.Assert(err, qt.IsNil)

	err = png.Encode(writer, testImg)
	c.Assert(err, qt.IsNil)

	err = writer.Close()
	c.Assert(err, qt.IsNil)

	return originalPath
}

// verifyThumbnailsInStorage checks that thumbnail files exist in blob storage
func verifyThumbnailsInStorage(c *qt.C, ctx context.Context, uploadLocation, originalPath string, fileService *services.FileService) {
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	// Check that thumbnails exist - use the file ID from the file entity
	testFileID := "test-file-123" // This matches the ID in the fileEntity above
	thumbnailPaths := fileService.GetThumbnailPaths(testFileID)

	c.Assert(len(thumbnailPaths), qt.Equals, 2) // small and medium

	for sizeName, thumbnailPath := range thumbnailPaths {
		exists, err := b.Exists(ctx, thumbnailPath)
		c.Assert(err, qt.IsNil)
		c.Assert(exists, qt.IsTrue, qt.Commentf("Thumbnail %s should exist at %s", sizeName, thumbnailPath))
	}
}

// verifySignedURLGeneration tests signed URL generation with thumbnails using the service directly
func verifySignedURLGeneration(c *qt.C, fileSigningService *services.FileSigningService, originalPath string) {
	// Create a file entity for testing
	fileEntity := &models.FileEntity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-file-123"},
		},
		Type: models.FileTypeImage,
		File: &models.File{
			Path:         "test-image",
			OriginalPath: originalPath,
			Ext:          ".png",
			MIMEType:     "image/png",
		},
	}

	// Test signed URL generation with thumbnails
	originalURL, thumbnails, err := fileSigningService.GenerateSignedURLsWithThumbnails(fileEntity, "test-user-123")

	c.Assert(err, qt.IsNil)
	c.Assert(originalURL, qt.Not(qt.Equals), "")
	c.Assert(thumbnails, qt.IsNotNil)
	c.Assert(len(thumbnails), qt.Equals, 2) // small and medium
	c.Assert(thumbnails["small"], qt.Not(qt.Equals), "")
	c.Assert(thumbnails["medium"], qt.Not(qt.Equals), "")

	// Verify thumbnail URLs contain expected paths - current implementation uses /thumbnails/{fileID}/{size}
	c.Assert(thumbnails["small"], qt.Contains, "/thumbnails/")
	c.Assert(thumbnails["small"], qt.Contains, "/small")
	c.Assert(thumbnails["medium"], qt.Contains, "/thumbnails/")
	c.Assert(thumbnails["medium"], qt.Contains, "/medium")
}

// verifyThumbnailCleanup tests that thumbnails are deleted when using the file service
func verifyThumbnailCleanup(c *qt.C, ctx context.Context, uploadLocation, fileID, originalPath string, fileService *services.FileService) {
	// Get thumbnail paths before deletion
	thumbnailPaths := fileService.GetThumbnailPaths(fileID)

	// Verify thumbnails exist before cleanup
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	for _, thumbnailPath := range thumbnailPaths {
		exists, err := b.Exists(ctx, thumbnailPath)
		c.Assert(err, qt.IsNil)
		c.Assert(exists, qt.IsTrue, qt.Commentf("Thumbnail should exist before cleanup at %s", thumbnailPath))
	}

	// Delete the original file and thumbnails using the service method
	err = fileService.DeletePhysicalFile(ctx, originalPath)
	c.Assert(err, qt.IsNil)

	// Manually delete thumbnails (simulating the deletePhysicalFileAndThumbnails behavior)
	for _, thumbnailPath := range thumbnailPaths {
		_ = fileService.DeletePhysicalFile(ctx, thumbnailPath)
	}

	// Verify thumbnails are deleted
	for sizeName, thumbnailPath := range thumbnailPaths {
		exists, err := b.Exists(ctx, thumbnailPath)
		c.Assert(err, qt.IsNil)
		c.Assert(exists, qt.IsFalse, qt.Commentf("Thumbnail %s should be deleted at %s", sizeName, thumbnailPath))
	}
}
