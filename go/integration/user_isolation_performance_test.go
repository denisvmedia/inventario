package integration_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// BenchmarkUserIsolation_ConcurrentUsers benchmarks user isolation with concurrent users
func BenchmarkUserIsolation_ConcurrentUsers(b *testing.B) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		b.Skip("POSTGRES_TEST_DSN environment variable not set")
		return
	}

	c := qt.New(b)

	registrySetFunc, cleanup := postgres.NewPostgresRegistrySet()
	defer cleanup()
	factorySet, err := registrySetFunc(registry.Config(dsn))
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create registry set: %v", err))

	// Create test users
	users := make([]*models.User, 10)
	for i := 0; i < 10; i++ {
		user := models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: fmt.Sprintf("bench-user-%d", i)},
				TenantID: "test-tenant-id",
			},
			Email:    fmt.Sprintf("bench-user-%d@example.com", i),
			Name:     fmt.Sprintf("Benchmark User %d", i),
			Role:     models.UserRoleUser,
			IsActive: true,
		}

		err = user.SetPassword("testpassword123")
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to set password: %v", err))

		created, err := factorySet.UserRegistry.Create(context.Background(), user)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user: %v", err))
		users[i] = created
	}

	b.ResetTimer()

	b.Run("ConcurrentCommodityOperations", func(b *testing.B) {
		c := qt.New(b)
		b.RunParallel(func(pb *testing.PB) {
			userIndex := 0
			for pb.Next() {
				user := users[userIndex%len(users)]
				ctx := appctx.WithUser(context.Background(), user)

				// Create commodity
				commodity := models.Commodity{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: fmt.Sprintf("bench-commodity-%d-%d", userIndex, time.Now().UnixNano())},
						TenantID: "test-tenant-id",
						UserID:   user.ID,
					},
					Name:                   fmt.Sprintf("Benchmark Commodity %d", userIndex),
					ShortName:              fmt.Sprintf("BC%d", userIndex),
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
				}

				userAwareCommodityRegistry, err := factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
				c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user-aware registry: %v", err))

				created, err := userAwareCommodityRegistry.Create(ctx, commodity)
				c.Assert(err, qt.IsNil, qt.Commentf("Failed to create commodity: %v", err))

				// List commodities (should only see own)
				userAwareCommodityRegistry, err = factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
				c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user-aware registry: %v", err))

				commodities, err := userAwareCommodityRegistry.List(ctx)
				c.Assert(err, qt.IsNil, qt.Commentf("Failed to list commodities: %v", err))

				// Verify isolation - should only see own commodities
				for _, commodity := range commodities {
					c.Assert(commodity.GetUserID(), qt.Equals, user.ID, qt.Commentf("Expected user ID %s, got %s", user.ID, commodity.GetUserID()))
				}

				// Clean up
				err = userAwareCommodityRegistry.Delete(ctx, created.ID)
				c.Assert(err, qt.IsNil, qt.Commentf("Failed to delete commodity: %v", err))

				userIndex++
			}
		})
	})
}

