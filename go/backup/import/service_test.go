package importpkg_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/memblob"

	importpkg "github.com/denisvmedia/inventario/backup/import"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func newTestRegistrySet() *registry.Set {
	// Use the proper NewRegistrySet function to ensure all dependencies are set up correctly
	registrySet, err := memory.NewRegistrySet(registry.Config("memory://"))
	if err != nil {
		panic(err)
	}
	return registrySet
}

func TestNewImportService(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()
	uploadLocation := "memory://test-bucket"

	service := importpkg.NewImportService(registrySet, uploadLocation)

	c.Assert(service, qt.IsNotNil)
}

func TestImportService_ProcessImport_ExportNotFound(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()
	uploadLocation := "mem://test-bucket"
	service := importpkg.NewImportService(registrySet, uploadLocation)
	ctx := context.Background()

	err := service.ProcessImport(ctx, "non-existent-id", "test-file.xml")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "not found")
}

func TestImportService_ProcessImport_BlobBucketError(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()
	// Use invalid upload location to trigger blob bucket error
	uploadLocation := "invalid://invalid-location"
	service := importpkg.NewImportService(registrySet, uploadLocation)
	ctx := context.Background()

	// Create a test export
	export := models.Export{
		Type:        models.ExportTypeImported,
		Status:      models.ExportStatusPending,
		Description: "Test import",
	}
	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	c.Assert(err, qt.IsNil)

	err = service.ProcessImport(ctx, createdExport.ID, "test-file.xml")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "failed to open blob bucket")

	// Verify export status was updated to failed
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedExport.Status, qt.Equals, models.ExportStatusFailed)
	c.Assert(updatedExport.ErrorMessage, qt.Contains, "failed to open blob bucket")
}

func TestImportService_ProcessImport_FileNotFound(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()
	uploadLocation := "mem://test-bucket"
	service := importpkg.NewImportService(registrySet, uploadLocation)
	ctx := context.Background()

	// Create a test export
	export := models.Export{
		Type:        models.ExportTypeImported,
		Status:      models.ExportStatusPending,
		Description: "Test import",
	}
	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	c.Assert(err, qt.IsNil)

	err = service.ProcessImport(ctx, createdExport.ID, "non-existent-file.xml")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "failed to open uploaded XML file")

	// Verify export status was updated to failed
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedExport.Status, qt.Equals, models.ExportStatusFailed)
	c.Assert(updatedExport.ErrorMessage, qt.Contains, "failed to open uploaded XML file")
}

func TestImportService_ProcessImport_InvalidXML(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for uploads
	tempDir := c.TempDir()
	uploadLocation := "file:///" + tempDir + "?create_dir=1"

	service := importpkg.NewImportService(registrySet, uploadLocation)
	ctx := context.Background()

	// Create blob bucket and upload invalid XML
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)

	invalidXML := "not valid xml at all <unclosed tag"
	filePath := "invalid.xml"
	err = b.WriteAll(ctx, filePath, []byte(invalidXML), nil)
	c.Assert(err, qt.IsNil)
	b.Close()

	// Create a test export
	export := models.Export{
		Type:        models.ExportTypeImported,
		Status:      models.ExportStatusPending,
		Description: "Test import",
	}
	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	c.Assert(err, qt.IsNil)

	err = service.ProcessImport(ctx, createdExport.ID, filePath)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "failed to parse XML metadata")

	// Verify export status was updated to failed
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedExport.Status, qt.Equals, models.ExportStatusFailed)
	c.Assert(updatedExport.ErrorMessage, qt.Contains, "failed to parse XML metadata")
}

func TestImportService_ProcessImport_Success(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for uploads
	tempDir := c.TempDir()
	uploadLocation := "file:///" + tempDir + "?create_dir=1"

	service := importpkg.NewImportService(registrySet, uploadLocation)
	ctx := context.Background()

	// Create blob bucket and upload valid XML
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)

	validXML := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/schema" exportDate="2024-01-01T00:00:00Z" exportType="commodities">
	<commodities>
		<commodity id="test-commodity-1">
			<name>Test Commodity</name>
			<type>electronics</type>
			<status>active</status>
			<count>1</count>
		</commodity>
	</commodities>
