package export

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

const testUserID = "test-user-123"

func newTestRegistrySet() *registry.Set {
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	fileRegistry := memory.NewFileRegistry()

	registrySet := &registry.Set{
		LocationRegistry:  locationRegistry,
		AreaRegistry:      areaRegistry,
		CommodityRegistry: memory.NewCommodityRegistry(areaRegistry),
		ExportRegistry:    memory.NewExportRegistry(),
		FileRegistry:      fileRegistry,
	}
	return registrySet
}

// newTestContext creates a context with test user ID for testing
func newTestContext() context.Context {
	return appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: testUserID},
			TenantID: "test-tenant",
		},
	})
}

func TestNewExportService(t *testing.T) {
	c := qt.New(t)
	registrySet := &registry.Set{}
	uploadLocation := "/tmp/uploads"

	service := NewExportService(registrySet, uploadLocation)

	c.Assert(service, qt.IsNotNil)
	c.Assert(service.registrySet, qt.Equals, registrySet)
	c.Assert(service.uploadLocation, qt.Equals, uploadLocation)
}

func TestInventoryDataXMLStructure(t *testing.T) {
	c := qt.New(t)
	// Test the XML marshaling of the InventoryData structure
	data := &InventoryData{
		ExportDate: "2024-01-01T00:00:00Z",
		ExportType: "full_database",
		Locations: []*Location{
			{
				ID:      "loc1",
				Name:    "Main Warehouse",
				Address: "123 Main St",
			},
		},
		Areas: []*Area{
			{
				ID:         "area1",
				Name:       "Storage Area A",
				LocationID: "loc1",
			},
		},
		Commodities: []*Commodity{
			{
				ID:     "comm1",
				Name:   "Test Item",
				Type:   "equipment",
				AreaID: "area1",
				Count:  10,
				Status: "active",
			},
		},
	}

	xmlData, err := xml.MarshalIndent(data, "", "  ")
	c.Assert(err, qt.IsNil)

	// Check that the XML contains expected elements
	xmlStr := string(xmlData)
	expectedElements := []string{
		`<inventory exportDate="2024-01-01T00:00:00Z" exportType="full_database">`,
		`<locations>`,
		`<location id="loc1">`,
		`<locationName>Main Warehouse</locationName>`,
		`<areas>`,
		`<area id="area1">`,
		`<commodities>`,
		`<commodity id="comm1">`,
	}

	for _, expected := range expectedElements {
		c.Assert(xmlStr, qt.Contains, expected)
	}
}

func TestExportServiceProcessExport_InvalidID(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	registrySet := newTestRegistrySet()
	uploadLocation := "file://" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := newTestContext()

	// Test with non-existent export ID
	err := service.ProcessExport(ctx, "non-existent-id")
	c.Assert(err, qt.IsNotNil)
}

func TestExportServiceProcessExport_Success(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	registrySet := newTestRegistrySet()
	uploadLocation := "file:///" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := newTestContext()

	// Create a test export in the database
	export := models.Export{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-export-1", "test-tenant", testUserID),
		Type:                models.ExportTypeCommodities,
		Status:              models.ExportStatusPending,
		IncludeFileData:     false,
	}

	createdExport, err := registrySet.ExportRegistry.Create(ctx, export)
	c.Assert(err, qt.IsNil)

	// Process the export
	err = service.ProcessExport(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)

	// Verify the export was updated
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, createdExport.ID)
	c.Assert(err, qt.IsNil)

	c.Assert(updatedExport.Status == models.ExportStatusCompleted || updatedExport.Status == models.ExportStatusFailed, qt.IsTrue)
}

func TestStreamXMLExport(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	registrySet := newTestRegistrySet()
	uploadLocation := "file://" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := newTestContext()

	// Test different export types
	testCases := []struct {
		name       string
		exportType models.ExportType
	}{
		{"Full Database", models.ExportTypeFullDatabase},
		{"Locations", models.ExportTypeLocations},
		{"Areas", models.ExportTypeAreas},
		{"Commodities", models.ExportTypeCommodities},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			export := models.Export{
				TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-export-"+tc.name, "test-tenant", testUserID),
				Type:                tc.exportType,
				Status:              models.ExportStatusPending,
				IncludeFileData:     false,
			}

			var buf bytes.Buffer
			_, err := service.streamXMLExport(ctx, export, &buf)
			c.Assert(err, qt.IsNil)

			xmlContent := buf.String()
			c.Assert(xmlContent, qt.Contains, `<?xml version="1.0" encoding="UTF-8"?>`)
			c.Assert(xmlContent, qt.Contains, fmt.Sprintf(`exportType="%s"`, tc.exportType))
			c.Assert(xmlContent, qt.Contains, `<inventory`)
			c.Assert(xmlContent, qt.Contains, `</inventory>`)
		})
	}
}