// TestUserIsolation_LoadTesting tests user isolation under load
func TestUserIsolation_LoadTesting(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Create multiple users
	numUsers := 50
	users := make([]*models.User, numUsers)
	for i := 0; i < numUsers; i++ {
		user := models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: fmt.Sprintf("load-user-%d", i)},
				TenantID: "test-tenant-id",
			},
			Email:    fmt.Sprintf("load-user-%d@example.com", i),
			Name:     fmt.Sprintf("Load Test User %d", i),
			Role:     models.UserRoleUser,
			IsActive: true,
		}
		err := user.SetPassword("testpassword123")
		c.Assert(err, qt.IsNil)

		created, err := registrySet.UserRegistry.Create(context.Background(), user)
		c.Assert(err, qt.IsNil)
		users[i] = created
	}

	// Each user creates commodities concurrently
	var wg sync.WaitGroup
	errors := make(chan error, numUsers*10)

	for i, user := range users {
		wg.Add(1)
		go func(userIndex int, u *models.User) {
			defer wg.Done()

			ctx := appctx.WithUser(context.Background(), u)

			// Create location and area for this user's commodities
			location := models.Location{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: fmt.Sprintf("load-location-%d", userIndex)},
					TenantID: "test-tenant-id",
					UserID:   u.ID,
				},
				Name:    fmt.Sprintf("Load Test Location %d", userIndex),
				Address: fmt.Sprintf("123 Load Street %d", userIndex),
			}
			userAwareLocationRegistry, err := registrySet.LocationRegistryFactory.CreateUserRegistry(ctx)
			if err != nil {
				errors <- fmt.Errorf("user %d failed to create location registry: %v", userIndex, err)
				return
			}
			createdLocation, err := userAwareLocationRegistry.Create(ctx, location)
			if err != nil {
				errors <- fmt.Errorf("user %d failed to create location: %v", userIndex, err)
				return
			}

			area := models.Area{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: fmt.Sprintf("load-area-%d", userIndex)},
					TenantID: "test-tenant-id",
					UserID:   u.ID,
				},
				Name:       fmt.Sprintf("Load Test Area %d", userIndex),
				LocationID: createdLocation.ID,
			}
			userAwareAreaRegistry, err := registrySet.AreaRegistryFactory.CreateUserRegistry(ctx)
			if err != nil {
				errors <- fmt.Errorf("user %d failed to create area registry: %v", userIndex, err)
				return
			}
			createdArea, err := userAwareAreaRegistry.Create(ctx, area)
			if err != nil {
				errors <- fmt.Errorf("user %d failed to create area: %v", userIndex, err)
				return
			}

			// Create user-aware registry once for this user
			userAwareCommodityRegistry, err := registrySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
			if err != nil {
				errors <- fmt.Errorf("user %d failed to create user-aware registry: %v", userIndex, err)
				return
			}

			// Create multiple commodities per user
			for j := 0; j < 10; j++ {
				commodity := models.Commodity{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: fmt.Sprintf("load-commodity-%d-%d", userIndex, j)},
						TenantID: "test-tenant-id",
						UserID:   u.ID,
					},
					Name:                   fmt.Sprintf("Load Test Commodity %d-%d", userIndex, j),
					ShortName:              fmt.Sprintf("LTC%d%d", userIndex, j),
					AreaID:                 createdArea.ID,
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
				}

				_, err = userAwareCommodityRegistry.Create(ctx, commodity)
				if err != nil {
					errors <- fmt.Errorf("user %d failed to create commodity %d: %v", userIndex, j, err)
					return
				}
			}

			// List commodities and verify isolation
			commodities, err := userAwareCommodityRegistry.List(ctx)
			if err != nil {
				errors <- fmt.Errorf("user %d failed to list commodities: %v", userIndex, err)
				return
			}

			// Should see exactly 10 commodities (own commodities only)
			if len(commodities) != 10 {
				errors <- fmt.Errorf("user %d sees %d commodities, expected 10", userIndex, len(commodities))
				return
			}

			// Verify all commodities belong to this user
			for _, commodity := range commodities {
				if commodity.GetUserID() != u.ID {
					errors <- fmt.Errorf("user %d can see commodity belonging to user %s", userIndex, commodity.GetUserID())
					return
				}
			}
		}(i, user)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		c.Errorf("Load test error: %v", err)
	}
}

