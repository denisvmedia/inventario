package postgresql_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
)

// TestCommodityRegistry_Create_HappyPath tests successful commodity creation scenarios.
func TestCommodityRegistry_Create_HappyPath(t *testing.T) {
	testCases := []struct {
		name      string
		commodity models.Commodity
	}{
		{
			name: "basic commodity",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				RegisteredDate:         models.ToPDate("2023-01-02"),
				LastModifiedDate:       models.ToPDate("2023-01-03"),
				Draft:                  false,
			},
		},
		{
			name: "commodity with all fields",
			commodity: models.Commodity{
				Name:                   "Complete Commodity",
				ShortName:              "CC",
				Type:                   models.CommodityTypeWhiteGoods,
				Count:                  2,
				OriginalPrice:          decimal.NewFromFloat(250.50),
				OriginalPriceCurrency:  "EUR",
				ConvertedOriginalPrice: decimal.NewFromFloat(275.00),
				CurrentPrice:           decimal.NewFromFloat(200.00),
				SerialNumber:           "SN123456",
				ExtraSerialNumbers:     []string{"SN654321", "SN789012"},
				PartNumbers:            []string{"P123", "P456"},
				Tags:                   []string{"tag1", "tag2", "tag3"},
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				RegisteredDate:         models.ToPDate("2023-01-02"),
				LastModifiedDate:       models.ToPDate("2023-01-03"),
				Comments:               "Test comments",
				Draft:                  false,
			},
		},
		{
			name: "draft commodity",
			commodity: models.Commodity{
				Name:      "Draft Commodity",
				ShortName: "DC",
				Type:      models.CommodityTypeOther,
				Count:     1,
				Status:    models.CommodityStatusInUse,
				Draft:     true,
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
			tc.commodity.AreaID = area.GetID()

			// Create commodity
			createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, tc.commodity)
			c.Assert(err, qt.IsNil)
			c.Assert(createdCommodity, qt.IsNotNil)
			c.Assert(createdCommodity.GetID(), qt.Not(qt.Equals), "")
			c.Assert(createdCommodity.Name, qt.Equals, tc.commodity.Name)
			c.Assert(createdCommodity.ShortName, qt.Equals, tc.commodity.ShortName)
			c.Assert(createdCommodity.Type, qt.Equals, tc.commodity.Type)
			c.Assert(createdCommodity.AreaID, qt.Equals, tc.commodity.AreaID)
			c.Assert(createdCommodity.Count, qt.Equals, tc.commodity.Count)
			c.Assert(createdCommodity.Status, qt.Equals, tc.commodity.Status)
			c.Assert(createdCommodity.Draft, qt.Equals, tc.commodity.Draft)

			// Verify count
			count, err := registrySet.CommodityRegistry.Count(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(count, qt.Equals, 1)
		})
	}
}

// TestCommodityRegistry_Create_UnhappyPath tests commodity creation error scenarios.
func TestCommodityRegistry_Create_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name      string
		commodity models.Commodity
	}{
		{
			name: "empty name",
			commodity: models.Commodity{
				Name:      "",
				ShortName: "TC",
				Type:      models.CommodityTypeElectronics,
				Count:     1,
				Status:    models.CommodityStatusInUse,
			},
		},
		{
			name: "empty short name",
			commodity: models.Commodity{
				Name:      "Test Commodity",
				ShortName: "",
				Type:      models.CommodityTypeElectronics,
				Count:     1,
				Status:    models.CommodityStatusInUse,
			},
		},
		{
			name: "invalid type",
			commodity: models.Commodity{
				Name:      "Test Commodity",
				ShortName: "TC",
				Type:      "invalid_type",
				Count:     1,
				Status:    models.CommodityStatusInUse,
			},
		},
		{
			name: "zero count",
			commodity: models.Commodity{
				Name:      "Test Commodity",
				ShortName: "TC",
				Type:      models.CommodityTypeElectronics,
				Count:     0,
				Status:    models.CommodityStatusInUse,
			},
		},
		{
			name: "invalid status",
			commodity: models.Commodity{
				Name:      "Test Commodity",
				ShortName: "TC",
				Type:      models.CommodityTypeElectronics,
				Count:     1,
				Status:    "invalid_status",
			},
		},
		{
			name: "empty area ID",
			commodity: models.Commodity{
				Name:      "Test Commodity",
				ShortName: "TC",
				Type:      models.CommodityTypeElectronics,
				AreaID:    "",
				Count:     1,
				Status:    models.CommodityStatusInUse,
			},
		},
		{
			name: "non-existent area ID",
			commodity: models.Commodity{
				Name:      "Test Commodity",
				ShortName: "TC",
				Type:      models.CommodityTypeElectronics,
				AreaID:    "non-existent-area",
				Count:     1,
				Status:    models.CommodityStatusInUse,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ctx := context.Background()

			registrySet, cleanup := setupTestRegistrySet(t)
			defer cleanup()

			// For valid area ID tests, create test hierarchy
			if tc.commodity.AreaID != "" && tc.commodity.AreaID != "non-existent-area" {
				location := createTestLocation(c, registrySet.LocationRegistry)
				area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
				tc.commodity.AreaID = area.GetID()
			}

			// Attempt to create invalid commodity
			_, err := registrySet.CommodityRegistry.Create(ctx, tc.commodity)
			c.Assert(err, qt.IsNotNil)

			// Verify count remains zero
			count, err := registrySet.CommodityRegistry.Count(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(count, qt.Equals, 0)
		})
	}
}

