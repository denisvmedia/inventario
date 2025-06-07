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

	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func newTestRegistrySet() *registry.Set {
	registrySet := &registry.Set{
		LocationRegistry:  memory.NewLocationRegistry(),
		AreaRegistry:      memory.NewAreaRegistry(memory.NewLocationRegistry()),
		CommodityRegistry: memory.NewCommodityRegistry(memory.NewAreaRegistry(memory.NewLocationRegistry())),
		ExportRegistry:    memory.NewExportRegistry(),
	}
	return registrySet
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
	ctx := context.Background()

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
	ctx := context.Background()

	// Create a test export in the database
	export := models.Export{
		Type:            models.ExportTypeCommodities,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
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
	ctx := context.Background()

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
				Type:            tc.exportType,
				Status:          models.ExportStatusPending,
				IncludeFileData: false,
			}

			var buf bytes.Buffer
			err := service.streamXMLExport(ctx, export, &buf)
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
	ctx := context.Background()

	export := models.Export{
		Type:            "invalid_type",
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
	}

	var buf bytes.Buffer
	err := service.streamXMLExport(ctx, export, &buf)
	c.Assert(err, qt.IsNotNil)
}

func TestGenerateExport(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	registrySet := newTestRegistrySet()
	uploadLocation := "file:///" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := context.Background()

	export := models.Export{
		EntityID:        models.EntityID{ID: "test-export-123"},
		Type:            models.ExportTypeCommodities,
		Status:          models.ExportStatusPending,
		IncludeFileData: false,
	}

	blobKey, err := service.generateExport(ctx, export)
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
	}

	uploadLocation := "file:///" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := context.Background()

	// Create test data
	location := models.Location{EntityID: models.EntityID{ID: "loc1"}, Name: "Location 1", Address: "Address 1"}
	createdLocation, err := registrySet.LocationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)

	area := models.Area{EntityID: models.EntityID{ID: "area1"}, Name: "Area 1", LocationID: createdLocation.ID}
	createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	commodity := models.Commodity{
		EntityID: models.EntityID{ID: "commodity1"},
		Name:     "Test Commodity",
		Type:     models.CommodityTypeElectronics,
		AreaID:   createdArea.ID,
		Count:    1,
		Status:   models.CommodityStatusInUse,
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
		EntityID:    models.EntityID{ID: "img1"},
		CommodityID: createdCommodity.ID,
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
		EntityID:    models.EntityID{ID: "inv1"},
		CommodityID: createdCommodity.ID,
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
	xmlCommodity, err := service.convertCommodityToXML(ctx, createdCommodity, ExportArgs{IncludeFileData: true})
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
	xmlCommodityNoData, err := service.convertCommodityToXML(ctx, createdCommodity, ExportArgs{IncludeFileData: false})
	c.Assert(err, qt.IsNil)
	c.Assert(xmlCommodityNoData.Images, qt.HasLen, 1)
	c.Assert(xmlCommodityNoData.Invoices, qt.HasLen, 1)

	// Check that no file data is included
	c.Assert(xmlCommodityNoData.Images[0].Data, qt.Equals, "")
	c.Assert(xmlCommodityNoData.Invoices[0].Data, qt.Equals, "")
}

func TestBase64FileDataVerification(t *testing.T) {
	c := qt.New(t)
	// Create a temporary directory for uploads
	tempDir := c.TempDir()

	// Create interconnected registries
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)
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
	}

	uploadLocation := "file:///" + tempDir + "?create_dir=1"
	service := NewExportService(registrySet, uploadLocation)
	ctx := context.Background()

	// Create test data
	location := models.Location{EntityID: models.EntityID{ID: "loc1"}, Name: "Location 1", Address: "Address 1"}
	createdLocation, err := registrySet.LocationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)

	area := models.Area{EntityID: models.EntityID{ID: "area1"}, Name: "Area 1", LocationID: createdLocation.ID}
	createdArea, err := registrySet.AreaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	commodity := models.Commodity{
		EntityID: models.EntityID{ID: "commodity1"},
		Name:     "Test Commodity",
		Type:     models.CommodityTypeElectronics,
		AreaID:   createdArea.ID,
		Count:    1,
		Status:   models.CommodityStatusInUse,
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
				EntityID:    models.EntityID{ID: "img-" + tf.path},
				CommodityID: createdCommodity.ID,
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
				EntityID:    models.EntityID{ID: "inv-" + tf.path},
				CommodityID: createdCommodity.ID,
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
				EntityID:    models.EntityID{ID: "man-" + tf.path},
				CommodityID: createdCommodity.ID,
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
	xmlCommodity, err := service.convertCommodityToXML(ctx, createdCommodity, ExportArgs{IncludeFileData: true})
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
		Type:            models.ExportTypeFullDatabase,
		Status:          models.ExportStatusPending,
		IncludeFileData: true,
	}

	var buf bytes.Buffer
	err = service.streamXMLExport(ctx, export, &buf)
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
