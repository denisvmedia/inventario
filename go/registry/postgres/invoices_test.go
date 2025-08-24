package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestInvoiceRegistry_Create_HappyPath(t *testing.T) {
	testCases := []struct {
		name    string
		invoice models.Invoice
	}{
		{
			name: "valid invoice with all fields",
			invoice: models.Invoice{
				File: &models.File{
					Path:         "test-invoice",
					OriginalPath: "test-invoice.pdf",
					Ext:          ".pdf",
					MIMEType:     "application/pdf",
				},
				TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-invoice-id", "test-tenant-id", "test-user-id"),
			},
		},
		{
			name: "valid invoice with different format",
			invoice: models.Invoice{
				File: &models.File{
					Path:         "another-invoice",
					OriginalPath: "another-invoice.docx",
					Ext:          ".docx",
					MIMEType:     "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
				},
				TenantAwareEntityID: models.WithTenantUserAwareEntityID("test-invoice-id2", "test-tenant-id", "test-user-id"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			locationReg, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			areaReg, err := registrySet.AreaRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			invoiceReg, err := registrySet.InvoiceRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)

			// Create test hierarchy
			location := createTestLocation(c, locationReg)
			area := createTestArea(c, areaReg, location.ID)
			commodity := createTestCommodity(c, registrySet, area.ID)

			// Set commodity ID
			tc.invoice.CommodityID = commodity.ID

			// Create invoice
			result, err := invoiceReg.Create(ctx, tc.invoice)
			c.Assert(err, qt.IsNil)
			c.Assert(result, qt.IsNotNil)
			c.Assert(result.ID, qt.Not(qt.Equals), "")
			c.Assert(result.CommodityID, qt.Equals, tc.invoice.CommodityID)
			c.Assert(result.File.Path, qt.Equals, tc.invoice.File.Path)
			c.Assert(result.File.OriginalPath, qt.Equals, tc.invoice.File.OriginalPath)
			c.Assert(result.File.Ext, qt.Equals, tc.invoice.File.Ext)
			c.Assert(result.File.MIMEType, qt.Equals, tc.invoice.File.MIMEType)
		})
	}
}

func TestInvoiceRegistry_Create_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name    string
		invoice models.Invoice
	}{
		{
			name: "missing commodity ID",
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
			name: "non-existent commodity",
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
			name: "missing file",
			invoice: models.Invoice{
				CommodityID: "some-commodity-id",
			},
		},
		{
			name:    "empty invoice",
			invoice: models.Invoice{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// For valid commodity ID tests, create test hierarchy
			if tc.invoice.CommodityID != "" && tc.invoice.CommodityID != "non-existent-commodity" {
				location := createTestLocation(c, registrySet.LocationRegistry)
				area := createTestArea(c, registrySet.AreaRegistry, location.ID)
				commodity := createTestCommodity(c, registrySet, area.ID)
				tc.invoice.CommodityID = commodity.ID
			}

			// Attempt to create invalid invoice
			result, err := registrySet.InvoiceRegistry.Create(ctx, tc.invoice)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestInvoiceRegistry_Get_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create test hierarchy and invoice
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	created := createTestInvoice(c, registrySet, commodity.ID)

	// Get the invoice
	result, err := registrySet.InvoiceRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.CommodityID, qt.Equals, created.CommodityID)
	c.Assert(result.File.Path, qt.Equals, created.File.Path)
	c.Assert(result.File.OriginalPath, qt.Equals, created.File.OriginalPath)
	c.Assert(result.File.Ext, qt.Equals, created.File.Ext)
	c.Assert(result.File.MIMEType, qt.Equals, created.File.MIMEType)
}

func TestInvoiceRegistry_Get_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name string
		id   string
	}{
		{
			name: "non-existent invoice",
			id:   "non-existent-id",
		},
		{
			name: "empty ID",
			id:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			result, err := registrySet.InvoiceRegistry.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestInvoiceRegistry_List_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Initially should be empty
	invoices, err := registrySet.InvoiceRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invoices), qt.Equals, 0)

	// Create test hierarchy and invoices
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	invoice1 := createTestInvoice(c, registrySet, commodity.ID)
	invoice2 := createTestInvoice(c, registrySet, commodity.ID)

	// List should now contain both invoices
	invoices, err = registrySet.InvoiceRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invoices), qt.Equals, 2)

	// Verify the invoices are correct
	invoiceIDs := make(map[string]bool)
	for _, invoice := range invoices {
		invoiceIDs[invoice.ID] = true
	}
	c.Assert(invoiceIDs[invoice1.ID], qt.IsTrue)
	c.Assert(invoiceIDs[invoice2.ID], qt.IsTrue)
}

func TestInvoiceRegistry_Update_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create test hierarchy and invoice
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	created := createTestInvoice(c, registrySet, commodity.ID)

	// Update the invoice
	created.File.Path = "updated-invoice-path"
	created.File.MIMEType = "application/vnd.ms-excel"

	result, err := registrySet.InvoiceRegistry.Update(ctx, *created)
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(result.ID, qt.Equals, created.ID)
	c.Assert(result.File.Path, qt.Equals, "updated-invoice-path")
	c.Assert(result.File.MIMEType, qt.Equals, "application/vnd.ms-excel")

	// Verify the update persisted
	retrieved, err := registrySet.InvoiceRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved.File.Path, qt.Equals, "updated-invoice-path")
	c.Assert(retrieved.File.MIMEType, qt.Equals, "application/vnd.ms-excel")
}

func TestInvoiceRegistry_Update_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name    string
		invoice models.Invoice
	}{
		{
			name: "non-existent invoice",
			invoice: models.Invoice{
				TenantAwareEntityID: models.WithTenantAwareEntityID("non-existent-id", "test-tenant-id"),
				CommodityID:         "some-commodity-id",
				File: &models.File{
					Path:         "test-invoice",
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
			ctx := c.Context()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// For valid commodity ID tests, create test hierarchy
			if tc.invoice.CommodityID != "" && tc.invoice.CommodityID != "non-existent-commodity" {
				location := createTestLocation(c, registrySet.LocationRegistry)
				area := createTestArea(c, registrySet.AreaRegistry, location.ID)
				commodity := createTestCommodity(c, registrySet, area.ID)
				tc.invoice.CommodityID = commodity.ID
			}

			// Attempt to update non-existent invoice
			result, err := registrySet.InvoiceRegistry.Update(ctx, tc.invoice)
			c.Assert(err, qt.IsNotNil)
			c.Assert(result, qt.IsNil)
		})
	}
}

func TestInvoiceRegistry_Delete_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Create test hierarchy and invoice
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	created := createTestInvoice(c, registrySet, commodity.ID)

	// Delete the invoice
	err := registrySet.InvoiceRegistry.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	// Verify the invoice is deleted
	result, err := registrySet.InvoiceRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(result, qt.IsNil)
}

func TestInvoiceRegistry_Delete_UnhappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	testCases := []struct {
		name string
		id   string
	}{
		{
			name: "non-existent invoice",
			id:   "non-existent-id",
		},
		{
			name: "empty ID",
			id:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := c.Context()

			err := registrySet.InvoiceRegistry.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestInvoiceRegistry_Count_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := c.Context()

	// Initially should be 0
	count, err := registrySet.InvoiceRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test hierarchy and invoices
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.ID)
	commodity := createTestCommodity(c, registrySet, area.ID)
	createTestInvoice(c, registrySet, commodity.ID)
	createTestInvoice(c, registrySet, commodity.ID)

	// Count should now be 2
	count, err = registrySet.InvoiceRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}
