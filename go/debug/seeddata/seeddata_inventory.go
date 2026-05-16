package seeddata

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// seedInventoryResult is what the inventory pass returns to the
// orchestrator: the created commodities (so loans / services /
// events can reference them by ID), and the canonical "first photo"
// file IDs (so the cover-override step can pin them).
type seedInventoryResult struct {
	commodities []*models.Commodity
	// commoditiesByName maps the human-readable name to the
	// commodity row — used by the loans / services / events
	// helpers to find commodities deterministically without
	// hard-coding indices.
	commoditiesByName map[string]*models.Commodity
	// locationsByName / areasByName let the history pass attach
	// location-level files to specific locations.
	locationsByName map[string]*models.Location
	areasByName     map[string]*models.Area
}

// daysFromToday returns a non-nil PDate offset from today, anchored at
// UTC midnight so the warranty status computation (which normalises to
// UTC) and the comparison string are consistent. Used for warranty
// dates and other relative-to-seed timeline fields. The result is
// always populated — pass 0 to get today's date; callers that want
// "no date set" must keep their PDate as nil before calling.
func daysFromToday(days int) *models.Date {
	t := time.Now().UTC().AddDate(0, 0, days)
	d := models.Date(t.Format("2006-01-02"))
	return &d
}

// locationSpec drives the per-location seeding loop.
type locationSpec struct {
	Name        string
	Address     string
	Icon        string
	Description string
	Areas       []areaSpec
}

// areaSpec drives the per-area commodity loop.
type areaSpec struct {
	Name string
	Icon string
}

// commoditySpec is the dense per-commodity row used to build the seeded
// inventory. The field set mirrors models.Commodity but adds a couple
// of "what files should this carry?" presentation fields the inventory
// pass turns into matching FileEntity rows.
type commoditySpec struct {
	Name      string
	ShortName string
	Type      models.CommodityType
	Area      string // area name, must match an areaSpec under the parent location
	Count     int
	// OriginalPrice + ConvertedOriginalPrice + CurrentPrice are
	// expressed in source currency / CZK / CZK respectively — same
	// convention the pre-existing seed used. Empty currency means
	// "skip the converted_original_price round-trip" (used for
	// drafts).
	OriginalPrice          float64
	OriginalPriceCurrency  string
	ConvertedOriginalPrice float64
	CurrentPrice           float64
	SerialNumber           string
	Status                 models.CommodityStatus
	// PurchaseDaysAgo controls the purchase_date / registered_date
	// fields — both are derived as `now - PurchaseDaysAgo` so we
	// don't have to hand-edit dates every time the bundled dataset
	// is refreshed.
	PurchaseDaysAgo int
	// WarrantyDaysFromNow drives the warranty bucket distribution.
	// Positive = future expiry (active or expiring); negative = past
	// expiry (expired); zero = no warranty. The orchestrator's
	// seedInventoryResult is later checked by the unit tests to make
	// sure each bucket has at least one row.
	WarrantyDaysFromNow int
	WarrantyNotes       string
	Tags                []string
	Comments            string
	Draft               bool
	// Photo selects which bundled JPG to attach as the cover-style
	// photo file. Falls back to fixturePhotoStorage when zero so
	// every commodity has at least one photo even if the table
	// editor forgot to pick one.
	Photo fixtureKind
	// PinCover sets the cover_file_id override on the commodity to
	// the photo created above — exercises FileCard's star-overlay
	// and CommodityDetailSheet's "current cover" affordance.
	PinCover bool
	// IncludeInvoice / IncludeManual seed an extra invoice PDF or
	// manual PDF alongside the photo.
	IncludeInvoice bool
	IncludeManual  bool
}

// locationCatalogue defines the inventory tree the seed installs.
// Kept verbose and inline — easier to review and edit than a CSV /
// YAML side file, and the size of this block is the single biggest
// signal of the seed's coverage.
var locationCatalogue = []locationSpec{
	{
		Name:        "Home",
		Address:     "123 Main St, Anytown, USA",
		Icon:        "🏠",
		Description: "Primary residence — most demo content lives here.",
		Areas: []areaSpec{
			{Name: "Living Room", Icon: "🛋️"},
			{Name: "Kitchen", Icon: "🍳"},
			{Name: "Bedroom", Icon: "🛏️"},
			{Name: "Home Office", Icon: "💻"},
			{Name: "Garage", Icon: "🔧"},
		},
	},
	{
		Name:        "Office",
		Address:     "456 Business Ave, Worktown, USA",
		Icon:        "🏢",
		Description: "Co-working hot desk + meeting room kit.",
		Areas: []areaSpec{
			{Name: "Work Desk", Icon: "🖥️"},
			{Name: "Conference Room", Icon: "📽️"},
			{Name: "Reception", Icon: "🪴"},
		},
	},
	{
		Name:        "Storage Unit",
		Address:     "789 Storage Blvd, Storeville, USA",
		Icon:        "📦",
		Description: "Off-site overflow — seasonal + spares.",
		Areas: []areaSpec{
			{Name: "Unit A", Icon: "📂"},
			{Name: "Unit B", Icon: "📁"},
		},
	},
}

