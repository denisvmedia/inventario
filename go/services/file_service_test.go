package services

import (
	"context"
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

	// Create test registry set
	registrySet := &registry.Set{
		FileRegistry: memory.NewFileRegistry(),
	}

	// Create file service
	service := NewFileService(registrySet, uploadLocation)

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

	// Create test registry set
	registrySet := &registry.Set{
		FileRegistry: memory.NewFileRegistry(),
	}

	// Create file service
	service := NewFileService(registrySet, uploadLocation)

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
	registrySet := &registry.Set{
		FileRegistry: memory.NewFileRegistry(),
	}

	// Create file service
	service := NewFileService(registrySet, uploadLocation)

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
	registrySet := &registry.Set{
		FileRegistry: memory.NewFileRegistry(),
	}

	// Create file service
	service := NewFileService(registrySet, uploadLocation)

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
	registrySet := &registry.Set{
		FileRegistry: memory.NewFileRegistry(),
	}

	// Create file service
	service := NewFileService(registrySet, uploadLocation)

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
	registrySet := &registry.Set{
		FileRegistry: memory.NewFileRegistry(),
	}

	// Create file service
	service := NewFileService(registrySet, uploadLocation)

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
	registrySet := &registry.Set{
		FileRegistry:   memory.NewFileRegistry(),
		ExportRegistry: memory.NewExportRegistry(),
	}

	// Create file service
	service := NewFileService(registrySet, uploadLocation)

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