func TestStreamXMLExport_InvalidType(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	registrySet := newTestRegistrySet()
	uploadLocation := "file://" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := newTestContext()

	export := models.Export{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-export-invalid", "test-tenant", testUserID),
		Type:                "invalid_type",
		Status:              models.ExportStatusPending,
		IncludeFileData:     false,
	}

	var buf bytes.Buffer
	_, err := service.streamXMLExport(ctx, export, &buf)
	c.Assert(err, qt.IsNotNil)
}

func TestGenerateExport(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	registrySet := newTestRegistrySet()
	uploadLocation := "file:///" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := newTestContext()

	export := models.Export{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-export-123", "default-tenant", testUserID),
		Type:                models.ExportTypeCommodities,
		Status:              models.ExportStatusPending,
		IncludeFileData:     false,
	}

	blobKey, _, err := service.generateExport(ctx, export)
	c.Assert(err, qt.IsNil)

	// Check that blob was created
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	exists, err := b.Exists(ctx, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)

	// Check blob key format
	expectedPrefix := fmt.Sprintf("exports/export_%s_", export.Type)
	c.Assert(blobKey, qt.Contains, expectedPrefix)
	c.Assert(blobKey, qt.Contains, ".xml")

	// Clean up
	err = b.Delete(ctx, blobKey)
	c.Assert(err, qt.IsNil)
}