// commodityCatalogue is the per-area inventory dataset. The Area name
// has to match one of the areaSpecs in locationCatalogue or the
// seedInventory pass will fail loudly — the seed treats a typo here as
// a hard error rather than silently dropping the commodity. ~35 rows
// in total, spanning the warranty / tag / status / file mix called out
// by the issue body.
var commodityCatalogue = []commoditySpec{
	// === Home / Living Room ==========================================
	{
		Name: "Smart TV", ShortName: "TV", Type: models.CommodityTypeElectronics,
		Area: "Living Room", Count: 1,
		OriginalPrice: 1299.99, OriginalPriceCurrency: "USD", ConvertedOriginalPrice: 29899.77, CurrentPrice: 899.99,
		SerialNumber: "TV123456789", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 540, WarrantyDaysFromNow: 5, // expiring inside reminder window — drives the email demo
		WarrantyNotes: "Manufacturer extended warranty — register by serial number on the OEM portal.",
		Tags:          []string{"electronics", "fragile", "warranty-watch"},
		Comments:      "65-inch 4K Smart TV",
		Photo:         fixturePhotoLivingRoom, PinCover: true, IncludeInvoice: true, IncludeManual: true,
	},
	{
		Name: "Sofa", ShortName: "Sofa", Type: models.CommodityTypeFurniture,
		Area: "Living Room", Count: 1,
		OriginalPrice: 899.99, OriginalPriceCurrency: "USD", ConvertedOriginalPrice: 20699.77, CurrentPrice: 699.99,
		SerialNumber: "SF987654321", Status: models.CommodityStatusSold,
		PurchaseDaysAgo: 880, WarrantyDaysFromNow: 0,
		Tags:     []string{"vintage"},
		Comments: "3-seat sectional sofa",
		Photo:    fixturePhotoLivingRoom, PinCover: false, IncludeInvoice: true,
	},
	{
		Name: "Coffee Table", ShortName: "Table", Type: models.CommodityTypeFurniture,
		Area: "Living Room", Count: 1,
		OriginalPrice: 249.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 6225.00, CurrentPrice: 4500.00,
		SerialNumber: "CT-LR-001", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 410, WarrantyDaysFromNow: 0,
		Tags:     []string{"vintage"},
		Comments: "Reclaimed-oak coffee table, oiled finish.",
		Photo:    fixturePhotoLivingRoom,
	},
	{
		Name: "Bookshelf Set", ShortName: "Books", Type: models.CommodityTypeFurniture,
		Area: "Living Room", Count: 2, // bundle — no warranty/loan possible
		OriginalPrice: 159.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 3975.00, CurrentPrice: 3000.00,
		Status:          models.CommodityStatusInUse,
		PurchaseDaysAgo: 200,
		Tags:            []string{},
		Comments:        "Matching pair, flatpack assembled.",
		Photo:           fixturePhotoLivingRoom,
	},
	{
		Name: "Vinyl Player", ShortName: "Vinyl", Type: models.CommodityTypeElectronics,
		Area: "Living Room", Count: 1,
		OriginalPrice: 549.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 13725.00, CurrentPrice: 12500.00,
		SerialNumber: "VP-AT-LP120", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 95, WarrantyDaysFromNow: 270, // active
		Tags:     []string{"electronics", "vintage", "fragile"},
		Comments: "Direct-drive turntable, pre-amp built in.",
		Photo:    fixturePhotoLivingRoom, IncludeManual: true,
	},

	// === Home / Kitchen =============================================
	{
		Name: "Refrigerator", ShortName: "Fridge", Type: models.CommodityTypeWhiteGoods,
		Area: "Kitchen", Count: 1,
		OriginalPrice: 1499.99, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 37499.75, CurrentPrice: 27599.77,
		SerialNumber: "RF123456789", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 700, WarrantyDaysFromNow: -90, // expired
		Tags:     []string{"kitchen"},
		Comments: "French door refrigerator with ice maker",
		Photo:    fixturePhotoKitchen, PinCover: true,
	},
	{
		Name: "Microwave Oven", ShortName: "Microwave", Type: models.CommodityTypeWhiteGoods,
		Area: "Kitchen", Count: 1,
		OriginalPrice: 199.99, OriginalPriceCurrency: "USD", ConvertedOriginalPrice: 4599.77, CurrentPrice: 3449.77,
		SerialNumber: "MW987654321", Status: models.CommodityStatusDisposed,
		PurchaseDaysAgo: 720, WarrantyDaysFromNow: -180, // expired
		Tags:     []string{"kitchen"},
		Comments: "1100W countertop microwave",
		Photo:    fixturePhotoKitchen,
	},
	{
		Name: "Dishwasher", ShortName: "Dishwasher", Type: models.CommodityTypeWhiteGoods,
		Area: "Kitchen", Count: 1,
		OriginalPrice: 699.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 17475.00, CurrentPrice: 15000.00,
		SerialNumber: "DW-BSCH-08", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 220, WarrantyDaysFromNow: 730, // active long
		WarrantyNotes: "5-year retailer extended warranty, claim ref CR-2025-DW.",
		Tags:          []string{"kitchen", "warranty-watch"},
		Comments:      "Integrated dishwasher, third basket model.",
		Photo:         fixturePhotoKitchen, IncludeInvoice: true, IncludeManual: true,
	},
	{
		Name: "Coffee Machine", ShortName: "Coffee", Type: models.CommodityTypeWhiteGoods,
		Area: "Kitchen", Count: 1,
		OriginalPrice: 4500.00, OriginalPriceCurrency: "CZK", CurrentPrice: 0,
		SerialNumber: "CM123456789", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 110,
		Tags:            []string{"kitchen"},
		Comments:        "Espresso machine with milk frother",
		Draft:           true, // currency-migration draft demo carried forward
		Photo:           fixturePhotoKitchen,
	},
	{
		Name: "Stand Mixer", ShortName: "Mixer", Type: models.CommodityTypeWhiteGoods,
		Area: "Kitchen", Count: 1,
		OriginalPrice: 399.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 9975.00, CurrentPrice: 8500.00,
		SerialNumber: "KA-CLASS-SM", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 30, WarrantyDaysFromNow: 730, // active
		WarrantyNotes: "Manufacturer 2-year warranty + 5-year motor coverage.",
		Tags:          []string{"kitchen", "gift"},
		Comments:      "Anniversary gift from in-laws.",
		Photo:         fixturePhotoKitchen, IncludeInvoice: true,
	},
	{
		Name: "Kettle", ShortName: "Kettle", Type: models.CommodityTypeWhiteGoods,
		Area: "Kitchen", Count: 1,
		OriginalPrice: 49.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 1225.00, CurrentPrice: 900.00,
		SerialNumber: "BR-KT-01", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 14, WarrantyDaysFromNow: 720, // active
		Tags:     []string{"kitchen"},
		Comments: "Replacement for the one that wore out.",
		Photo:    fixturePhotoKitchen,
	},

	// === Home / Bedroom =============================================
	{
		Name: "Bed Frame", ShortName: "Bed", Type: models.CommodityTypeFurniture,
		Area: "Bedroom", Count: 1,
		OriginalPrice: 599.99, OriginalPriceCurrency: "USD", ConvertedOriginalPrice: 13799.77, CurrentPrice: 11499.77,
		SerialNumber: "BF123456789", Status: models.CommodityStatusWrittenOff,
		PurchaseDaysAgo: 900, WarrantyDaysFromNow: 0,
		Tags:     []string{},
		Comments: "Queen size bed frame",
		Photo:    fixturePhotoBedroom,
	},
	{
		Name: "Mattress", ShortName: "Mattress", Type: models.CommodityTypeFurniture,
		Area: "Bedroom", Count: 1,
		OriginalPrice: 1199.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 29975.00, CurrentPrice: 25000.00,
		SerialNumber: "MT-EM-Q-09", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 180, WarrantyDaysFromNow: 3000, // long active
		WarrantyNotes: "10-year manufacturer warranty on indentation > 25mm.",
		Tags:          []string{"warranty-watch"},
		Comments:      "Pocket-spring + memory-foam hybrid.",
		Photo:         fixturePhotoBedroom, PinCover: true, IncludeInvoice: true,
	},
	{
		Name: "Wardrobe", ShortName: "Wardrobe", Type: models.CommodityTypeFurniture,
		Area: "Bedroom", Count: 1,
		OriginalPrice: 459.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 11475.00, CurrentPrice: 9500.00,
		Status:          models.CommodityStatusInUse,
		PurchaseDaysAgo: 360,
		Tags:            []string{},
		Comments:        "Free-standing 3-door.",
		Photo:           fixturePhotoBedroom,
	},
	{
		Name: "Reading Lamp", ShortName: "Lamp", Type: models.CommodityTypeOther,
		Area: "Bedroom", Count: 2, // bundle
		OriginalPrice: 89.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 2225.00, CurrentPrice: 1500.00,
		Status:          models.CommodityStatusInUse,
		PurchaseDaysAgo: 130,
		Tags:            []string{},
		Comments:        "Pair of bedside lamps.",
		Photo:           fixturePhotoBedroom,
	},

	// === Home / Home Office =========================================
	{
		Name: "Desktop PC", ShortName: "Desktop", Type: models.CommodityTypeElectronics,
		Area: "Home Office", Count: 1,
		OriginalPrice: 2199.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 54975.00, CurrentPrice: 45000.00,
		SerialNumber: "PC-HO-CUSTOM-01", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 60, WarrantyDaysFromNow: 670, // active
		WarrantyNotes: "Per-component warranties tracked in linked invoice.",
		Tags:          []string{"electronics", "work"},
		Comments:      "Custom build, AMD + Nvidia.",
		Photo:         fixturePhotoWork, PinCover: true, IncludeInvoice: true, IncludeManual: true,
	},
	{
		Name: "External SSD", ShortName: "SSD", Type: models.CommodityTypeElectronics,
		Area: "Home Office", Count: 1,
		OriginalPrice: 159.00, OriginalPriceCurrency: "USD", ConvertedOriginalPrice: 3659.00, CurrentPrice: 2200.00,
		SerialNumber: "EX-SSD-2TB-04", Status: models.CommodityStatusLost,
		PurchaseDaysAgo: 280, WarrantyDaysFromNow: 0,
		Tags:     []string{"electronics", "fragile"},
		Comments: "Lost during the office move — left in cab.",
		Photo:    fixturePhotoWork,
	},
	{
		Name: "Ergonomic Chair", ShortName: "Chair", Type: models.CommodityTypeFurniture,
		Area: "Home Office", Count: 1,
		OriginalPrice: 749.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 18725.00, CurrentPrice: 15000.00,
		SerialNumber: "ERG-CHAIR-MIRRA", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 365, WarrantyDaysFromNow: 30, // expiring inside 60d window
		WarrantyNotes: "12-year frame warranty, 2-year fabric warranty (about to expire).",
		Tags:          []string{"work", "warranty-watch"},
		Comments:      "Was the first thing the company bought when WFH started.",
		Photo:         fixturePhotoWork, IncludeInvoice: true,
	},

	// === Home / Garage ==============================================
	{
		Name: "Power Drill", ShortName: "Drill", Type: models.CommodityTypeEquipment,
		Area: "Garage", Count: 1,
		OriginalPrice: 189.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 4725.00, CurrentPrice: 3800.00,
		SerialNumber: "DR-DEWALT-001", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 90, WarrantyDaysFromNow: 1050, // active long
		Tags:     []string{"loaned-out"},
		Comments: "Cordless drill + impact driver bundle.",
		Photo:    fixturePhotoStorage, PinCover: true,
	},
	{
		Name: "Lawn Mower", ShortName: "Mower", Type: models.CommodityTypeEquipment,
		Area: "Garage", Count: 1,
		OriginalPrice: 459.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 11475.00, CurrentPrice: 9500.00,
		SerialNumber: "LM-HUSQ-2025", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 30, WarrantyDaysFromNow: 700, // active
		Tags:     []string{"outdoor", "seasonal"},
		Comments: "Battery-electric mower, two spare packs.",
		Photo:    fixturePhotoOutdoor, IncludeInvoice: true,
	},
	{
		Name: "Bicycle", ShortName: "Bike", Type: models.CommodityTypeEquipment,
		Area: "Garage", Count: 1,
		OriginalPrice: 1899.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 47475.00, CurrentPrice: 38000.00,
		SerialNumber: "BIKE-TREK-DM-19", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 730, WarrantyDaysFromNow: 90, // expiring
		WarrantyNotes: "Frame: lifetime; components: 24mo (countdown).",
		Tags:          []string{"outdoor", "fragile", "warranty-watch"},
		Comments:      "Commuter, currently in service for a brake-lever swap.",
		Photo:         fixturePhotoOutdoor, IncludeInvoice: true,
	},
	{
		Name: "Garden Hose", ShortName: "Hose", Type: models.CommodityTypeOther,
		Area: "Garage", Count: 1,
		OriginalPrice: 25.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 625.00, CurrentPrice: 350.00,
		Status:          models.CommodityStatusInUse,
		PurchaseDaysAgo: 200,
		Tags:            []string{"outdoor", "seasonal"},
		Comments:        "Retractable reel.",
		Photo:           fixturePhotoOutdoor,
	},

	// === Office / Work Desk =========================================
	{
		Name: "Laptop", ShortName: "Laptop", Type: models.CommodityTypeElectronics,
		Area: "Work Desk", Count: 1,
		OriginalPrice: 1299.99, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 32499.75, CurrentPrice: 22499.75,
		SerialNumber: "LT123456789", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 540,
		Tags:            []string{"electronics", "work"},
		Comments:        "15-inch business laptop",
		Draft:           true, // currency-migration draft demo carried forward
		Photo:           fixturePhotoWork,
	},
	{
		Name: "Monitor", ShortName: "Monitor", Type: models.CommodityTypeElectronics,
		Area: "Work Desk", Count: 2, // bundle
		OriginalPrice: 349.99, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 8749.75, CurrentPrice: 7499.75,
		SerialNumber:    "MN123456789",
		Status:          models.CommodityStatusInUse,
		PurchaseDaysAgo: 540,
		Tags:            []string{"electronics", "work"},
		Comments:        "27-inch 4K monitors",
		Draft:           true,
		Photo:           fixturePhotoWork,
	},
	{
		Name: "Desk Chair", ShortName: "Chair", Type: models.CommodityTypeFurniture,
		Area: "Work Desk", Count: 1,
		OriginalPrice: 249.99, OriginalPriceCurrency: "USD", ConvertedOriginalPrice: 5749.77, CurrentPrice: 0,
		SerialNumber: "DC123456789", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 540,
		Tags:            []string{"work"},
		Comments:        "Ergonomic office chair",
		Photo:           fixturePhotoWork,
	},
	{
		Name: "Camera Lens", ShortName: "Lens", Type: models.CommodityTypeElectronics,
		Area: "Work Desk", Count: 1,
		OriginalPrice: 599.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 14975.00, CurrentPrice: 12500.00,
		SerialNumber: "LN-SIGMA-2470", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 120, WarrantyDaysFromNow: 240,
		Tags:     []string{"electronics", "fragile", "loaned-out"},
		Comments: "Sigma 24-70 f2.8 — lent out to a colleague this week.",
		Photo:    fixturePhotoWork, IncludeInvoice: true,
	},

	// === Office / Conference Room ===================================
	{
		Name: "Projector", ShortName: "Projector", Type: models.CommodityTypeElectronics,
		Area: "Conference Room", Count: 1,
		OriginalPrice: 799.99, OriginalPriceCurrency: "USD", ConvertedOriginalPrice: 18399.77, CurrentPrice: 16099.77,
		SerialNumber: "PJ123456789", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 460, WarrantyDaysFromNow: -45, // expired
		Tags:     []string{"electronics", "fragile"},
		Comments: "4K projector for conference room",
		Photo:    fixturePhotoWork, PinCover: true,
	},
	{
		Name: "Conference Phone", ShortName: "Conf phone", Type: models.CommodityTypeElectronics,
		Area: "Conference Room", Count: 1,
		OriginalPrice: 199.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 4975.00, CurrentPrice: 3000.00,
		SerialNumber: "CP-POLYS-X30", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 800, WarrantyDaysFromNow: -300,
		Tags:     []string{"electronics", "work"},
		Comments: "Speakerphone, BT + USB-C.",
		Photo:    fixturePhotoWork,
	},

	// === Office / Reception =========================================
	{
		Name: "Reception Sign", ShortName: "Sign", Type: models.CommodityTypeOther,
		Area: "Reception", Count: 1,
		OriginalPrice: 350.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 8750.00, CurrentPrice: 0,
		Status:          models.CommodityStatusInUse,
		PurchaseDaysAgo: 200,
		Tags:            []string{},
		Comments:        "Backlit logo, brushed aluminium.",
		Photo:           fixturePhotoStorage,
	},
	{
		Name: "Plants (Set)", ShortName: "Plants", Type: models.CommodityTypeOther,
		Area: "Reception", Count: 4, // bundle, low-stakes
		OriginalPrice: 35.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 875.00, CurrentPrice: 600.00,
		Status:          models.CommodityStatusInUse,
		PurchaseDaysAgo: 50,
		Tags:            []string{},
		Comments:        "Snake plants + a fiddle leaf fig.",
		Photo:           fixturePhotoOutdoor,
	},

	// === Storage Unit / Unit A ======================================
	{
		Name: "Winter Clothes", ShortName: "Winter", Type: models.CommodityTypeClothes,
		Area: "Unit A", Count: 10, // bundle — no per-instance trackers
		OriginalPrice: 1200.00, OriginalPriceCurrency: "CZK", CurrentPrice: 600.00,
		Status:          models.CommodityStatusInUse,
		PurchaseDaysAgo: 950,
		Tags:            []string{"seasonal"},
		Comments:        "Winter clothes in storage",
		Photo:           fixturePhotoStorage,
	},
	{
		Name: "Camping Equipment", ShortName: "Camping", Type: models.CommodityTypeEquipment,
		Area: "Unit A", Count: 5, // bundle
		OriginalPrice: 850.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 21250.00, CurrentPrice: 17500.00,
		Status:          models.CommodityStatusInUse,
		PurchaseDaysAgo: 1080,
		Tags:            []string{"outdoor", "seasonal"},
		Comments:        "Tent, sleeping bags, and other camping gear",
		Photo:           fixturePhotoOutdoor,
	},
	{
		Name: "Skis", ShortName: "Skis", Type: models.CommodityTypeEquipment,
		Area: "Unit A", Count: 1,
		OriginalPrice: 720.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 18000.00, CurrentPrice: 12000.00,
		SerialNumber: "SK-VOLKL-179", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 350, WarrantyDaysFromNow: 120, // active
		Tags:     []string{"outdoor", "seasonal", "fragile"},
		Comments: "All-mountain, 179cm.",
		Photo:    fixturePhotoOutdoor,
	},

	// === Storage Unit / Unit B ======================================
	{
		Name: "Power Tools (Crate)", ShortName: "Tools", Type: models.CommodityTypeEquipment,
		Area: "Unit B", Count: 6, // bundle
		OriginalPrice: 600.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 15000.00, CurrentPrice: 12000.00,
		Status:          models.CommodityStatusInUse,
		PurchaseDaysAgo: 600,
		Tags:            []string{},
		Comments:        "Misc tools box.",
		Photo:           fixturePhotoStorage,
	},
	{
		Name: "Vacuum Cleaner", ShortName: "Vacuum", Type: models.CommodityTypeWhiteGoods,
		Area: "Unit B", Count: 1,
		OriginalPrice: 499.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 12475.00, CurrentPrice: 9500.00,
		SerialNumber: "VC-DYS-V11", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 540, WarrantyDaysFromNow: 14, // expiring fast
		WarrantyNotes: "Sent out for an authorized service - expect 2-week turnaround.",
		Tags:          []string{"warranty-watch"},
		Comments:      "Currently at the workshop for a battery swap.",
		Photo:         fixturePhotoStorage, IncludeInvoice: true,
	},
	{
		Name: "Game Console", ShortName: "Console", Type: models.CommodityTypeElectronics,
		Area: "Unit B", Count: 1,
		OriginalPrice: 549.00, OriginalPriceCurrency: "EUR", ConvertedOriginalPrice: 13725.00, CurrentPrice: 11000.00,
		SerialNumber: "GC-PS5-SLIM", Status: models.CommodityStatusInUse,
		PurchaseDaysAgo: 220, WarrantyDaysFromNow: 510, // active
		Tags:     []string{"electronics", "loaned-out"},
		Comments: "Lent to the upstairs neighbour for the weekend.",
		Photo:    fixturePhotoWork, IncludeManual: true,
	},
}