</inventory>`
	filePath := "valid.xml"
	err = b.WriteAll(ctx, filePath, []byte(validXML), nil)
	c.Assert(err, qt.IsNil)
	b.Close()

	// Create a test export
	export := models.Export{
		Type:        models.ExportTypeImported,
		Status:      models.ExportStatusPending,
		Description: "Test import",
	}
	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	c.Assert(err, qt.IsNil)

	err = service.ProcessImport(ctx, createdExport.ID, filePath)
	c.Assert(err, qt.IsNil)

	// Verify export was updated successfully
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedExport.Status, qt.Equals, models.ExportStatusCompleted)
	c.Assert(updatedExport.FilePath, qt.Equals, filePath)
	c.Assert(updatedExport.FileSize > 0, qt.IsTrue)
	c.Assert(updatedExport.CommodityCount, qt.Equals, 1)
	c.Assert(updatedExport.CompletedDate, qt.IsNotNil)

	// Verify file entity was created
	c.Assert(updatedExport.FileID, qt.IsNotNil)
	fileEntity, err := registrySet.FileRegistry.Get(ctx, *updatedExport.FileID)
	c.Assert(err, qt.IsNil)
	c.Assert(fileEntity.LinkedEntityType, qt.Equals, "export")
	c.Assert(fileEntity.LinkedEntityID, qt.Equals, updatedExport.ID)
	c.Assert(fileEntity.LinkedEntityMeta, qt.Equals, "xml-1.0")
	c.Assert(fileEntity.Type, qt.Equals, models.FileTypeDocument)
	c.Assert(fileEntity.Tags, qt.Contains, "export")
	c.Assert(fileEntity.Tags, qt.Contains, "xml")
	c.Assert(fileEntity.Tags, qt.Contains, "imported")
	c.Assert(fileEntity.Title, qt.Contains, "Import:")
	c.Assert(fileEntity.File.Ext, qt.Equals, ".xml")
	c.Assert(fileEntity.File.MIMEType, qt.Equals, "application/xml")
}

func TestImportService_ProcessImport_SuccessWithFileData(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for uploads
	tempDir := c.TempDir()
	uploadLocation := "file:///" + tempDir + "?create_dir=1"

	service := importpkg.NewImportService(registrySet, uploadLocation)
	ctx := context.Background()

	// Create blob bucket and upload XML with file data
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)

	xmlWithFiles := `<?xml version="1.0" encoding="UTF-8"?>
<inventory xmlns="http://inventario.example.com/schema" exportDate="2024-01-01T00:00:00Z" exportType="commodities">
	<commodities>
		<commodity id="test-commodity-1">
			<name>Test Commodity</name>
			<type>electronics</type>
			<status>active</status>
			<count>1</count>
			<images>
				<file id="img1">
					<path>test-image</path>
					<originalPath>test-image.jpg</originalPath>
					<extension>.jpg</extension>
					<mimeType>image/jpeg</mimeType>
					<data>dGVzdCBpbWFnZSBkYXRh</data>
				</file>
			</images>
		</commodity>
	</commodities>
</inventory>`
	filePath := "with-files.xml"
	err = b.WriteAll(ctx, filePath, []byte(xmlWithFiles), nil)
	c.Assert(err, qt.IsNil)
	b.Close()

	// Create a test export
	export := models.Export{
		Type:        models.ExportTypeImported,
		Status:      models.ExportStatusPending,
		Description: "Test import with files",
	}
	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	c.Assert(err, qt.IsNil)

	err = service.ProcessImport(ctx, createdExport.ID, filePath)
	c.Assert(err, qt.IsNil)

	// Verify export was updated successfully with file data
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedExport.Status, qt.Equals, models.ExportStatusCompleted)
	c.Assert(updatedExport.CommodityCount, qt.Equals, 1)
	c.Assert(updatedExport.ImageCount, qt.Equals, 1)
	c.Assert(updatedExport.BinaryDataSize > 0, qt.IsTrue)
	c.Assert(updatedExport.IncludeFileData, qt.IsTrue)
}

func TestImportService_ProcessImport_ExportRecordDeleted(t *testing.T) {
	c := qt.New(t)
	registrySet := newTestRegistrySet()

	// Create a temporary directory for uploads
	tempDir := c.TempDir()
	uploadLocation := "file:///" + tempDir + "?create_dir=1"

	service := importpkg.NewImportService(registrySet, uploadLocation)
	ctx := context.Background()

	// Create a test export
	export := models.Export{
		Type:        models.ExportTypeImported,
		Status:      models.ExportStatusPending,
		Description: "Test import",
	}
	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	c.Assert(err, qt.IsNil)

	// Delete the export to simulate record not found
	err = registrySet.ExportRegistry.Delete(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)

	err = service.ProcessImport(ctx, createdExport.ID, "test-file.xml")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "not found")
}