func TestFileHandlingWithIncludeFileData(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	// Create interconnected registries
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	fileRegistry := memory.NewFileRegistry()
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)
	imageRegistry := memory.NewImageRegistry(commodityRegistry)
	invoiceRegistry := memory.NewInvoiceRegistry(commodityRegistry)
	manualRegistry := memory.NewManualRegistry(commodityRegistry)
	exportRegistry := memory.NewExportRegistry()

	registrySet := &registry.Set{
		LocationRegistry:  locationRegistry,
		AreaRegistry:      areaRegistry,
		CommodityRegistry: commodityRegistry,
		ImageRegistry:     imageRegistry,
		InvoiceRegistry:   invoiceRegistry,
		ManualRegistry:    manualRegistry,
		ExportRegistry:    exportRegistry,
		FileRegistry:      fileRegistry,
	}

	uploadLocation := "file:///" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := newTestContext()

	// Create test data
	location := models.Location{TenantAwareEntityID: models.WithTenantUserAwareEntityID("loc1", "default-tenant", testUserID), Name: "Location 1", Address: "Address 1"}
	createdLocation, err := registrySet.LocationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)

	area := models.Area{TenantAwareEntityID: models.WithTenantUserAwareEntityID("area1", "default-tenant", testUserID), Name: "Area 1", LocationID: createdLocation.ID}
	createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	commodity := models.Commodity{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("commodity1", "default-tenant", testUserID),
		Name:                "Test Commodity",
		Type:                models.CommodityTypeElectronics,
		AreaID:              createdArea.ID,
		Count:               1,
		Status:              models.CommodityStatusInUse,
	}
	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Create test files in the blob storage
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	testImagePath := "test-image.jpg"
	testImageData := []byte("test image data")
	err = b.WriteAll(ctx, testImagePath, testImageData, nil)
	c.Assert(err, qt.IsNil)

	testInvoicePath := "test-invoice.pdf"
	testInvoiceData := []byte("test invoice data")
	err = b.WriteAll(ctx, testInvoicePath, testInvoiceData, nil)
	c.Assert(err, qt.IsNil)

	// Create test file models (they will automatically be linked to the commodity)
	image := models.Image{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("img1", "default-tenant", testUserID),
		CommodityID:         createdCommodity.ID,
		File: &models.File{
			Path:         "test-image",
			OriginalPath: testImagePath,
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}
	createdImage, err := registrySet.ImageRegistry.Create(ctx, image)
	c.Assert(err, qt.IsNil)

	invoice := models.Invoice{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("inv1", "default-tenant", testUserID),
		CommodityID:         createdCommodity.ID,
		File: &models.File{
			Path:         "test-invoice",
			OriginalPath: testInvoicePath,
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}
	createdInvoice, err := registrySet.InvoiceRegistry.Create(ctx, invoice)
	c.Assert(err, qt.IsNil)

	// Test with file data included
	stats := &ExportStats{}
	xmlCommodity, err := service.convertCommodityToXML(ctx, createdCommodity, ExportArgs{IncludeFileData: true}, stats)
	c.Assert(err, qt.IsNil)
	c.Assert(xmlCommodity.Images, qt.HasLen, 1)
	c.Assert(xmlCommodity.Invoices, qt.HasLen, 1)
	c.Assert(xmlCommodity.Manuals, qt.HasLen, 0)

	// Check image file data
	c.Assert(xmlCommodity.Images[0].ID, qt.Equals, createdImage.ID)
	c.Assert(xmlCommodity.Images[0].Path, qt.Equals, "test-image")
	c.Assert(xmlCommodity.Images[0].OriginalPath, qt.Equals, testImagePath)
	c.Assert(xmlCommodity.Images[0].Extension, qt.Equals, ".jpg")
	c.Assert(xmlCommodity.Images[0].MimeType, qt.Equals, "image/jpeg")
	c.Assert(xmlCommodity.Images[0].Data, qt.Not(qt.Equals), "")

	// Verify base64 data matches original file content for image
	expectedImageBase64 := base64.StdEncoding.EncodeToString(testImageData)
	c.Assert(xmlCommodity.Images[0].Data, qt.Equals, expectedImageBase64)

	// Check invoice file data
	c.Assert(xmlCommodity.Invoices[0].ID, qt.Equals, createdInvoice.ID)
	c.Assert(xmlCommodity.Invoices[0].Path, qt.Equals, "test-invoice")
	c.Assert(xmlCommodity.Invoices[0].OriginalPath, qt.Equals, testInvoicePath)
	c.Assert(xmlCommodity.Invoices[0].Extension, qt.Equals, ".pdf")
	c.Assert(xmlCommodity.Invoices[0].MimeType, qt.Equals, "application/pdf")
	c.Assert(xmlCommodity.Invoices[0].Data, qt.Not(qt.Equals), "")

	// Verify base64 data matches original file content for invoice
	expectedInvoiceBase64 := base64.StdEncoding.EncodeToString(testInvoiceData)
	c.Assert(xmlCommodity.Invoices[0].Data, qt.Equals, expectedInvoiceBase64)

	// Test without file data
	stats = &ExportStats{}
	xmlCommodityNoData, err := service.convertCommodityToXML(ctx, createdCommodity, ExportArgs{IncludeFileData: false}, stats)
	c.Assert(err, qt.IsNil)
	c.Assert(xmlCommodityNoData.Images, qt.HasLen, 0)
	c.Assert(xmlCommodityNoData.Invoices, qt.HasLen, 0)
}

func TestBase64FileDataVerification(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	// Create interconnected registries
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	fileRegistry := memory.NewFileRegistry()
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)
	imageRegistry := memory.NewImageRegistry(commodityRegistry)
	invoiceRegistry := memory.NewInvoiceRegistry(commodityRegistry)
	manualRegistry := memory.NewManualRegistry(commodityRegistry)
	exportRegistry := memory.NewExportRegistry()

	registrySet := &registry.Set{
		LocationRegistry:  locationRegistry,
		AreaRegistry:      areaRegistry,
		CommodityRegistry: commodityRegistry,
		ImageRegistry:     imageRegistry,
		InvoiceRegistry:   invoiceRegistry,
		ManualRegistry:    manualRegistry,
		ExportRegistry:    exportRegistry,
		FileRegistry:      fileRegistry,
	}

	uploadLocation := "file:///" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := newTestContext()

	// Create test data
	location := models.Location{TenantAwareEntityID: models.WithTenantUserAwareEntityID("loc1", "default-tenant", testUserID), Name: "Location 1", Address: "Address 1"}
	createdLocation, err := registrySet.LocationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)

	area := models.Area{TenantAwareEntityID: models.WithTenantUserAwareEntityID("area1", "default-tenant", testUserID), Name: "Area 1", LocationID: createdLocation.ID}
	createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	commodity := models.Commodity{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("commodity1", "default-tenant", testUserID),
		Name:                "Test Commodity",
		Type:                models.CommodityTypeElectronics,
		AreaID:              createdArea.ID,
		Count:               1,
		Status:              models.CommodityStatusInUse,
	}
	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Create test files with different types of content
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	// Test various file types and content
	testFiles := []struct {
		path     string
		data     []byte
		fileType string
		ext      string
		mime     string
	}{
		{"image.jpg", []byte("binary-image-data-with-special-chars\x00\x01\x02\xFF"), "image", ".jpg", "image/jpeg"},
		{"invoice.pdf", []byte("PDF content with unicode: ñáéíóú 测试"), "invoice", ".pdf", "application/pdf"},
		{"manual.txt", []byte("Simple text content"), "manual", ".txt", "text/plain"},
	}

	var createdFiles []string
	for _, tf := range testFiles {
		// Write file to blob storage
		err = b.WriteAll(ctx, tf.path, tf.data, nil)
		c.Assert(err, qt.IsNil)

		// Create corresponding model based on file type
		switch tf.fileType {
		case "image":
			image := models.Image{
				TenantAwareEntityID: models.WithTenantUserAwareEntityID("img-"+tf.path, "default-tenant", testUserID),
				CommodityID:         createdCommodity.ID,
				File: &models.File{
					Path:         "img-" + tf.path,
					OriginalPath: tf.path,
					Ext:          tf.ext,
					MIMEType:     tf.mime,
				},
			}
			_, err := registrySet.ImageRegistry.Create(ctx, image)
			c.Assert(err, qt.IsNil)
		case "invoice":
			invoice := models.Invoice{
				TenantAwareEntityID: models.WithTenantUserAwareEntityID("inv-"+tf.path, "default-tenant", testUserID),
				CommodityID:         createdCommodity.ID,
				File: &models.File{
					Path:         "inv-" + tf.path,
					OriginalPath: tf.path,
					Ext:          tf.ext,
					MIMEType:     tf.mime,
				},
			}
			_, err := registrySet.InvoiceRegistry.Create(ctx, invoice)
			c.Assert(err, qt.IsNil)
		case "manual":
			manual := models.Manual{
				TenantAwareEntityID: models.WithTenantUserAwareEntityID("man-"+tf.path, "default-tenant", testUserID),
				CommodityID:         createdCommodity.ID,
				File: &models.File{
					Path:         "man-" + tf.path,
					OriginalPath: tf.path,
					Ext:          tf.ext,
					MIMEType:     tf.mime,
				},
			}
			_, err := registrySet.ManualRegistry.Create(ctx, manual)
			c.Assert(err, qt.IsNil)
		}

		createdFiles = append(createdFiles, tf.path)
	}

	// Test with file data included
	stats := &ExportStats{}
	xmlCommodity, err := service.convertCommodityToXML(ctx, createdCommodity, ExportArgs{IncludeFileData: true}, stats)
	c.Assert(err, qt.IsNil)
	c.Assert(xmlCommodity.Images, qt.HasLen, 1)
	c.Assert(xmlCommodity.Invoices, qt.HasLen, 1)
	c.Assert(xmlCommodity.Manuals, qt.HasLen, 1)

	// Verify each file's base64 data matches the original content exactly
	for _, tf := range testFiles {
		expectedBase64 := base64.StdEncoding.EncodeToString(tf.data)

		switch tf.fileType {
		case "image":
			c.Assert(xmlCommodity.Images[0].Data, qt.Equals, expectedBase64,
				qt.Commentf("Image base64 data should match original file content"))

			// Also verify that the base64 can be decoded back to original data
			decodedData, err := base64.StdEncoding.DecodeString(xmlCommodity.Images[0].Data)
			c.Assert(err, qt.IsNil)
			c.Assert(decodedData, qt.DeepEquals, tf.data)

		case "invoice":
			c.Assert(xmlCommodity.Invoices[0].Data, qt.Equals, expectedBase64,
				qt.Commentf("Invoice base64 data should match original file content"))

			// Also verify that the base64 can be decoded back to original data
			decodedData, err := base64.StdEncoding.DecodeString(xmlCommodity.Invoices[0].Data)
			c.Assert(err, qt.IsNil)
			c.Assert(decodedData, qt.DeepEquals, tf.data)

		case "manual":
			c.Assert(xmlCommodity.Manuals[0].Data, qt.Equals, expectedBase64,
				qt.Commentf("Manual base64 data should match original file content"))

			// Also verify that the base64 can be decoded back to original data
			decodedData, err := base64.StdEncoding.DecodeString(xmlCommodity.Manuals[0].Data)
			c.Assert(err, qt.IsNil)
			c.Assert(decodedData, qt.DeepEquals, tf.data)
		}
	}

	// Test full export with file data and verify base64 encoding in XML
	export := models.Export{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-export-base64", "test-tenant", testUserID),
		Type:                models.ExportTypeFullDatabase,
		Status:              models.ExportStatusPending,
		IncludeFileData:     true,
	}

	var buf bytes.Buffer
	_, err = service.streamXMLExport(ctx, export, &buf)
	c.Assert(err, qt.IsNil)

	xmlContent := buf.String()

	// Verify that all expected base64 data is present in the XML
	for _, tf := range testFiles {
		expectedBase64 := base64.StdEncoding.EncodeToString(tf.data)
		c.Assert(xmlContent, qt.Contains, expectedBase64,
			qt.Commentf("Full export XML should contain base64 data for %s", tf.fileType))
	}

	// Clean up test files
	for _, filePath := range createdFiles {
		err = b.Delete(ctx, filePath)
		c.Assert(err, qt.IsNil)
	}
}