// seedInventory walks the location/area/commodity catalogues, creates
// each entity in registry order, and attaches the bundled files. The
// result struct lets the loans / services / events passes look up
// commodities by name without re-querying the registry.
func seedInventory(ctx context.Context, set *registry.Set, user *models.User, group *models.LocationGroup, uploader blobUploader) (*seedInventoryResult, error) {
	res := &seedInventoryResult{
		commodities:       make([]*models.Commodity, 0, len(commodityCatalogue)),
		commoditiesByName: make(map[string]*models.Commodity, len(commodityCatalogue)),
		locationsByName:   make(map[string]*models.Location, len(locationCatalogue)),
		areasByName:       make(map[string]*models.Area, 12),
	}

	// Cumulative tag-name set so a typo in a commodity row surfaces
	// loudly during seeding rather than silently auto-provisioning a
	// duplicate, off-color tag row.
	validTagSlugs := make(map[string]struct{}, len(seedTagCatalogue))
	for _, t := range seedTagCatalogue {
		validTagSlugs[t.Slug] = struct{}{}
	}

	if err := seedLocationsAndAreas(ctx, set, user, res); err != nil {
		return nil, err
	}

	for i := range commodityCatalogue {
		spec := commodityCatalogue[i]
		if err := seedOneCommodity(ctx, set, uploader, user, group, spec, validTagSlugs, res); err != nil {
			return nil, err
		}
	}

	if err := seedLocationFiles(ctx, set, uploader, user, group, res); err != nil {
		return nil, err
	}

	return res, nil
}

