package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
)

func TestCommodityDeleteRecursive_Integration(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	params := newParams()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}

	// Create a commodity first
	commodity := models.Commodity{
		Name:                  "Test Commodity for Deletion",
		ShortName:             "TestDel",
		Type:                  models.CommodityTypeElectronics,
		Status:                models.CommodityStatusInUse,
		Count:                 1,
		OriginalPrice:         decimal.NewFromFloat(100.00),
		OriginalPriceCurrency: "USD",
		CurrentPrice:          decimal.NewFromFloat(80.00),
	}

	// Get the first area to link the commodity to
	areas, err := params.RegistrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Not(qt.Equals), 0)
	commodity.AreaID = areas[0].ID

	createdCommodity, err := params.RegistrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Create test files linked to the commodity
	now := time.Now()

	// Create an image file
	imageFile := models.FileEntity{
		Title:            "Test Image for Deletion",
		Description:      "A test image file that should be deleted",
		Type:             models.FileTypeImage,
		Tags:             []string{"test", "deletion"},
		LinkedEntityType: "commodity",
		LinkedEntityID:   createdCommodity.ID,
		LinkedEntityMeta: "images",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "test-image-delete",
			OriginalPath: "test-image-delete.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}
	createdImageFile, err := params.RegistrySet.FileRegistry.Create(ctx, imageFile)
	c.Assert(err, qt.IsNil)

	// Create a manual file
	manualFile := models.FileEntity{
		Title:            "Test Manual for Deletion",
		Description:      "A test manual file that should be deleted",
		Type:             models.FileTypeDocument,
		Tags:             []string{"test", "deletion"},
		LinkedEntityType: "commodity",
		LinkedEntityID:   createdCommodity.ID,
		LinkedEntityMeta: "manuals",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "test-manual-delete",
			OriginalPath: "test-manual-delete.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}
	createdManualFile, err := params.RegistrySet.FileRegistry.Create(ctx, manualFile)
	c.Assert(err, qt.IsNil)

	// Create an invoice file
	invoiceFile := models.FileEntity{
		Title:            "Test Invoice for Deletion",
		Description:      "A test invoice file that should be deleted",
		Type:             models.FileTypeDocument,
		Tags:             []string{"test", "deletion"},
		LinkedEntityType: "commodity",
		LinkedEntityID:   createdCommodity.ID,
		LinkedEntityMeta: "invoices",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "test-invoice-delete",
			OriginalPath: "test-invoice-delete.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}
	createdInvoiceFile, err := params.RegistrySet.FileRegistry.Create(ctx, invoiceFile)
	c.Assert(err, qt.IsNil)

	// Verify all files exist and are linked to the commodity
	files, err := params.RegistrySet.FileRegistry.ListByLinkedEntity(ctx, "commodity", createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.HasLen, 3)

	// Verify each file type exists
	imageFiles, err := params.RegistrySet.FileRegistry.ListByLinkedEntityAndMeta(ctx, "commodity", createdCommodity.ID, "images")
	c.Assert(err, qt.IsNil)
	c.Assert(imageFiles, qt.HasLen, 1)

	manualFiles, err := params.RegistrySet.FileRegistry.ListByLinkedEntityAndMeta(ctx, "commodity", createdCommodity.ID, "manuals")
	c.Assert(err, qt.IsNil)
	c.Assert(manualFiles, qt.HasLen, 1)

	invoiceFiles, err := params.RegistrySet.FileRegistry.ListByLinkedEntityAndMeta(ctx, "commodity", createdCommodity.ID, "invoices")
	c.Assert(err, qt.IsNil)
	c.Assert(invoiceFiles, qt.HasLen, 1)

	// Now delete the commodity via API
	req, err := http.NewRequest("DELETE", "/api/v1/commodities/"+createdCommodity.ID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	// Verify the API call was successful
	c.Check(rr.Code, qt.Equals, http.StatusNoContent)

	// Verify commodity is deleted
	_, err = params.RegistrySet.CommodityRegistry.Get(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	// Verify all linked files are deleted
	files, err = params.RegistrySet.FileRegistry.ListByLinkedEntity(ctx, "commodity", createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.HasLen, 0)

	// Verify individual files are deleted
	_, err = params.RegistrySet.FileRegistry.Get(ctx, createdImageFile.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	_, err = params.RegistrySet.FileRegistry.Get(ctx, createdManualFile.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	_, err = params.RegistrySet.FileRegistry.Get(ctx, createdInvoiceFile.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	// Verify area still exists (should not be affected)
	_, err = params.RegistrySet.AreaRegistry.Get(ctx, areas[0].ID)
	c.Assert(err, qt.IsNil) // Should still exist
}

func TestCommodityDeleteRecursive_NoFiles_Integration(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	params := newParams()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}

	// Create a commodity without any files
	commodity := models.Commodity{
		Name:                  "Test Commodity No Files",
		ShortName:             "TestNoFiles",
		Type:                  models.CommodityTypeElectronics,
		Status:                models.CommodityStatusInUse,
		Count:                 1,
		OriginalPrice:         decimal.NewFromFloat(50.00),
		OriginalPriceCurrency: "USD",
		CurrentPrice:          decimal.NewFromFloat(40.00),
	}

	// Get the first area to link the commodity to
	areas, err := params.RegistrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas), qt.Not(qt.Equals), 0)
	commodity.AreaID = areas[0].ID

	createdCommodity, err := params.RegistrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Verify no files are linked to the commodity
	files, err := params.RegistrySet.FileRegistry.ListByLinkedEntity(ctx, "commodity", createdCommodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.HasLen, 0)

	// Delete the commodity via API
	req, err := http.NewRequest("DELETE", "/api/v1/commodities/"+createdCommodity.ID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	// Verify the API call was successful
	c.Check(rr.Code, qt.Equals, http.StatusNoContent)

	// Verify commodity is deleted
	_, err = params.RegistrySet.CommodityRegistry.Get(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNotNil) // Should be deleted

	// Verify area still exists (should not be affected)
	_, err = params.RegistrySet.AreaRegistry.Get(ctx, areas[0].ID)
	c.Assert(err, qt.IsNil) // Should still exist
}

func TestCommodityDeleteRecursive_NonExistent_Integration(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}

	// Try to delete a non-existent commodity
	req, err := http.NewRequest("DELETE", "/api/v1/commodities/non-existent-id", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	// Verify the API call returns 404
	c.Check(rr.Code, qt.Equals, http.StatusNotFound)

	// Verify the response contains an error
	var response jsonapi.Errors
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	c.Assert(err, qt.IsNil)
	c.Assert(len(response.Errors), qt.Not(qt.Equals), 0)
}