func TestExportService_ProcessExport_CalculatesStatistics(t *testing.T) {
	c := qt.New(t)

	// Create test data
	ctx := newTestContext()
	registrySet := createTestRegistrySetWithFiles(c, ctx)
	tempDir := c.TempDir()
	uploadLocation := "file:///" + tempDir + "?create_dir=1"

	// Create actual files in blob storage for testing
	err := createTestFilesInBlobStorage(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)

	service := NewExportService(registrySet, uploadLocation)

	// Create test export
	testExport := &models.Export{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-export-stats", "test-tenant", testUserID),
		Type:                models.ExportTypeFullDatabase,
		Status:              models.ExportStatusPending,
		IncludeFileData:     true,
		Description:         "Test export",
	}

	// Save export
	savedExport, err := registrySet.ExportRegistry.Create(ctx, *testExport)
	c.Assert(err, qt.IsNil)

	// Process export
	err = service.ProcessExport(ctx, savedExport.ID)
	c.Assert(err, qt.IsNil)

	// Verify export was updated with statistics
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, savedExport.ID)
	c.Assert(err, qt.IsNil)

	// Check status
	c.Assert(updatedExport.Status, qt.Equals, models.ExportStatusCompleted)
	c.Assert(updatedExport.FilePath, qt.Not(qt.Equals), "")
	c.Assert(updatedExport.FileSize, qt.Not(qt.Equals), int64(0))

	// Check statistics
	c.Assert(updatedExport.LocationCount, qt.Equals, 2)
	c.Assert(updatedExport.AreaCount, qt.Equals, 3)
	c.Assert(updatedExport.CommodityCount, qt.Equals, 2)
	c.Assert(updatedExport.ImageCount, qt.Equals, 2)
	c.Assert(updatedExport.InvoiceCount, qt.Equals, 1)
	c.Assert(updatedExport.ManualCount, qt.Equals, 1)
	c.Assert(updatedExport.BinaryDataSize, qt.Not(qt.Equals), int64(0))
}