// TestCommodityRegistry_Get_HappyPath tests successful commodity retrieval scenarios.
func TestCommodityRegistry_Get_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())

	// Get the commodity
	retrievedCommodity, err := registrySet.CommodityRegistry.Get(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedCommodity, qt.IsNotNil)
	c.Assert(retrievedCommodity.GetID(), qt.Equals, commodity.GetID())
	c.Assert(retrievedCommodity.Name, qt.Equals, commodity.Name)
	c.Assert(retrievedCommodity.ShortName, qt.Equals, commodity.ShortName)
	c.Assert(retrievedCommodity.Type, qt.Equals, commodity.Type)
	c.Assert(retrievedCommodity.AreaID, qt.Equals, commodity.AreaID)
	c.Assert(retrievedCommodity.Count, qt.Equals, commodity.Count)
	c.Assert(retrievedCommodity.Status, qt.Equals, commodity.Status)
	c.Assert(retrievedCommodity.Draft, qt.Equals, commodity.Draft)
}

// TestCommodityRegistry_Get_UnhappyPath tests commodity retrieval error scenarios.
func TestCommodityRegistry_Get_UnhappyPath(t *testing.T) {
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

			// Try to get non-existent commodity
			_, err := registrySet.CommodityRegistry.Get(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err, qt.ErrorMatches, ".*not found.*")
		})
	}
}

// TestCommodityRegistry_Update_HappyPath tests successful commodity update scenarios.
func TestCommodityRegistry_Update_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())

	// Update the commodity
	commodity.Name = "Updated Commodity"
	commodity.ShortName = "UC"
	commodity.Comments = "Updated comments"
	commodity.Tags = []string{"updated", "tags"}

	updatedCommodity, err := registrySet.CommodityRegistry.Update(ctx, *commodity)
	c.Assert(err, qt.IsNil)
	c.Assert(updatedCommodity, qt.IsNotNil)
	c.Assert(updatedCommodity.GetID(), qt.Equals, commodity.GetID())
	c.Assert(updatedCommodity.Name, qt.Equals, "Updated Commodity")
	c.Assert(updatedCommodity.ShortName, qt.Equals, "UC")
	c.Assert(updatedCommodity.Comments, qt.Equals, "Updated comments")
	c.Assert(updatedCommodity.Tags, qt.DeepEquals, []string{"updated", "tags"})

	// Verify the update persisted
	retrievedCommodity, err := registrySet.CommodityRegistry.Get(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedCommodity.Name, qt.Equals, "Updated Commodity")
	c.Assert(retrievedCommodity.ShortName, qt.Equals, "UC")
	c.Assert(retrievedCommodity.Comments, qt.Equals, "Updated comments")
}