// seedLocationsAndAreas creates the location + area rows from the
// catalogue and indexes them by name on the result.
func seedLocationsAndAreas(ctx context.Context, set *registry.Set, user *models.User, res *seedInventoryResult) error {
	for _, locSpec := range locationCatalogue {
		loc, err := set.LocationRegistry.Create(ctx, models.Location{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				TenantID:        user.TenantID,
				CreatedByUserID: user.ID,
			},
			Name:        locSpec.Name,
			Address:     locSpec.Address,
			Icon:        locSpec.Icon,
			Description: locSpec.Description,
		})
		if err != nil {
			return fmt.Errorf("create location %s: %w", locSpec.Name, err)
		}
		res.locationsByName[locSpec.Name] = loc

		for _, areaSpec := range locSpec.Areas {
			area, err := set.AreaRegistry.Create(ctx, models.Area{
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
					TenantID:        user.TenantID,
					CreatedByUserID: user.ID,
				},
				Name:       areaSpec.Name,
				LocationID: loc.ID,
				Icon:       areaSpec.Icon,
			})
			if err != nil {
				return fmt.Errorf("create area %s: %w", areaSpec.Name, err)
			}
			res.areasByName[areaSpec.Name] = area
		}
	}
	return nil
}

// seedOneCommodity creates a single commodity row from spec, attaches
// its bundled files, and (optionally) pins the cover.
func seedOneCommodity(ctx context.Context, set *registry.Set, uploader blobUploader, user *models.User, group *models.LocationGroup, spec commoditySpec, validTagSlugs map[string]struct{}, res *seedInventoryResult) error {
	area, ok := res.areasByName[spec.Area]
	if !ok {
		return fmt.Errorf("commodity %q references unknown area %q — check locationCatalogue", spec.Name, spec.Area)
	}
	for _, slug := range spec.Tags {
		if _, ok := validTagSlugs[slug]; !ok {
			return fmt.Errorf("commodity %q references unknown tag slug %q — add it to seedTagCatalogue", spec.Name, slug)
		}
	}

	created, err := set.CommodityRegistry.Create(ctx, buildCommodity(spec, area.ID, user, group))
	if err != nil {
		return fmt.Errorf("create commodity %q: %w", spec.Name, err)
	}
	res.commodities = append(res.commodities, created)
	res.commoditiesByName[spec.Name] = created

	return attachCommodityFiles(ctx, set, uploader, user, group, spec, created)
}