// TestUserIsolation_SecurityBoundaries tests security edge cases
func TestUserIsolation_SecurityBoundaries(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Create a legitimate user
	user := createTestUser(c, registrySet.UserRegistry, "legitimate@example.com")
	ctx := appctx.WithUser(context.Background(), user)

	// Create location and area for the commodity
	location := models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "security-test-location"},
			TenantID: "test-tenant-id",
			UserID:   user.ID,
		},
		Name:    "Security Test Location",
		Address: "123 Security Street",
	}
	userAwareLocationRegistry, err := registrySet.LocationRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	createdLocation, err := userAwareLocationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)

	area := models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "security-test-area"},
			TenantID: "test-tenant-id",
			UserID:   user.ID,
		},
		Name:       "Security Test Area",
		LocationID: createdLocation.ID,
	}
	userAwareAreaRegistry, err := registrySet.AreaRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	createdArea, err := userAwareAreaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	// Create a commodity
	commodity := models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "security-test-commodity"},
			TenantID: "test-tenant-id",
			UserID:   user.ID,
		},
		Name:                   "Security Test Commodity",
		ShortName:              "STC",
		AreaID:                 createdArea.ID,
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
	}
	userAwareCommodityRegistry, err := registrySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	created, err := userAwareCommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Test various malicious user ID attempts
	maliciousUserIDs := []string{
		"'; DROP TABLE commodities; --",
		"<script>alert('xss')</script>",
		"../../../etc/passwd",
		"null",
		"undefined",
		"0",
		"-1",
		"999999999999999999999999999999",
		"user-id' OR '1'='1",
		"user-id\x00null-byte",
		"user-id\n\r\t",
		string(make([]byte, 10000)), // Very long string
	}

	for _, maliciousID := range maliciousUserIDs {
		// Create a sanitized test name to avoid 0x00 pollution in output
		testName := fmt.Sprintf("Malicious ID: %q", maliciousID)
		if len(maliciousID) > 100 {
			testName = fmt.Sprintf("Malicious ID: very_long_string_%d_bytes", len(maliciousID))
		}
		t.Run(testName, func(t *testing.T) {
			c := qt.New(t)
			maliciousCtx := appctx.WithUser(context.Background(), &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: maliciousID},
					TenantID: "test-tenant-id",
				},
			})

			// Try to access the legitimate user's commodity
			maliciousUserAwareRegistry, err := registrySet.CommodityRegistryFactory.CreateUserRegistry(maliciousCtx)
			if err != nil {
				// If CreateUserRegistry fails, that's expected for malicious contexts
				return
			}
			_, err = maliciousUserAwareRegistry.Get(maliciousCtx, created.ID)
			c.Assert(err, qt.IsNotNil) // Should fail

			// Try to list commodities
			commodities, err := maliciousUserAwareRegistry.List(maliciousCtx)
			if err == nil {
				// If no error, should return empty list
				c.Assert(len(commodities), qt.Equals, 0)
			}

			// Try to update the commodity
			created.Name = "Hacked Name"
			_, err = maliciousUserAwareRegistry.Update(maliciousCtx, *created)
			c.Assert(err, qt.IsNotNil) // Should fail

			// Try to delete the commodity
			err = maliciousUserAwareRegistry.Delete(maliciousCtx, created.ID)
			c.Assert(err, qt.IsNotNil) // Should fail
		})
	}

	// Verify the original commodity is still intact
	retrieved, err := userAwareCommodityRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved.Name, qt.Equals, "Security Test Commodity")
}

// TestUserIsolation_PerformanceRegression tests for performance regressions
func TestUserIsolation_PerformanceRegression(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Create a user
	user := createTestUser(c, registrySet.UserRegistry, "perf@example.com")
	ctx := appctx.WithUser(context.Background(), user)

	// Create location and area for commodities
	location := models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "perf-location"},
			TenantID: "test-tenant-id",
			UserID:   user.ID,
		},
		Name:    "Performance Test Location",
		Address: "123 Performance Street",
	}
	userAwareLocationRegistry, err := registrySet.LocationRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	createdLocation, err := userAwareLocationRegistry.Create(ctx, location)
	c.Assert(err, qt.IsNil)

	area := models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "perf-area"},
			TenantID: "test-tenant-id",
			UserID:   user.ID,
		},
		Name:       "Performance Test Area",
		LocationID: createdLocation.ID,
	}
	userAwareAreaRegistry, err := registrySet.AreaRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	createdArea, err := userAwareAreaRegistry.Create(ctx, area)
	c.Assert(err, qt.IsNil)

	// Create many commodities
	numCommodities := 1000
	userAwareCommodityRegistry, err := registrySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)

	for i := 0; i < numCommodities; i++ {
		commodity := models.Commodity{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: fmt.Sprintf("perf-commodity-%d", i)},
				TenantID: "test-tenant-id",
				UserID:   user.ID,
			},
			Name:                   fmt.Sprintf("Performance Test Commodity %d", i),
			ShortName:              fmt.Sprintf("PTC%d", i),
			AreaID:                 createdArea.ID,
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
		}
		_, err = userAwareCommodityRegistry.Create(ctx, commodity)
		c.Assert(err, qt.IsNil)
	}

	// Measure list performance
	start := time.Now()
	commodities, err := userAwareCommodityRegistry.List(ctx)
	duration := time.Since(start)

	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities), qt.Equals, numCommodities)

	// Performance should be reasonable (less than 1 second for 1000 items)
	if duration > time.Second {
		t.Errorf("List operation took too long: %v", duration)
	}

	t.Logf("Listed %d commodities in %v", numCommodities, duration)
}
