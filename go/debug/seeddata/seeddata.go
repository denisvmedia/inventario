package seeddata

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-extras/go-kit/ptr"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// SeedOptions contains optional parameters for seeding
type SeedOptions struct {
	UserEmail  string // Optional: email of user to seed for
	TenantSlug string // Optional: slug of tenant to seed for
}

// findOrCreateTenant finds an existing tenant by slug or creates a new test tenant
func findOrCreateTenant(ctx context.Context, registrySet *registry.Set, tenantSlug string) (*models.Tenant, error) {
	if tenantSlug != "" {
		// User specified a tenant slug, try to find it
		tenant, err := registrySet.TenantRegistry.GetBySlug(ctx, tenantSlug)
		if err != nil {
			return nil, fmt.Errorf("tenant with slug '%s' not found: %w", tenantSlug, err)
		}
		return tenant, nil
	}

	// No tenant specified, try to find an existing tenant
	existingTenants, err := registrySet.TenantRegistry.List(ctx)
	if err == nil && len(existingTenants) > 0 {
		// Use the first existing tenant
		return existingTenants[0], nil
	}

	// No tenants exist, create test tenant
	testTenant := models.Tenant{
		Name:   "Test Organization",
		Slug:   "test-org",
		Status: models.TenantStatusActive,
	}
	return registrySet.TenantRegistry.Create(ctx, testTenant)
}

// findOrCreateUsers finds existing users or creates test users based on options
func findOrCreateUsers(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant, users []*models.User, userEmail string) (user1 *models.User, user2 *models.User, err error) {
	if userEmail != "" {
		user1, user2 = findUserByEmail(users, tenant.ID, userEmail)
		if user1 == nil && user2 == nil {
			return nil, nil, fmt.Errorf("user with email '%s' not found in tenant '%s'", userEmail, tenant.Slug)
		}
		return user1, user2, nil
	}

	// No user specified, find existing admin and regular users for this tenant
	user1, user2 = findExistingUsers(users, tenant.ID)

	// Only create test users if no specific user was requested AND no suitable users were found
	if user1 == nil {
		return createTestUsers(ctx, registrySet, tenant, user2)
	}

	return user1, user2, nil
}

// findUserByEmail finds a specific user by email and tenant ID
func findUserByEmail(users []*models.User, tenantID, email string) (admin *models.User, regular *models.User) {
	for _, user := range users {
		if user.TenantID == tenantID && user.Email == email {
			if user.Role == models.UserRoleAdmin {
				return user, nil
			}
			return nil, user
		}
	}
	return nil, nil
}

// findExistingUsers finds the first admin and regular user for a tenant
func findExistingUsers(users []*models.User, tenantID string) (admin *models.User, regular *models.User) {
	for _, user := range users {
		if user.TenantID != tenantID {
			continue
		}

		if user.Role == models.UserRoleAdmin && admin == nil {
			admin = user
		} else if user.Role == models.UserRoleUser && regular == nil {
			regular = user
		}

		// Stop if we found both
		if admin != nil && regular != nil {
			break
		}
	}
	return admin, regular
}

// createTestUsers creates test admin and regular users
func createTestUsers(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant, existingUser2 *models.User) (admin *models.User, regular *models.User, err error) {
	slog.Info("Creating test users", "tenant", tenant.Slug)

	// Create test admin
	testUser1 := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		Email:    "admin@test-org.com",
		Name:     "Test Administrator",
		Role:     models.UserRoleAdmin,
		IsActive: true,
	}
	err = testUser1.SetPassword("testpassword123")
	if err != nil {
		return nil, nil, err
	}
	admin, err = registrySet.UserRegistry.Create(ctx, testUser1)
	if err != nil {
		return nil, nil, err
	}

	// If no regular user exists, create test user 2
	regular = existingUser2
	if regular == nil {
		testUser2 := models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: tenant.ID,
			},
			Email:    "user2@test-org.com",
			Name:     "Test User 2",
			Role:     models.UserRoleUser,
			IsActive: true,
		}
		err = testUser2.SetPassword("testpassword123")
		if err != nil {
			return nil, nil, err
		}
		regular, err = registrySet.UserRegistry.Create(ctx, testUser2)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create test user 2: %v", err)
		}
	}

	return admin, regular, nil
}

// createCommodityWithTenant is a helper function to create commodities with proper user context
func createCommodityWithTenant(ctx context.Context, registrySet *registry.Set, commodity models.Commodity, user *models.User) (*models.Commodity, error) {
	// Set the tenant and user IDs from the actual user
	commodity.TenantID = user.TenantID
	commodity.UserID = user.ID

	return registrySet.CommodityRegistry.Create(ctx, commodity)
}