// buildCommodity materialises a models.Commodity from the spec —
// extracted so seedOneCommodity stays terse and the loop above stays
// under gocyclo's complexity ceiling.
func buildCommodity(spec commoditySpec, areaID string, user *models.User, group *models.LocationGroup) models.Commodity {
	purchase := daysFromToday(-spec.PurchaseDaysAgo)
	registered := daysFromToday(-spec.PurchaseDaysAgo + 1)
	var warranty *models.Date
	if spec.WarrantyDaysFromNow != 0 && spec.Count == 1 {
		warranty = daysFromToday(spec.WarrantyDaysFromNow)
	}

	commodity := models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        user.TenantID,
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		Name:                  spec.Name,
		ShortName:             spec.ShortName,
		Type:                  spec.Type,
		AreaID:                areaID,
		Count:                 spec.Count,
		OriginalPrice:         decimal.NewFromFloat(spec.OriginalPrice),
		OriginalPriceCurrency: models.Currency(spec.OriginalPriceCurrency),
		CurrentPrice:          decimal.NewFromFloat(spec.CurrentPrice),
		SerialNumber:          spec.SerialNumber,
		Status:                spec.Status,
		PurchaseDate:          purchase,
		RegisteredDate:        registered,
		Tags:                  models.ValuerSlice[string](spec.Tags),
		Comments:              spec.Comments,
		Draft:                 spec.Draft,
		WarrantyExpiresAt:     warranty,
		WarrantyNotes:         spec.WarrantyNotes,
	}
	if spec.ConvertedOriginalPrice != 0 {
		commodity.ConvertedOriginalPrice = decimal.NewFromFloat(spec.ConvertedOriginalPrice)
	}
	return commodity
}

