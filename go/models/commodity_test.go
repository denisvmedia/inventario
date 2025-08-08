package models_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
)

func TestCommodityStatus_IsValid_HappyPaths(t *testing.T) {
	c := qt.New(t)

	testCases := []struct {
		name   string
		status models.CommodityStatus
	}{
		{"InUse", models.CommodityStatusInUse},
		{"Sold", models.CommodityStatusSold},
		{"Lost", models.CommodityStatusLost},
		{"Disposed", models.CommodityStatusDisposed},
		{"WrittenOff", models.CommodityStatusWrittenOff},
	}

	for _, tc := range testCases {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(tc.status.IsValid(), qt.IsTrue)
		})
	}
}

func TestCommodityStatus_IsValid_Invalid(t *testing.T) {
	c := qt.New(t)
	c.Assert(models.CommodityStatus("invalid_status").IsValid(), qt.IsFalse)
}

func TestCommodityStatus_IsValid_Empty(t *testing.T) {
	c := qt.New(t)
	c.Assert(models.CommodityStatus("").IsValid(), qt.IsFalse)
}

func TestCommodityStatus_Validate_Valid(t *testing.T) {
	c := qt.New(t)
	status := models.CommodityStatusInUse

	err := status.Validate()
	c.Assert(err, qt.IsNil)
}

func TestCommodityStatus_Validate_Invalid(t *testing.T) {
	c := qt.New(t)
	status := models.CommodityStatus("invalid_status")

	err := status.Validate()
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Equals, "invalid status")
}

func TestCommodityType_IsValid_HappyPaths(t *testing.T) {
	c := qt.New(t)

	testCases := []struct {
		name  string
		cType models.CommodityType
	}{
		{"WhiteGoods", models.CommodityTypeWhiteGoods},
		{"Electronics", models.CommodityTypeElectronics},
		{"Equipment", models.CommodityTypeEquipment},
		{"Furniture", models.CommodityTypeFurniture},
		{"Clothes", models.CommodityTypeClothes},
		{"Other", models.CommodityTypeOther},
	}

	for _, tc := range testCases {
		c.Run(tc.name, func(c *qt.C) {
			c.Assert(tc.cType.IsValid(), qt.IsTrue)
		})
	}
}

func TestCommodityType_IsValid_Invalid(t *testing.T) {
	c := qt.New(t)
	c.Assert(models.CommodityType("invalid_type").IsValid(), qt.IsFalse)
}

func TestCommodityType_IsValid_Empty(t *testing.T) {
	c := qt.New(t)
	c.Assert(models.CommodityType("").IsValid(), qt.IsFalse)
}

func TestCommodityType_Validate_Valid(t *testing.T) {
	c := qt.New(t)
	cType := models.CommodityTypeWhiteGoods

	err := cType.Validate()
	c.Assert(err, qt.IsNil)
}

func TestCommodityType_Validate_Invalid(t *testing.T) {
	c := qt.New(t)
	cType := models.CommodityType("invalid_type")

	err := cType.Validate()
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Equals, "invalid type")
}

func TestCommodity_Validate(t *testing.T) {
	c := qt.New(t)

	commodity := &models.Commodity{}
	err := commodity.Validate()
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Equals, "must use validate with context")
}