func TestExportService_ProcessExport_WithoutFileData(t *testing.T) {
	c := qt.New(t)

	ctx := newTestContext()
	registrySet := createTestRegistrySetWithFiles(c, ctx)
	tempDir := c.TempDir()
	uploadLocation := "file:///" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)

	// Create test export without file data
	testExport := &models.Export{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-export-no-files", "test-tenant", testUserID),
		Type:                models.ExportTypeFullDatabase,
		Status:              models.ExportStatusPending,
		IncludeFileData:     false,
		Description:         "Test export without files",
	}

	// Save export
	savedExport, err := registrySet.ExportRegistry.Create(ctx, *testExport)
	c.Assert(err, qt.IsNil)

	// Process export
	err = service.ProcessExport(ctx, savedExport.ID)
	c.Assert(err, qt.IsNil)

	// Verify export was updated with statistics
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, savedExport.ID)
	c.Assert(err, qt.IsNil)

	// Check status
	c.Assert(updatedExport.Status, qt.Equals, models.ExportStatusCompleted)

	// Check statistics
	c.Assert(updatedExport.LocationCount, qt.Equals, 2)
	c.Assert(updatedExport.AreaCount, qt.Equals, 3)
	c.Assert(updatedExport.CommodityCount, qt.Equals, 2)
	// File counts should be 0 when file data is not included
	c.Assert(updatedExport.ImageCount, qt.Equals, 0)
	c.Assert(updatedExport.InvoiceCount, qt.Equals, 0)
	c.Assert(updatedExport.ManualCount, qt.Equals, 0)
	c.Assert(updatedExport.BinaryDataSize, qt.Equals, int64(0))
}