// attachCommodityFiles uploads the bundled photo and optional
// invoice/manual files for a single commodity, then optionally pins
// the photo as the cover.
func attachCommodityFiles(ctx context.Context, set *registry.Set, uploader blobUploader, user *models.User, group *models.LocationGroup, spec commoditySpec, created *models.Commodity) error {
	photo := spec.Photo
	if photo == "" {
		photo = fixturePhotoStorage
	}
	coverID, err := attachCommodityFile(ctx, set, uploader, user, group, created, photo, "images", deriveTitle(spec.Name, "photo"), spec.Tags)
	if err != nil {
		return fmt.Errorf("attach photo for %q: %w", spec.Name, err)
	}
	if spec.PinCover && coverID != "" {
		c := *created
		c.CoverFileID = &coverID
		if _, err := set.CommodityRegistry.Update(ctx, c); err != nil {
			return fmt.Errorf("pin cover for %q: %w", spec.Name, err)
		}
	}
	if spec.IncludeInvoice {
		if _, err := attachCommodityFile(ctx, set, uploader, user, group, created, fixtureInvoice, "invoices", deriveTitle(spec.Name, "invoice"), nil); err != nil {
			return fmt.Errorf("attach invoice for %q: %w", spec.Name, err)
		}
	}
	if spec.IncludeManual {
		if _, err := attachCommodityFile(ctx, set, uploader, user, group, created, fixtureManual, "manuals", deriveTitle(spec.Name, "manual"), nil); err != nil {
			return fmt.Errorf("attach manual for %q: %w", spec.Name, err)
		}
	}
	return nil
}