// TestCommodityRegistry_Update_UnhappyPath tests commodity update error scenarios.
func TestCommodityRegistry_Update_UnhappyPath(t *testing.T) {
	testCases := []struct {
		name      string
		commodity models.Commodity
	}{
		{
			name: "non-existent commodity",
			commodity: models.Commodity{
				EntityID:  models.EntityID{ID: "non-existent-id"},
				Name:      "Test Commodity",
				ShortName: "TC",
				Type:      models.CommodityTypeElectronics,
				AreaID:    "some-area-id",
				Count:     1,
				Status:    models.CommodityStatusInUse,
			},
		},
		{
			name: "empty name",
			commodity: models.Commodity{
				EntityID:  models.EntityID{ID: "some-id"},
				Name:      "",
				ShortName: "TC",
				Type:      models.CommodityTypeElectronics,
				AreaID:    "some-area-id",
				Count:     1,
				Status:    models.CommodityStatusInUse,
			},
		},
		{
			name: "empty short name",
			commodity: models.Commodity{
				EntityID:  models.EntityID{ID: "some-id"},
				Name:      "Test Commodity",
				ShortName: "",
				Type:      models.CommodityTypeElectronics,
				AreaID:    "some-area-id",
				Count:     1,
				Status:    models.CommodityStatusInUse,
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
			_, err := registrySet.CommodityRegistry.Update(ctx, tc.commodity)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestCommodityRegistry_Delete_HappyPath tests successful commodity deletion scenarios.
func TestCommodityRegistry_Delete_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())

	// Verify commodity exists
	_, err := registrySet.CommodityRegistry.Get(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)

	// Delete the commodity
	err = registrySet.CommodityRegistry.Delete(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)

	// Verify commodity is deleted
	_, err = registrySet.CommodityRegistry.Get(ctx, commodity.GetID())
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify count is zero
	count, err := registrySet.CommodityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}

// TestCommodityRegistry_Delete_UnhappyPath tests commodity deletion error scenarios.
func TestCommodityRegistry_Delete_UnhappyPath(t *testing.T) {
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

			// Try to delete non-existent commodity
			err := registrySet.CommodityRegistry.Delete(ctx, tc.id)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestCommodityRegistry_List_HappyPath tests successful commodity listing scenarios.
func TestCommodityRegistry_List_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test empty list
	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 0)

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity1 := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())

	commodity2 := models.Commodity{
		Name:                   "Second Commodity",
		ShortName:              "SC",
		Type:                   models.CommodityTypeWhiteGoods,
		AreaID:                 area.GetID(),
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(200.00),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.NewFromFloat(180.00),
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           models.ToPDate("2023-02-01"),
		RegisteredDate:         models.ToPDate("2023-02-02"),
		LastModifiedDate:       models.ToPDate("2023-02-03"),
		Draft:                  false,
	}
	createdCommodity2, err := registrySet.CommodityRegistry.Create(ctx, commodity2)
	c.Assert(err, qt.IsNil)

	// List all commodities
	commodities, err = registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, 2)

	// Verify commodities are in the list
	commodityIDs := make(map[string]bool)
	for _, commodity := range commodities {
		commodityIDs[commodity.GetID()] = true
	}
	c.Assert(commodityIDs[commodity1.GetID()], qt.IsTrue)
	c.Assert(commodityIDs[createdCommodity2.GetID()], qt.IsTrue)
}

// TestCommodityRegistry_Count_HappyPath tests successful commodity counting scenarios.
func TestCommodityRegistry_Count_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test empty count
	count, err := registrySet.CommodityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())

	count, err = registrySet.CommodityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)

	// Create another commodity
	commodity2 := models.Commodity{
		Name:                   "Second Commodity",
		ShortName:              "SC",
		Type:                   models.CommodityTypeWhiteGoods,
		AreaID:                 area.GetID(),
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(200.00),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.NewFromFloat(180.00),
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           models.ToPDate("2023-02-01"),
		RegisteredDate:         models.ToPDate("2023-02-02"),
		LastModifiedDate:       models.ToPDate("2023-02-03"),
		Draft:                  false,
	}
	_, err = registrySet.CommodityRegistry.Create(ctx, commodity2)
	c.Assert(err, qt.IsNil)

	count, err = registrySet.CommodityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