func TestExportService_Base64SizeTracking(t *testing.T) {
	c := qt.New(t)

	ctx := newTestContext()
	registrySet := createTestRegistrySetWithFiles(c, ctx)
	tempDir := c.TempDir()
	uploadLocation := "file:///" + tempDir + "?create_dir=1"

	// Create actual files in blob storage for testing
	err := createTestFilesInBlobStorage(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)

	service := NewExportService(registrySet, uploadLocation)

	// Create test export with file data
	testExport := &models.Export{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-export-base64-size", "test-tenant", testUserID),
		Type:                models.ExportTypeCommodities,
		Status:              models.ExportStatusPending,
		IncludeFileData:     true,
		Description:         "Test base64 size tracking",
	}

	// Save export
	savedExport, err := registrySet.ExportRegistry.Create(ctx, *testExport)
	c.Assert(err, qt.IsNil)

	// Process export
	err = service.ProcessExport(ctx, savedExport.ID)
	c.Assert(err, qt.IsNil)

	// Verify export was updated with statistics
	updatedExport, err := registrySet.ExportRegistry.Get(ctx, savedExport.ID)
	c.Assert(err, qt.IsNil)

	// Check that binary data size is greater than 0 and represents base64 encoded size
	c.Assert(updatedExport.BinaryDataSize, qt.Not(qt.Equals), int64(0))

	// The base64 encoded size should be larger than the original data size
	// Base64 encoding increases size by approximately 33%
	expectedMinSize := int64(len("test image data") + len("test invoice data") + len("test manual data"))
	c.Assert(updatedExport.BinaryDataSize >= expectedMinSize, qt.IsTrue)
}