// seedLocationFiles drops a small number of location-level files in
// known locations so the per-location Files panel isn't a stub.
func seedLocationFiles(ctx context.Context, set *registry.Set, uploader blobUploader, user *models.User, group *models.LocationGroup, res *seedInventoryResult) error {
	if home, ok := res.locationsByName["Home"]; ok {
		if _, err := attachLocationFile(ctx, set, uploader, user, group, home, fixturePhotoLivingRoom, "images", "Front door photo", nil); err != nil {
			return err
		}
		if _, err := attachLocationFile(ctx, set, uploader, user, group, home, fixtureInvoice, "files", "Lease agreement", []string{"warranty-watch"}); err != nil {
			return err
		}
	}
	if office, ok := res.locationsByName["Office"]; ok {
		if _, err := attachLocationFile(ctx, set, uploader, user, group, office, fixtureInvoice, "files", "Office insurance certificate", nil); err != nil {
			return err
		}
	}
	return nil
}

// deriveTitle produces a stable, human-readable file title from a
// commodity name + a kind keyword. Used so the Files page rows read as
// "Smart TV photo" / "Smart TV invoice" rather than the raw fixture
// filename.
func deriveTitle(commodityName, kind string) string {
	base := strings.TrimSpace(commodityName)
	if base == "" {
		base = "Item"
	}
	return base + " " + kind
}