// TestCommodityRegistry_Images_HappyPath tests commodity-image relationship management.
func TestCommodityRegistry_Images_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())

	// Initially no images
	images, err := registrySet.CommodityRegistry.GetImages(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(images, qt.HasLen, 0)

	// Create an image
	image := createTestImage(c, registrySet.ImageRegistry, commodity.GetID())

	// Add image to commodity
	err = registrySet.CommodityRegistry.AddImage(ctx, commodity.GetID(), image.GetID())
	c.Assert(err, qt.IsNil)

	// Verify image is added
	images, err = registrySet.CommodityRegistry.GetImages(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(images, qt.HasLen, 1)
	c.Assert(images[0], qt.Equals, image.GetID())

	// Remove image from commodity
	err = registrySet.CommodityRegistry.DeleteImage(ctx, commodity.GetID(), image.GetID())
	c.Assert(err, qt.IsNil)

	// Verify image is removed
	images, err = registrySet.CommodityRegistry.GetImages(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(images, qt.HasLen, 0)
}

// TestCommodityRegistry_Manuals_HappyPath tests commodity-manual relationship management.
func TestCommodityRegistry_Manuals_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())

	// Initially no manuals
	manuals, err := registrySet.CommodityRegistry.GetManuals(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(manuals, qt.HasLen, 0)

	// Create a manual
	manual := createTestManual(c, registrySet.ManualRegistry, commodity.GetID())

	// Add manual to commodity
	err = registrySet.CommodityRegistry.AddManual(ctx, commodity.GetID(), manual.GetID())
	c.Assert(err, qt.IsNil)

	// Verify manual is added
	manuals, err = registrySet.CommodityRegistry.GetManuals(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(manuals, qt.HasLen, 1)
	c.Assert(manuals[0], qt.Equals, manual.GetID())

	// Remove manual from commodity
	err = registrySet.CommodityRegistry.DeleteManual(ctx, commodity.GetID(), manual.GetID())
	c.Assert(err, qt.IsNil)

	// Verify manual is removed
	manuals, err = registrySet.CommodityRegistry.GetManuals(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(manuals, qt.HasLen, 0)
}

// TestCommodityRegistry_Invoices_HappyPath tests commodity-invoice relationship management.
func TestCommodityRegistry_Invoices_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())

	// Initially no invoices
	invoices, err := registrySet.CommodityRegistry.GetInvoices(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(invoices, qt.HasLen, 0)

	// Create an invoice
	invoice := createTestInvoice(c, registrySet.InvoiceRegistry, commodity.GetID())

	// Add invoice to commodity
	err = registrySet.CommodityRegistry.AddInvoice(ctx, commodity.GetID(), invoice.GetID())
	c.Assert(err, qt.IsNil)

	// Verify invoice is added
	invoices, err = registrySet.CommodityRegistry.GetInvoices(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(invoices, qt.HasLen, 1)
	c.Assert(invoices[0], qt.Equals, invoice.GetID())

	// Remove invoice from commodity
	err = registrySet.CommodityRegistry.DeleteInvoice(ctx, commodity.GetID(), invoice.GetID())
	c.Assert(err, qt.IsNil)

	// Verify invoice is removed
	invoices, err = registrySet.CommodityRegistry.GetInvoices(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
	c.Assert(invoices, qt.HasLen, 0)
}

// TestCommodityRegistry_Relationships_UnhappyPath tests commodity relationship error scenarios.
func TestCommodityRegistry_Relationships_UnhappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Test with non-existent commodity
	err := registrySet.CommodityRegistry.AddImage(ctx, "non-existent-commodity", "some-image")
	c.Assert(err, qt.IsNotNil)

	err = registrySet.CommodityRegistry.AddManual(ctx, "non-existent-commodity", "some-manual")
	c.Assert(err, qt.IsNotNil)

	err = registrySet.CommodityRegistry.AddInvoice(ctx, "non-existent-commodity", "some-invoice")
	c.Assert(err, qt.IsNotNil)

	// Test getting relationships for non-existent commodity
	_, err = registrySet.CommodityRegistry.GetImages(ctx, "non-existent-commodity")
	c.Assert(err, qt.IsNotNil)

	_, err = registrySet.CommodityRegistry.GetManuals(ctx, "non-existent-commodity")
	c.Assert(err, qt.IsNotNil)

	_, err = registrySet.CommodityRegistry.GetInvoices(ctx, "non-existent-commodity")
	c.Assert(err, qt.IsNotNil)

	// Test deleting relationships from non-existent commodity
	err = registrySet.CommodityRegistry.DeleteImage(ctx, "non-existent-commodity", "some-image")
	c.Assert(err, qt.IsNotNil)

	err = registrySet.CommodityRegistry.DeleteManual(ctx, "non-existent-commodity", "some-manual")
	c.Assert(err, qt.IsNotNil)

	err = registrySet.CommodityRegistry.DeleteInvoice(ctx, "non-existent-commodity", "some-invoice")
	c.Assert(err, qt.IsNotNil)
}

// TestCommodityRegistry_CascadeDelete tests that deleting an area cascades to commodities.
func TestCommodityRegistry_CascadeDelete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	// Create test hierarchy
	location := createTestLocation(c, registrySet.LocationRegistry)
	area := createTestArea(c, registrySet.AreaRegistry, location.GetID())
	commodity := createTestCommodity(c, registrySet.CommodityRegistry, area.GetID())

	// Verify commodity exists
	_, err := registrySet.CommodityRegistry.Get(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)

	// Delete the area (should cascade to commodity)
	err = registrySet.AreaRegistry.Delete(ctx, area.GetID())
	c.Assert(err, qt.IsNil)

	// Verify commodity is also deleted due to cascade
	_, err = registrySet.CommodityRegistry.Get(ctx, commodity.GetID())
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify count is zero
	count, err := registrySet.CommodityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)
}