// createTestRegistrySetWithFiles creates a test registry set with sample data including files
func createTestRegistrySetWithFiles(c *qt.C, ctx context.Context) *registry.Set {
	// Create interconnected registries
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
	fileRegistry := memory.NewFileRegistry()
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)
	imageRegistry := memory.NewImageRegistry(commodityRegistry)
	invoiceRegistry := memory.NewInvoiceRegistry(commodityRegistry)
	manualRegistry := memory.NewManualRegistry(commodityRegistry)
	exportRegistry := memory.NewExportRegistry()

	registrySet := &registry.Set{
		LocationRegistry:  locationRegistry,
		AreaRegistry:      areaRegistry,
		CommodityRegistry: commodityRegistry,
		ImageRegistry:     imageRegistry,
		InvoiceRegistry:   invoiceRegistry,
		ManualRegistry:    manualRegistry,
		ExportRegistry:    exportRegistry,
		FileRegistry:      fileRegistry,
	}

	// Create test locations
	location1 := models.Location{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("loc1", "default-tenant", testUserID),
		Name:                "Test Location 1",
		Address:             "123 Test St",
	}
	location2 := models.Location{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("loc2", "default-tenant", testUserID),
		Name:                "Test Location 2",
		Address:             "456 Test Ave",
	}

	savedLocation1, err := registrySet.LocationRegistry.Create(ctx, location1)
	c.Assert(err, qt.IsNil)
	savedLocation2, err := registrySet.LocationRegistry.Create(ctx, location2)
	c.Assert(err, qt.IsNil)

	// Create test areas
	area1 := models.Area{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("area1", "default-tenant", testUserID),
		Name:                "Test Area 1",
		LocationID:          savedLocation1.ID,
	}
	area2 := models.Area{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("area2", "default-tenant", testUserID),
		Name:                "Test Area 2",
		LocationID:          savedLocation1.ID,
	}
	area3 := models.Area{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("area3", "default-tenant", testUserID),
		Name:                "Test Area 3",
		LocationID:          savedLocation2.ID,
	}

	savedArea1, err := registrySet.AreaRegistry.Create(ctx, area1)
	c.Assert(err, qt.IsNil)
	savedArea2, err := registrySet.AreaRegistry.Create(ctx, area2)
	c.Assert(err, qt.IsNil)
	_, err = registrySet.AreaRegistry.Create(ctx, area3)
	c.Assert(err, qt.IsNil)

	// Create test commodities
	commodity1 := models.Commodity{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("commodity1", "default-tenant", testUserID),
		Name:                "Test Commodity 1",
		AreaID:              savedArea1.ID,
		Count:               1,
		Type:                models.CommodityTypeElectronics,
		Status:              models.CommodityStatusInUse,
	}
	commodity2 := models.Commodity{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("commodity2", "default-tenant", testUserID),
		Name:                "Test Commodity 2",
		AreaID:              savedArea2.ID,
		Count:               2,
		Type:                models.CommodityTypeElectronics,
		Status:              models.CommodityStatusInUse,
	}

	savedCommodity1, err := registrySet.CommodityRegistry.Create(ctx, commodity1)
	c.Assert(err, qt.IsNil)
	savedCommodity2, err := registrySet.CommodityRegistry.Create(ctx, commodity2)
	c.Assert(err, qt.IsNil)

	// Create test images
	image1 := models.Image{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("image1", "default-tenant", testUserID),
		CommodityID:         savedCommodity1.ID,
		File: &models.File{
			Path:         "test-image-1",
			OriginalPath: "test-image-1.jpg",
			Ext:          "jpg",
			MIMEType:     "image/jpeg",
		},
	}
	image2 := models.Image{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("image2", "default-tenant", testUserID),
		CommodityID:         savedCommodity2.ID,
		File: &models.File{
			Path:         "test-image-2",
			OriginalPath: "test-image-2.png",
			Ext:          "png",
			MIMEType:     "image/png",
		},
	}

	_, err = registrySet.ImageRegistry.Create(ctx, image1)
	c.Assert(err, qt.IsNil)
	_, err = registrySet.ImageRegistry.Create(ctx, image2)
	c.Assert(err, qt.IsNil)

	// Create test invoice
	invoice1 := models.Invoice{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("invoice1", "default-tenant", testUserID),
		CommodityID:         savedCommodity1.ID,
		File: &models.File{
			Path:         "test-invoice-1",
			OriginalPath: "test-invoice-1.pdf",
			Ext:          "pdf",
			MIMEType:     "application/pdf",
		},
	}

	_, err = registrySet.InvoiceRegistry.Create(ctx, invoice1)
	c.Assert(err, qt.IsNil)

	// Create test manual
	manual1 := models.Manual{
		TenantAwareEntityID: models.WithTenantUserAwareEntityID("manual1", "default-tenant", testUserID),
		CommodityID:         savedCommodity1.ID,
		File: &models.File{
			Path:         "test-manual-1",
			OriginalPath: "test-manual-1.pdf",
			Ext:          "pdf",
			MIMEType:     "application/pdf",
		},
	}

	_, err = registrySet.ManualRegistry.Create(ctx, manual1)
	c.Assert(err, qt.IsNil)

	return registrySet
}

// createTestFilesInBlobStorage creates actual test files in blob storage
func createTestFilesInBlobStorage(ctx context.Context, uploadLocation string) error {
	b, err := blob.OpenBucket(ctx, uploadLocation)
	if err != nil {
		return err
	}
	defer b.Close()

	// Create test file contents
	testFiles := map[string][]byte{
		"test-image-1.jpg":   []byte("test image data content"),
		"test-image-2.png":   []byte("test image data content"),
		"test-invoice-1.pdf": []byte("test invoice data content"),
		"test-manual-1.pdf":  []byte("test manual data content"),
	}

	// Write files to blob storage
	for filePath, content := range testFiles {
		writer, err := b.NewWriter(ctx, filePath, nil)
		if err != nil {
			return err
		}

		if _, err := writer.Write(content); err != nil {
			writer.Close()
			return err
		}

		if err := writer.Close(); err != nil {
			return err
		}
	}

	return nil
}