// SeedData seeds the database with example data.
func SeedData(factorySet *registry.FactorySet, opts SeedOptions) error { //nolint:funlen,gocyclo,gocognit // it's a seed function
	slog.Info("Seeding database",
		"user_email", opts.UserEmail,
		"tenant_slug", opts.TenantSlug,
	)
	ctx := context.Background()
	registrySet := factorySet.CreateServiceRegistrySet()

	// Find or create tenant
	tenant, err := findOrCreateTenant(ctx, registrySet, opts.TenantSlug)
	if err != nil {
		return err
	}

	// Get existing users for the tenant
	users, err := registrySet.UserRegistry.List(ctx)
	if err != nil {
		return err
	}

	user1, user2, err := findOrCreateUsers(ctx, registrySet, tenant, users, opts.UserEmail)
	if err != nil {
		return err
	}

	// Create default system configuration with CZK as main currency for the first test user
	systemConfig := models.SettingsObject{
		MainCurrency: new("CZK"),
	}

	// Set user context for settings (settings are per-user)
	userCtx := appctx.WithUser(ctx, user1)

	// Create user-aware registry set for settings operations
	userRegistrySet, err := factorySet.CreateUserRegistrySet(userCtx)
	if err != nil {
		return fmt.Errorf("failed to create user registry set for user 1: %w", err)
	}

	err = userRegistrySet.SettingsRegistry.Save(userCtx, systemConfig)
	if err != nil {
		return fmt.Errorf("failed to save settings for user: %w", err)
	}

	// Also create default settings for the second user if they exist
	if user2 != nil {
		userCtx2 := appctx.WithUser(ctx, user2)

		// Create user-aware registry set for the second user
		userRegistrySet2, err := factorySet.CreateUserRegistrySet(userCtx2)
		if err != nil {
			return fmt.Errorf("failed to create user registry set for user 2: %w", err)
		}

		// User 2 gets EUR as main currency (different from user 1)
		systemConfig2 := models.SettingsObject{
			MainCurrency: new("EUR"),
		}

		err = userRegistrySet2.SettingsRegistry.Save(userCtx2, systemConfig2)
		if err != nil {
			return fmt.Errorf("failed to save settings for user 2: %w", err)
		}
	}

	// Create locations using user-aware registry
	home, err := userRegistrySet.LocationRegistry.Create(userCtx, models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user1.TenantID,
			UserID:   user1.ID,
		},
		Name:    "Home",
		Address: "123 Main St, Anytown, USA",
	})
	if err != nil {
		return err
	}

	office, err := userRegistrySet.LocationRegistry.Create(userCtx, models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user1.TenantID,
			UserID:   user1.ID,
		},
		Name:    "Office",
		Address: "456 Business Ave, Worktown, USA",
	})
	if err != nil {
		return err
	}

	storage, err := userRegistrySet.LocationRegistry.Create(userCtx, models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user1.TenantID,
			UserID:   user1.ID,
		},
		Name:    "Storage Unit",
		Address: "789 Storage Blvd, Storeville, USA",
	})
	if err != nil {
		return err
	}

	// Create areas for Home
	livingRoom, err := userRegistrySet.AreaRegistry.Create(userCtx, models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user1.TenantID,
			UserID:   user1.ID,
		},
		Name:       "Living Room",
		LocationID: home.ID,
	})
	if err != nil {
		return err
	}

	kitchen, err := userRegistrySet.AreaRegistry.Create(userCtx, models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user1.TenantID,
			UserID:   user1.ID,
		},
		Name:       "Kitchen",
		LocationID: home.ID,
	})
	if err != nil {
		return err
	}

	bedroom, err := userRegistrySet.AreaRegistry.Create(userCtx, models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user1.TenantID,
			UserID:   user1.ID,
		},
		Name:       "Bedroom",
		LocationID: home.ID,
	})
	if err != nil {
		return err
	}

	// Create areas for Office
	workDesk, err := userRegistrySet.AreaRegistry.Create(userCtx, models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user1.TenantID,
			UserID:   user1.ID,
		},
		Name:       "Work Desk",
		LocationID: office.ID,
	})
	if err != nil {
		return err
	}

	conferenceRoom, err := userRegistrySet.AreaRegistry.Create(userCtx, models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user1.TenantID,
			UserID:   user1.ID,
		},
		Name:       "Conference Room",
		LocationID: office.ID,
	})
	if err != nil {
		return err
	}

	// Create areas for Storage
	unitA, err := userRegistrySet.AreaRegistry.Create(userCtx, models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user1.TenantID,
			UserID:   user1.ID,
		},
		Name:       "Unit A",
		LocationID: storage.ID,
	})
	if err != nil {
		return err
	}

	// Create commodities for Living Room
	_, err = userRegistrySet.CommodityRegistry.Create(userCtx, models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user1.TenantID,
			UserID:   user1.ID,
		},
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

	_, err = userRegistrySet.CommodityRegistry.Create(userCtx, models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user1.TenantID,
			UserID:   user1.ID,
		},
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
	_, err = createCommodityWithTenant(userCtx, userRegistrySet, models.Commodity{
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
	}, user1)
	if err != nil {
		return err
	}

	_, err = createCommodityWithTenant(userCtx, userRegistrySet, models.Commodity{
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
	}, user1)
	if err != nil {
		return err
	}

	// Create commodities for Bedroom
	_, err = createCommodityWithTenant(userCtx, userRegistrySet, models.Commodity{
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
	}, user1)
	if err != nil {
		return err
	}

	// Create commodities for Work Desk
	_, err = createCommodityWithTenant(userCtx, userRegistrySet, models.Commodity{
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
	}, user1)
	if err != nil {
		return err
	}

	_, err = createCommodityWithTenant(userCtx, userRegistrySet, models.Commodity{
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
	}, user1)
	if err != nil {
		return err
	}

	// Create commodities for Conference Room
	_, err = createCommodityWithTenant(userCtx, userRegistrySet, models.Commodity{
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
	}, user1)
	if err != nil {
		return err
	}

	// Create commodities for Storage Unit
	_, err = createCommodityWithTenant(userCtx, userRegistrySet, models.Commodity{
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
	}, user1)
	if err != nil {
		return err
	}

	_, err = createCommodityWithTenant(userCtx, userRegistrySet, models.Commodity{
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
	}, user1)
	if err != nil {
		return err
	}

	// Create a new draft commodity with CZK as original currency
	_, err = createCommodityWithTenant(userCtx, userRegistrySet, models.Commodity{
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
		Draft:                 true, // Value status
	}, user1)
	if err != nil {
		return err
	}

	// Create a commodity with original price in USD but no current price, only converted price
	_, err = createCommodityWithTenant(userCtx, userRegistrySet, models.Commodity{
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
	}, user1)
	if err != nil {
		return err
	}

	return nil
}