// attachArgs bundles the arguments shared by attachCommodityFile and
// attachLocationFile so the file-create signatures stay under the lll
// threshold and the call sites read sensibly.
type attachArgs struct {
	Set      *registry.Set
	Uploader blobUploader
	User     *models.User
	Group    *models.LocationGroup
	Fixture  fixtureKind
	Meta     string
	Title    string
	Tags     []string
}

// attachCommodityFile uploads the chosen bundled fixture, creates the
// matching FileEntity row, links it to the commodity, and returns the
// new row's ID.
func attachCommodityFile(ctx context.Context, set *registry.Set, uploader blobUploader, user *models.User, group *models.LocationGroup, commodity *models.Commodity, fixture fixtureKind, meta, title string, tags []string) (string, error) {
	return attachFile(ctx, attachArgs{Set: set, Uploader: uploader, User: user, Group: group, Fixture: fixture, Meta: meta, Title: title, Tags: tags}, "commodity", commodity.ID)
}

// attachLocationFile is the same as attachCommodityFile but for
// location-scoped files (LinkedEntityType="location").
func attachLocationFile(ctx context.Context, set *registry.Set, uploader blobUploader, user *models.User, group *models.LocationGroup, loc *models.Location, fixture fixtureKind, meta, title string, tags []string) (string, error) {
	return attachFile(ctx, attachArgs{Set: set, Uploader: uploader, User: user, Group: group, Fixture: fixture, Meta: meta, Title: title, Tags: tags}, "location", loc.ID)
}

func attachFile(ctx context.Context, args attachArgs, linkedEntityType, linkedEntityID string) (string, error) {
	storagePath, size, err := args.Uploader.upload(ctx, args.Fixture)
	if err != nil {
		return "", err
	}
	mime := fixtureMIME(args.Fixture)
	ext := fixtureExt(args.Fixture)
	// Backdate seed files by ~1 day so any file uploaded by a live
	// test or user lands chronologically AFTER the seed batch — the
	// Files page sorts by created_at, and a freshly-uploaded file
	// being newer than the entire seed corpus is what keeps the
	// per-test "user1 sees their own upload" assertion stable
	// regardless of how many fixture files the seed grows in the
	// future.
	now := time.Now().AddDate(0, 0, -1)
	pathTitle := strings.TrimSuffix(strings.TrimPrefix(string(args.Fixture), "_files/"), ext)
	fileEntity := models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        args.User.TenantID,
			GroupID:         args.Group.ID,
			CreatedByUserID: args.User.ID,
		},
		Title:            args.Title,
		Description:      "",
		Type:             models.FileTypeFromMIME(mime),
		Category:         models.FileCategoryFromContext(linkedEntityType, args.Meta, mime),
		Tags:             mergeSeedAutoTags(args.Tags, linkedEntityType, args.Meta),
		LinkedEntityType: linkedEntityType,
		LinkedEntityID:   linkedEntityID,
		LinkedEntityMeta: args.Meta,
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         pathTitle,
			OriginalPath: storagePath,
			Ext:          ext,
			MIMEType:     mime,
			SizeBytes:    size,
		},
	}
	created, err := args.Set.FileRegistry.Create(ctx, fileEntity)
	if err != nil {
		return "", fmt.Errorf("create file row for %s: %w", args.Title, err)
	}
	return created.ID, nil
}

// mergeSeedAutoTags clones the explicit per-fixture tag list and appends
// any conventional auto-tags implied by the linked-entity bucket — same
// rule the apiserver applies on create/update (post-#1622), so seeded
// rows stay consistent with what an interactive upload would produce.
// Returns a fresh slice; never mutates the input.
func mergeSeedAutoTags(explicit []string, linkedEntityType, linkedEntityMeta string) []string {
	out := append([]string(nil), explicit...)
	for _, t := range models.AutoTagsForContext(linkedEntityType, linkedEntityMeta) {
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}
