package seeddata

import (
	"github.com/go-extras/go-kit/ptr"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// SeedData seeds the database with example data.
func SeedData(registrySet *registry.Set) error { //nolint:funlen,gocyclo // it's a seed function
	// Create default system configuration with CZK as main currency
	systemConfig := models.SettingsObject{
		MainCurrency: ptr.To("CZK"),
	}

	err := registrySet.SettingsRegistry.Save(systemConfig)
	if err != nil {
		return err
	}

	// Create locations
	home, err := registrySet.LocationRegistry.Create(models.Location{
		Name:    "Home",
		Address: "123 Main St, Anytown, USA",
	})
	if err != nil {
		return err
	}

	office, err := registrySet.LocationRegistry.Create(models.Location{
		Name:    "Office",
		Address: "456 Business Ave, Worktown, USA",
	})
	if err != nil {
		return err
	}

	storage, err := registrySet.LocationRegistry.Create(models.Location{
		Name:    "Storage Unit",
		Address: "789 Storage Blvd, Storeville, USA",
	})
	if err != nil {
		return err
	}

	// Create areas for Home
	livingRoom, err := registrySet.AreaRegistry.Create(models.Area{
		Name:       "Living Room",
		LocationID: home.ID,
	})
	if err != nil {
		return err
	}

	kitchen, err := registrySet.AreaRegistry.Create(models.Area{
		Name:       "Kitchen",
		LocationID: home.ID,
	})
	if err != nil {
		return err
	}

	bedroom, err := registrySet.AreaRegistry.Create(models.Area{
		Name:       "Bedroom",
		LocationID: home.ID,
	})
	if err != nil {
		return err
	}

	// Create areas for Office
	workDesk, err := registrySet.AreaRegistry.Create(models.Area{
		Name:       "Work Desk",
		LocationID: office.ID,
	})
	if err != nil {
		return err
	}

	conferenceRoom, err := registrySet.AreaRegistry.Create(models.Area{
		Name:       "Conference Room",
		LocationID: office.ID,
	})
	if err != nil {
		return err
	}

	// Create areas for Storage
	unitA, err := registrySet.AreaRegistry.Create(models.Area{
		Name:       "Unit A",
		LocationID: storage.ID,
	})
	if err != nil {
		return err
	}

	// Create commodities for Living Room
	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                   "Smart TV",
		ShortName:              "TV",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 livingRoom.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(1299.99),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.NewFromFloat(29899.77), // Converted to CZK
		CurrentPrice:           decimal.NewFromFloat(899.99),
		SerialNumber:           "TV123456789",
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           ptr.To(models.Date("2022-01-15")),
		RegisteredDate:         ptr.To(models.Date("2022-01-16")),
		Tags:                   []string{"electronics", "entertainment"},
		Comments:               "65-inch 4K Smart TV",
	})
	if err != nil {
		return err
	}

	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                   "Sofa",
		ShortName:              "Sofa",
		Type:                   models.CommodityTypeFurniture,
		AreaID:                 livingRoom.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(899.99),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.NewFromFloat(20699.77), // Converted to CZK
		CurrentPrice:           decimal.NewFromFloat(699.99),
		SerialNumber:           "SF987654321",
		Status:                 models.CommodityStatusSold, // Changed status to Sold
		PurchaseDate:           ptr.To(models.Date("2021-11-20")),
		RegisteredDate:         ptr.To(models.Date("2021-11-25")),
		Tags:                   []string{"furniture", "living room"},
		Comments:               "3-seat sectional sofa",
	})
	if err != nil {
		return err
	}

	// Create commodities for Kitchen
	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                   "Refrigerator",
		ShortName:              "Fridge",
		Type:                   models.CommodityTypeWhiteGoods,
		AreaID:                 kitchen.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(1499.99),
		OriginalPriceCurrency:  "EUR",                          // Changed to EUR
		ConvertedOriginalPrice: decimal.NewFromFloat(37499.75), // Converted to CZK
		CurrentPrice:           decimal.NewFromFloat(27599.77), // Price in CZK
		SerialNumber:           "RF123456789",
		Status:                 models.CommodityStatusLost, // Changed status to Lost
		PurchaseDate:           ptr.To(models.Date("2022-03-10")),
		RegisteredDate:         ptr.To(models.Date("2022-03-15")),
		Tags:                   []string{"appliance", "kitchen"},
		Comments:               "French door refrigerator with ice maker",
	})
	if err != nil {
		return err
	}

	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                   "Microwave Oven",
		ShortName:              "Microwave",
		Type:                   models.CommodityTypeWhiteGoods,
		AreaID:                 kitchen.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(199.99),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.NewFromFloat(4599.77), // Converted to CZK
		CurrentPrice:           decimal.NewFromFloat(3449.77), // Price in CZK
		SerialNumber:           "MW987654321",
		Status:                 models.CommodityStatusDisposed, // Changed status to Disposed
		PurchaseDate:           ptr.To(models.Date("2022-02-05")),
		RegisteredDate:         ptr.To(models.Date("2022-02-10")),
		Tags:                   []string{"appliance", "kitchen"},
		Comments:               "1100W countertop microwave",
	})
	if err != nil {
		return err
	}

	// Create commodities for Bedroom
	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                   "Bed Frame",
		ShortName:              "Bed",
		Type:                   models.CommodityTypeFurniture,
		AreaID:                 bedroom.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(599.99),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.NewFromFloat(13799.77), // Converted to CZK
		CurrentPrice:           decimal.NewFromFloat(11499.77), // Price in CZK
		SerialNumber:           "BF123456789",
		Status:                 models.CommodityStatusWrittenOff, // Changed status to Written Off
		PurchaseDate:           ptr.To(models.Date("2021-10-15")),
		RegisteredDate:         ptr.To(models.Date("2021-10-20")),
		Tags:                   []string{"furniture", "bedroom"},
		Comments:               "Queen size bed frame",
	})
	if err != nil {
		return err
	}

	// Create commodities for Work Desk
	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                   "Laptop",
		ShortName:              "Laptop",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 workDesk.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(1299.99),
		OriginalPriceCurrency:  "EUR",                          // Changed to EUR
		ConvertedOriginalPrice: decimal.NewFromFloat(32499.75), // Converted to CZK
		CurrentPrice:           decimal.NewFromFloat(22499.75), // Price in CZK
		SerialNumber:           "LT123456789",
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           ptr.To(models.Date("2022-05-10")),
		RegisteredDate:         ptr.To(models.Date("2022-05-15")),
		Tags:                   []string{"electronics", "work"},
		Comments:               "15-inch business laptop",
		Draft:                  true, // Added draft status
	})
	if err != nil {
		return err
	}

	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                   "Monitor",
		ShortName:              "Monitor",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 workDesk.ID,
		Count:                  2,
		OriginalPrice:          decimal.NewFromFloat(349.99),
		OriginalPriceCurrency:  "EUR",                         // Changed to EUR
		ConvertedOriginalPrice: decimal.NewFromFloat(8749.75), // Converted to CZK
		CurrentPrice:           decimal.NewFromFloat(7499.75), // Price in CZK
		SerialNumber:           "MN123456789",
		ExtraSerialNumbers:     []string{"MN987654321"},
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           ptr.To(models.Date("2022-05-10")),
		RegisteredDate:         ptr.To(models.Date("2022-05-15")),
		Tags:                   []string{"electronics", "work"},
		Comments:               "27-inch 4K monitors",
		Draft:                  true, // Added draft status
	})
	if err != nil {
		return err
	}

	// Create commodities for Conference Room
	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                   "Projector",
		ShortName:              "Projector",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 conferenceRoom.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(799.99),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.NewFromFloat(18399.77), // Converted to CZK
		CurrentPrice:           decimal.NewFromFloat(16099.77), // Price in CZK
		SerialNumber:           "PJ123456789",
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           ptr.To(models.Date("2022-04-20")),
		RegisteredDate:         ptr.To(models.Date("2022-04-25")),
		Tags:                   []string{"electronics", "presentation"},
		Comments:               "4K projector for conference room",
	})
	if err != nil {
		return err
	}

	// Create commodities for Storage Unit
	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                  "Winter Clothes",
		ShortName:             "Winter",
		Type:                  models.CommodityTypeClothes,
		AreaID:                unitA.ID,
		Count:                 10,
		OriginalPrice:         decimal.NewFromFloat(1200.00),
		OriginalPriceCurrency: "CZK",                        // Changed to CZK (main currency)
		CurrentPrice:          decimal.NewFromFloat(600.00), // Price in CZK
		Status:                models.CommodityStatusInUse,
		PurchaseDate:          ptr.To(models.Date("2021-09-15")),
		RegisteredDate:        ptr.To(models.Date("2021-09-20")),
		Tags:                  []string{"clothes", "seasonal"},
		Comments:              "Winter clothes in storage",
	})
	if err != nil {
		return err
	}

	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                   "Camping Equipment",
		ShortName:              "Camping",
		Type:                   models.CommodityTypeEquipment,
		AreaID:                 unitA.ID,
		Count:                  5,
		OriginalPrice:          decimal.NewFromFloat(850.00),
		OriginalPriceCurrency:  "EUR",                          // Changed to EUR
		ConvertedOriginalPrice: decimal.NewFromFloat(21250.00), // Converted to CZK
		CurrentPrice:           decimal.NewFromFloat(17500.00), // Price in CZK
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           ptr.To(models.Date("2021-07-10")),
		RegisteredDate:         ptr.To(models.Date("2021-07-15")),
		Tags:                   []string{"outdoor", "seasonal"},
		Comments:               "Tent, sleeping bags, and other camping gear",
	})
	if err != nil {
		return err
	}

	// Create a new draft commodity with CZK as original currency
	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                  "Coffee Machine",
		ShortName:             "Coffee",
		Type:                  models.CommodityTypeWhiteGoods,
		AreaID:                kitchen.ID,
		Count:                 1,
		OriginalPrice:         decimal.NewFromFloat(4500.00),
		OriginalPriceCurrency: "CZK",                   // Main currency
		CurrentPrice:          decimal.NewFromFloat(0), // No current price
		SerialNumber:          "CM123456789",
		Status:                models.CommodityStatusInUse,
		PurchaseDate:          ptr.To(models.Date("2023-01-15")),
		RegisteredDate:        ptr.To(models.Date("2023-01-16")),
		Tags:                  []string{"appliance", "kitchen"},
		Comments:              "Espresso machine with milk frother",
		Draft:                 true, // Draft status
	})
	if err != nil {
		return err
	}

	// Create a commodity with original price in USD but no current price, only converted price
	_, err = registrySet.CommodityRegistry.Create(models.Commodity{
		Name:                   "Desk Chair",
		ShortName:              "Chair",
		Type:                   models.CommodityTypeFurniture,
		AreaID:                 workDesk.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(249.99),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.NewFromFloat(5749.77), // Converted to CZK
		CurrentPrice:           decimal.NewFromFloat(0),       // No current price
		SerialNumber:           "DC123456789",
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           ptr.To(models.Date("2022-05-10")),
		RegisteredDate:         ptr.To(models.Date("2022-05-15")),
		Tags:                   []string{"furniture", "work"},
		Comments:               "Ergonomic office chair",
	})
	if err != nil {
		return err
	}

	return nil
}