func TestCommodity_ValidateWithContext_HappyPaths(t *testing.T) {
	c := qt.New(t)

	testCases := []struct {
		name      string
		commodity models.Commodity
		ctxFunc   func(context.Context) context.Context
	}{
		{
			name: "Valid commodity with main currency",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "area1",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero, // Must be zero when currency is main currency
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
			},
			ctxFunc: func(ctx context.Context) context.Context {
				return validationctx.WithMainCurrency(ctx, "USD")
			},
		},
		{
			name: "Valid commodity with different currency",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "area1",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "EUR",
				ConvertedOriginalPrice: decimal.NewFromFloat(110.00),
				CurrentPrice:           decimal.NewFromFloat(105.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
			},
			ctxFunc: func(ctx context.Context) context.Context {
				return validationctx.WithMainCurrency(ctx, "USD")
			},
		},
		{
			name: "Valid draft commodity",
			commodity: models.Commodity{
				Name:                  "Draft Commodity",
				ShortName:             "DC",
				Type:                  models.CommodityTypeElectronics,
				AreaID:                "area1",
				Count:                 1,
				Status:                models.CommodityStatusInUse,
				OriginalPriceCurrency: "USD", // Need to set a valid currency
				Draft:                 true,
			},
			ctxFunc: func(ctx context.Context) context.Context {
				return validationctx.WithMainCurrency(ctx, "USD")
			},
		},
	}

	for _, tc := range testCases {
		c.Run(tc.name, func(c *qt.C) {
			ctx := tc.ctxFunc(context.Background())
			err := tc.commodity.ValidateWithContext(ctx)
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestCommodity_ValidateWithContext_UnhappyPaths(t *testing.T) {
	c := qt.New(t)

	// Create a valid commodity as a base for our tests
	validCommodity := models.Commodity{
		Name:                   "Test Commodity",
		ShortName:              "TC",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 "area1",
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(100.00),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.NewFromFloat(90.00),
		Status:                 models.CommodityStatusInUse,
	}

	testCases := []struct {
		name            string
		modifyCommodity func(*models.Commodity)
		modifyContext   func(context.Context) context.Context
		errorContains   string
	}{
		{
			name:            "Missing main currency",
			modifyCommodity: func(*models.Commodity) {},
			modifyContext: func(context.Context) context.Context {
				return context.Background() // No currency in context
			},
			errorContains: "main currency not set",
		},
		{
			name: "Missing name",
			modifyCommodity: func(c *models.Commodity) {
				c.Name = ""
			},
			modifyContext: func(ctx context.Context) context.Context {
				return validationctx.WithMainCurrency(ctx, "USD")
			},
			errorContains: "name: cannot be blank",
		},
		{
			name: "Missing short name",
			modifyCommodity: func(c *models.Commodity) {
				c.ShortName = ""
			},
			modifyContext: func(ctx context.Context) context.Context {
				return validationctx.WithMainCurrency(ctx, "USD")
			},
			errorContains: "short_name: cannot be blank",
		},
		{
			name: "Missing area ID",
			modifyCommodity: func(c *models.Commodity) {
				c.AreaID = ""
			},
			modifyContext: func(ctx context.Context) context.Context {
				return validationctx.WithMainCurrency(ctx, "USD")
			},
			errorContains: "area_id: cannot be blank",
		},
		{
			name: "Invalid count",
			modifyCommodity: func(c *models.Commodity) {
				c.Count = 0
			},
			modifyContext: func(ctx context.Context) context.Context {
				return validationctx.WithMainCurrency(ctx, "USD")
			},
			errorContains: "count: cannot be blank",
		},
		{
			name: "Missing status",
			modifyCommodity: func(c *models.Commodity) {
				c.Status = ""
			},
			modifyContext: func(ctx context.Context) context.Context {
				return validationctx.WithMainCurrency(ctx, "USD")
			},
			errorContains: "status: cannot be blank",
		},
		{
			name: "Missing purchase date",
			modifyCommodity: func(c *models.Commodity) {
				c.PurchaseDate = nil
			},
			modifyContext: func(ctx context.Context) context.Context {
				return validationctx.WithMainCurrency(ctx, "USD")
			},
			errorContains: "purchase_date: cannot be blank",
		},
	}

	for _, tc := range testCases {
		c.Run(tc.name, func(c *qt.C) {
			// Create a copy of the valid commodity for this test case
			commodity := validCommodity

			// Apply the modifications for this test case
			tc.modifyCommodity(&commodity)
			ctx := tc.modifyContext(context.Background())

			// Run the validation
			err := commodity.ValidateWithContext(ctx)

			// Check the results
			c.Assert(err, qt.Not(qt.IsNil))
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestCommodity_ValidateWithContext_PriceValidation_HappyPath(t *testing.T) {
	c := qt.New(t)

	testCases := []struct {
		name         string
		commodity    models.Commodity
		mainCurrency string
	}{
		{
			name: "Main currency with zero converted price",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "area1",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
			},
			mainCurrency: "USD",
		},
		{
			name: "Non-main currency with converted price",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "area1",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "EUR",
				ConvertedOriginalPrice: decimal.NewFromFloat(110.00),
				CurrentPrice:           decimal.NewFromFloat(105.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
			},
			mainCurrency: "USD",
		},
		{
			name: "Non-main currency with only current price",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "area1",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "EUR",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(105.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
			},
			mainCurrency: "USD",
		},
	}

	for _, tc := range testCases {
		c.Run(tc.name, func(c *qt.C) {
			ctx := validationctx.WithMainCurrency(context.Background(), tc.mainCurrency)
			err := tc.commodity.ValidateWithContext(ctx)
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestCommodity_ValidateWithContext_PriceValidation_UnhappyPaths(t *testing.T) {
	c := qt.New(t)

	testCases := []struct {
		name          string
		commodity     models.Commodity
		mainCurrency  string
		errorContains string
	}{
		{
			name: "Main currency with non-zero converted price",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "area1",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.NewFromFloat(100.00),
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
			},
			mainCurrency:  "USD",
			errorContains: "converted original price must be zero",
		},
		{
			name: "Non-main currency with zero converted price and zero current price",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "area1",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "EUR",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.Zero,
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
			},
			mainCurrency:  "USD",
			errorContains: "converted original price or current price must be set",
		},
	}

	for _, tc := range testCases {
		c.Run(tc.name, func(c *qt.C) {
			ctx := validationctx.WithMainCurrency(context.Background(), tc.mainCurrency)
			err := tc.commodity.ValidateWithContext(ctx)

			c.Assert(err, qt.Not(qt.IsNil))
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestCommodity_ValidateWithContext_NegativePrices(t *testing.T) {
	testCases := []struct {
		name          string
		commodity     models.Commodity
		shouldBeValid bool
		errorContains string
	}{
		{
			name: "Negative original price",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "area1",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(-100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
			},
			shouldBeValid: false,
			errorContains: "must be no less than 0",
		},
		{
			name: "Negative original converted price",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "area1",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.NewFromFloat(-100.00),
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
			},
			shouldBeValid: false,
			errorContains: "must be no less than 0",
		},
		{
			name: "Negative current price",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "area1",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(-90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
			},
			shouldBeValid: false,
			errorContains: "must be no less than 0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			ctx := validationctx.WithMainCurrency(context.Background(), "USD")
			err := tc.commodity.ValidateWithContext(ctx)

			c.Assert(err, qt.Not(qt.IsNil))
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestCommodity_ValidateWithContext_NameLength(t *testing.T) {
	c := qt.New(t)

	// Create a very long name
	longName := strings.Repeat("a", 256)
	longShortName := strings.Repeat("a", 30)

	testCases := []struct {
		name          string
		commodity     models.Commodity
		errorContains string
	}{
		{
			name: "Too long name",
			commodity: models.Commodity{
				Name:                   longName,
				ShortName:              "TC",
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "area1",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
			},
			errorContains: "the length must be between",
		},
		{
			name: "Too long short name",
			commodity: models.Commodity{
				Name:                   "Test Commodity",
				ShortName:              longShortName,
				Type:                   models.CommodityTypeElectronics,
				AreaID:                 "area1",
				Count:                  1,
				OriginalPrice:          decimal.NewFromFloat(100.00),
				OriginalPriceCurrency:  "USD",
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.NewFromFloat(90.00),
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
			},
			errorContains: "the length must be between 1 and 20",
		},
	}

	for _, tc := range testCases {
		c.Run(tc.name, func(c *qt.C) {
			ctx := validationctx.WithMainCurrency(context.Background(), "USD")
			err := tc.commodity.ValidateWithContext(ctx)

			c.Assert(err, qt.Not(qt.IsNil))
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestCommodity_JSONMarshaling(t *testing.T) {
	c := qt.New(t)

	// Create a commodity with all fields populated
	commodity := models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{
				ID: "test-id",
			},
			TenantID: "test-tenant",
		},
		Name:                   "Test Commodity",
		ShortName:              "TC",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 "area1",
		Count:                  2,
		OriginalPrice:          decimal.NewFromFloat(100.00),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.Zero, // Must be zero when currency is main currency
		CurrentPrice:           decimal.NewFromFloat(90.00),
		SerialNumber:           "SN123456",
		ExtraSerialNumbers:     []string{"SN654321", "SN789012"},
		PartNumbers:            []string{"P123", "P456"},
		Tags:                   []string{"tag1", "tag2"},
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           models.ToPDate("2023-01-01"),
		RegisteredDate:         models.ToPDate("2023-01-02"),
		LastModifiedDate:       models.ToPDate("2023-01-03"),
		URLs:                   []*models.URL{{Scheme: "http", Host: "example.com"}},
		Comments:               "Test comments",
		Draft:                  false,
	}

	// Marshal to JSON
	data, err := json.Marshal(commodity)
	c.Assert(err, qt.IsNil)

	// Unmarshal back to a new commodity
	var newCommodity models.Commodity
	err = json.Unmarshal(data, &newCommodity)
	c.Assert(err, qt.IsNil)

	// Verify fields match
	c.Assert(newCommodity.ID, qt.Equals, commodity.ID)
	c.Assert(newCommodity.Name, qt.Equals, commodity.Name)
	c.Assert(newCommodity.ShortName, qt.Equals, commodity.ShortName)
	c.Assert(newCommodity.Type, qt.Equals, commodity.Type)
	c.Assert(newCommodity.AreaID, qt.Equals, commodity.AreaID)
	c.Assert(newCommodity.Count, qt.Equals, commodity.Count)
	c.Assert(newCommodity.OriginalPrice.Equal(commodity.OriginalPrice), qt.IsTrue)
	c.Assert(newCommodity.OriginalPriceCurrency, qt.Equals, commodity.OriginalPriceCurrency)
	c.Assert(newCommodity.ConvertedOriginalPrice.Equal(commodity.ConvertedOriginalPrice), qt.IsTrue)
	c.Assert(newCommodity.CurrentPrice.Equal(commodity.CurrentPrice), qt.IsTrue)
	c.Assert(newCommodity.SerialNumber, qt.Equals, commodity.SerialNumber)
	c.Assert(newCommodity.ExtraSerialNumbers, qt.DeepEquals, commodity.ExtraSerialNumbers)
	c.Assert(newCommodity.PartNumbers, qt.DeepEquals, commodity.PartNumbers)
	c.Assert(newCommodity.Tags, qt.DeepEquals, commodity.Tags)
	c.Assert(newCommodity.Status, qt.Equals, commodity.Status)

	// For PDate fields, we need to compare the string values since they're pointers
	// Check that both PurchaseDate pointers are non-nil
	c.Assert(commodity.PurchaseDate, qt.IsNotNil)
	c.Assert(newCommodity.PurchaseDate, qt.IsNotNil)
	// Compare the string values
	c.Assert(string(*newCommodity.PurchaseDate), qt.Equals, string(*commodity.PurchaseDate))

	// Check that both RegisteredDate pointers are non-nil
	c.Assert(commodity.RegisteredDate, qt.IsNotNil)
	c.Assert(newCommodity.RegisteredDate, qt.IsNotNil)
	// Compare the string values
	c.Assert(string(*newCommodity.RegisteredDate), qt.Equals, string(*commodity.RegisteredDate))

	// Check that both LastModifiedDate pointers are non-nil
	c.Assert(commodity.LastModifiedDate, qt.IsNotNil)
	c.Assert(newCommodity.LastModifiedDate, qt.IsNotNil)
	// Compare the string values
	c.Assert(string(*newCommodity.LastModifiedDate), qt.Equals, string(*commodity.LastModifiedDate))

	c.Assert(newCommodity.Comments, qt.Equals, commodity.Comments)
	c.Assert(newCommodity.Draft, qt.Equals, commodity.Draft)
}
