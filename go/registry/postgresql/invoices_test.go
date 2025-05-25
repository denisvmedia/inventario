package postgresql_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

// TestInvoiceRegistry_Create_HappyPath tests successful invoice creation scenarios.
func TestInvoiceRegistry_Create_HappyPath(t *testing.T) {
	testCases := []struct {
		name    string
		invoice models.Invoice
	}{
		{
			name: "basic invoice",
			invoice: models.Invoice{
				File: &models.File{
					Path:         "test-invoice",
					OriginalPath: "test-invoice.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
		{
			name: "invoice with special characters",
			invoice: models.Invoice{
				File: &models.File{
					Path:         "factura-café",
					OriginalPath: "factura café.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Create test hierarchy
			location := createTestLocation(c, registrySet.LocationRegistry)
			area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
			commodity := createTestCommodity(c, registrySet, area.GetID())
			tc.invoice.CommodityID = commodity.GetID()

			// Create invoice
			createdInvoice, err := registrySet.InvoiceRegistry.Create(ctx, tc.invoice)
			c.Assert(err, qt.IsNil)
			c.Assert(createdInvoice, qt.IsNotNil)
			c.Assert(createdInvoice.GetID(), qt.Not(qt.Equals), "")
			c.Assert(createdInvoice.CommodityID, qt.Equals, tc.invoice.CommodityID)
			c.Assert(createdInvoice.File.Path, qt.Equals, tc.invoice.File.Path)
			c.Assert(createdInvoice.File.OriginalPath, qt.Equals, tc.invoice.File.OriginalPath)
			c.Assert(createdInvoice.File.Ext, qt.Equals, tc.invoice.File.Ext)
			c.Assert(createdInvoice.File.MIMEType, qt.Equals, tc.invoice.File.MIMEType)

			// Verify count
			count, err := registrySet.InvoiceRegistry.Count(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(count, qt.Equals, 1)
		})
	}
}

// TestInvoiceRegistry_Create_UnhappyPath tests invoice creation error scenarios.
func TestInvoiceRegistry_Create_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name    string
		invoice models.Invoice
	}{
		{
			name: "empty commodity ID",
			invoice: models.Invoice{
				CommodityID: "",
				File: &models.File{
					Path:         "test-invoice",
					OriginalPath: "test-invoice.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
		{
			name: "non-existent commodity ID",
			invoice: models.Invoice{
				CommodityID: "non-existent-commodity",
				File: &models.File{
					Path:         "test-invoice",
					OriginalPath: "test-invoice.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
		{
			name: "nil file",
			invoice: models.Invoice{
				CommodityID: "some-commodity-id",
				File:        nil,
			},
		},
		{
			name: "empty path",
			invoice: models.Invoice{
				CommodityID: "some-commodity-id",
				File: &models.File{
					Path:         "",
					OriginalPath: "test-invoice.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// For valid commodity ID tests, create test hierarchy
			if tc.invoice.CommodityID != "" && tc.invoice.CommodityID != "non-existent-commodity" {
				location := createTestLocation(c, registrySet.LocationRegistry)
				area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
				commodity := createTestCommodity(c, registrySet, area.GetID())
				tc.invoice.CommodityID = commodity.GetID()
			}

			// Attempt to create invalid invoice
			_, err := registrySet.InvoiceRegistry.Create(ctx, tc.invoice)
			c.Assert(err, qt.IsNotNil)

			// Verify count remains zero
			count, err := registrySet.InvoiceRegistry.Count(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(count, qt.Equals, 0)
		})
	}
}

// TestInvoiceRegistry_Get_HappyPath tests successful invoice retrieval scenarios.
func TestInvoiceRegistry_Get_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())
	invoice := createTestInvoice(c, registrySet.InvoiceRegistry, commodity.GetID())

	// Get the invoice
	retrievedInvoice, err := registrySet.InvoiceRegistry.Get(ctx, invoice.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedInvoice, qt.IsNotNil)
	c.Assert(retrievedInvoice.GetID(), qt.Equals, invoice.GetID())
	c.Assert(retrievedInvoice.CommodityID, qt.Equals, invoice.CommodityID)
	c.Assert(retrievedInvoice.File.Path, qt.Equals, invoice.File.Path)
	c.Assert(retrievedInvoice.File.OriginalPath, qt.Equals, invoice.File.OriginalPath)
	c.Assert(retrievedInvoice.File.Ext, qt.Equals, invoice.File.Ext)
	c.Assert(retrievedInvoice.File.MIMEType, qt.Equals, invoice.File.MIMEType)
}

// TestInvoiceRegistry_Get_UnhappyPath tests invoice retrieval error scenarios.
func TestInvoiceRegistry_Get_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name string
		id   string
	}{
		{
			name: "non-existent ID",
			id:   "non-existent-id",
		},
		{
			name: "empty ID",
			id:   "",
		},
		{
			name: "UUID format but non-existent",
			id:   "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Try to get non-existent invoice
			_, err := registrySet.InvoiceRegistry.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err, qt.ErrorMatches, ".*not found.*")
		})
	}
}

// TestInvoiceRegistry_Update_HappyPath tests successful invoice update scenarios.
func TestInvoiceRegistry_Update_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())
	invoice := createTestInvoice(c, registrySet.InvoiceRegistry, commodity.GetID())

	// Update the invoice
	invoice.File.Path = "updated-invoice"

	updatedInvoice, err := registrySet.InvoiceRegistry.Update(ctx, *invoice)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedInvoice, qt.IsNotNil)
	c.Assert(updatedInvoice.GetID(), qt.Equals, invoice.GetID())
	c.Assert(updatedInvoice.File.Path, qt.Equals, "updated-invoice")

	// Verify the update persisted
	retrievedInvoice, err := registrySet.InvoiceRegistry.Get(ctx, invoice.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedInvoice.File.Path, qt.Equals, "updated-invoice")
}

// TestInvoiceRegistry_Update_UnhappyPath tests invoice update error scenarios.
func TestInvoiceRegistry_Update_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name    string
		invoice models.Invoice
	}{
		{
			name: "non-existent invoice",
			invoice: models.Invoice{
				EntityID:    models.EntityID{ID: "non-existent-id"},
				CommodityID: "some-commodity-id",
				File: &models.File{
					Path:         "test-invoice",
					OriginalPath: "test-invoice.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
		{
			name: "empty path",
			invoice: models.Invoice{
				EntityID:    models.EntityID{ID: "some-id"},
				CommodityID: "some-commodity-id",
				File: &models.File{
					Path:         "",
					OriginalPath: "test-invoice.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Try to update with invalid data
			_, err := registrySet.InvoiceRegistry.Update(ctx, tc.invoice)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestInvoiceRegistry_Delete_HappyPath tests successful invoice deletion scenarios.
func TestInvoiceRegistry_Delete_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())
	invoice := createTestInvoice(c, registrySet.InvoiceRegistry, commodity.GetID())

	// Verify invoice exists
	_, err := registrySet.InvoiceRegistry.Get(ctx, invoice.GetID())
	c.Assert(err, qt.IsNil)

	// Delete the invoice
	err = registrySet.InvoiceRegistry.Delete(ctx, invoice.GetID())
	c.Assert(err, qt.IsNil)

	// Verify invoice is deleted
	_, err = registrySet.InvoiceRegistry.Get(ctx, invoice.GetID())
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify count is zero
	count, err := registrySet.InvoiceRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

// TestInvoiceRegistry_Delete_UnhappyPath tests invoice deletion error scenarios.
func TestInvoiceRegistry_Delete_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name string
		id   string
	}{
		{
			name: "non-existent ID",
			id:   "non-existent-id",
		},
		{
			name: "empty ID",
			id:   "",
		},
		{
			name: "UUID format but non-existent",
			id:   "550e8400-e29b-41d4-a716-446655440000",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// Try to delete non-existent invoice
			err := registrySet.InvoiceRegistry.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestInvoiceRegistry_List_HappyPath tests successful invoice listing scenarios.
func TestInvoiceRegistry_List_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test empty list
	invoices, err := registrySet.InvoiceRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(invoices, qt.HasLen, 0)

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())
	invoice1 := createTestInvoice(c, registrySet.InvoiceRegistry, commodity.GetID())

	invoice2 := models.Invoice{
		CommodityID: commodity.GetID(),
		File: &models.File{
			Path:         "second-invoice",
			OriginalPath: "second-invoice.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}
	createdInvoice2, err := registrySet.InvoiceRegistry.Create(ctx, invoice2)
	c.Assert(err, qt.IsNil)

	// List all invoices
	invoices, err = registrySet.InvoiceRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(invoices, qt.HasLen, 2)

	// Verify invoices are in the list
	invoiceIDs := make(map[string]bool)
	for _, invoice := range invoices {
		invoiceIDs[invoice.GetID()] = true
	}
	c.Assert(invoiceIDs[invoice1.GetID()], qt.IsTrue)
	c.Assert(invoiceIDs[createdInvoice2.GetID()], qt.IsTrue)
}

// TestInvoiceRegistry_Count_HappyPath tests successful invoice counting scenarios.
func TestInvoiceRegistry_Count_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test empty count
	count, err := registrySet.InvoiceRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())
	createTestInvoice(c, registrySet.InvoiceRegistry, commodity.GetID())

	count, err = registrySet.InvoiceRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)

	// Create another invoice
	invoice2 := models.Invoice{
		CommodityID: commodity.GetID(),
		File: &models.File{
			Path:         "second-invoice",
			OriginalPath: "second-invoice.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}
	_, err = registrySet.InvoiceRegistry.Create(ctx, invoice2)
	c.Assert(err, qt.IsNil)

	count, err = registrySet.InvoiceRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

// TestInvoiceRegistry_CascadeDelete tests that deleting a commodity cascades to invoices.
func TestInvoiceRegistry_CascadeDelete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet, area.GetID())
	invoice := createTestInvoice(c, registrySet.InvoiceRegistry, commodity.GetID())

	// Verify invoice exists
	_, err := registrySet.InvoiceRegistry.Get(ctx, invoice.GetID())
	c.Assert(err, qt.IsNil)

	// Delete the commodity (should cascade to invoice)
	err = registrySet.CommodityRegistry.Delete(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)

	// Verify invoice is also deleted due to cascade
	_, err = registrySet.InvoiceRegistry.Get(ctx, invoice.GetID())
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify count is zero
	count, err := registrySet.InvoiceRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}
