package integration_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// BenchmarkUserIsolation_ConcurrentUsers benchmarks user isolation with
// concurrent users, each in their own group.
func BenchmarkUserIsolation_ConcurrentUsers(b *testing.B) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		b.Skip("POSTGRES_TEST_DSN environment variable not set")
		return
	}

	c := qt.New(b)

	c.Assert(setupFreshDatabase(dsn), qt.IsNil, qt.Commentf("failed to set up fresh database"))

	registrySetFunc, cleanup := postgres.NewPostgresRegistrySet()
	defer cleanup()
	factorySet, err := registrySetFunc(registry.Config(dsn))
	c.Assert(err, qt.IsNil, qt.Commentf("Failed to create registry set: %v", err))

	tenant := createIsolationTenant(c, factorySet)

	// Create test users, each with their own group + ready context + area.
	const numUsers = 10
	fixtures := make([]isolationFixture, numUsers)
	areaIDs := make([]string, numUsers)
	for i := range numUsers {
		f := newGroupedUser(c, factorySet, tenant, fmt.Sprintf("bench-user-%d@example.com", i), fmt.Sprintf("Bench Group %d", i))
		loc := seedLocation(c, factorySet, f, fmt.Sprintf("Bench Location %d", i))
		area := seedArea(c, factorySet, f, loc.ID, fmt.Sprintf("Bench Area %d", i))
		fixtures[i] = f
		areaIDs[i] = area.ID
	}

	b.ResetTimer()

	b.Run("ConcurrentCommodityOperations", func(b *testing.B) {
		c := qt.New(b)
		b.RunParallel(func(pb *testing.PB) {
			userIndex := 0
			for pb.Next() {
				f := fixtures[userIndex%len(fixtures)]
				areaID := areaIDs[userIndex%len(areaIDs)]

				reg := must.Must(factorySet.CommodityRegistryFactory.CreateUserRegistry(f.ctx))

				created := must.Must(reg.Create(f.ctx, models.Commodity{
					TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
						TenantID:        f.tenant.ID,
						GroupID:         f.group.ID,
						CreatedByUserID: f.user.ID,
					},
					Name:                   fmt.Sprintf("Benchmark Commodity %d-%d", userIndex, time.Now().UnixNano()),
					ShortName:              fmt.Sprintf("BC%d", userIndex),
					AreaID:                 new(areaID),
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
				}))

				// List commodities — should only see own group's commodities.
				commodities := must.Must(reg.List(f.ctx))
				for _, commodity := range commodities {
					c.Assert(commodity.GetCreatedByUserID(), qt.Equals, f.user.ID)
				}

				// Clean up.
				c.Assert(reg.Delete(f.ctx, created.ID), qt.IsNil)

				userIndex++
			}
		})
	})
}

// TestUserIsolation_LoadTesting tests user isolation under load. Excluded from
// CI via `-skip 'LoadTesting'`; kept compiling + correct for local runs.
func TestUserIsolation_LoadTesting(t *testing.T) {
	c := qt.New(t)
	fs, cleanup := setupTestDatabase(t)
	defer cleanup()

	tenant := createIsolationTenant(c, fs)

	// Create multiple users, each in their own group.
	numUsers := 50
	fixtures := make([]isolationFixture, numUsers)
	for i := range numUsers {
		fixtures[i] = newGroupedUser(c, fs, tenant, fmt.Sprintf("load-user-%d@example.com", i), fmt.Sprintf("Load Group %d", i))
	}

	// Each user creates commodities concurrently.
	var wg sync.WaitGroup
	errCh := make(chan error, numUsers*10)

	for userIndex, f := range fixtures {
		wg.Add(1)
		go func(userIndex int, f isolationFixture) {
			defer wg.Done()

			locReg, err := fs.LocationRegistryFactory.CreateUserRegistry(f.ctx)
			if err != nil {
				errCh <- fmt.Errorf("user %d failed to create location registry: %w", userIndex, err)
				return
			}
			createdLocation, err := locReg.Create(f.ctx, models.Location{
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
					TenantID:        f.tenant.ID,
					GroupID:         f.group.ID,
					CreatedByUserID: f.user.ID,
				},
				Name:    fmt.Sprintf("Load Test Location %d", userIndex),
				Address: fmt.Sprintf("123 Load Street %d", userIndex),
			})
			if err != nil {
				errCh <- fmt.Errorf("user %d failed to create location: %w", userIndex, err)
				return
			}

			areaReg, err := fs.AreaRegistryFactory.CreateUserRegistry(f.ctx)
			if err != nil {
				errCh <- fmt.Errorf("user %d failed to create area registry: %w", userIndex, err)
				return
			}
			createdArea, err := areaReg.Create(f.ctx, models.Area{
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
					TenantID:        f.tenant.ID,
					GroupID:         f.group.ID,
					CreatedByUserID: f.user.ID,
				},
				Name:       fmt.Sprintf("Load Test Area %d", userIndex),
				LocationID: createdLocation.ID,
			})
			if err != nil {
				errCh <- fmt.Errorf("user %d failed to create area: %w", userIndex, err)
				return
			}

			comReg, err := fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx)
			if err != nil {
				errCh <- fmt.Errorf("user %d failed to create commodity registry: %w", userIndex, err)
				return
			}

			for j := range 10 {
				_, err = comReg.Create(f.ctx, models.Commodity{
					TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
						TenantID:        f.tenant.ID,
						GroupID:         f.group.ID,
						CreatedByUserID: f.user.ID,
					},
					Name:                   fmt.Sprintf("Load Test Commodity %d-%d", userIndex, j),
					ShortName:              fmt.Sprintf("LTC%d%d", userIndex, j),
					AreaID:                 new(createdArea.ID),
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
				})
				if err != nil {
					errCh <- fmt.Errorf("user %d failed to create commodity %d: %w", userIndex, j, err)
					return
				}
			}

			commodities, err := comReg.List(f.ctx)
			if err != nil {
				errCh <- fmt.Errorf("user %d failed to list commodities: %w", userIndex, err)
				return
			}
			if len(commodities) != 10 {
				errCh <- fmt.Errorf("user %d sees %d commodities, expected 10", userIndex, len(commodities))
				return
			}
			for _, commodity := range commodities {
				if commodity.GetCreatedByUserID() != f.user.ID {
					errCh <- fmt.Errorf("user %d can see commodity belonging to user %s", userIndex, commodity.GetCreatedByUserID())
					return
				}
			}
		}(userIndex, f)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		c.Errorf("Load test error: %v", err)
	}
}

// TestUserIsolation_SecurityBoundaries tests security edge cases: a battery of
// malicious / malformed user identities must never be able to read, list,
// update, or delete a legitimate user's commodity. This test RUNS in CI.
func TestUserIsolation_SecurityBoundaries(t *testing.T) {
	c := qt.New(t)
	fs, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Create a legitimate user + group and seed a commodity they own.
	tenant := createIsolationTenant(c, fs)
	legit := newGroupedUser(c, fs, tenant, "legitimate@example.com", "Legit Group")

	loc := seedLocation(c, fs, legit, "Security Test Location")
	area := seedArea(c, fs, legit, loc.ID, "Security Test Area")
	created := seedCommodity(c, fs, legit, area.ID, "Security Test Commodity", "STC")

	// Various malicious user IDs. Each is paired with a bogus group so the
	// registry factory always builds successfully and the RLS denial — not a
	// missing-group setup error — is what we exercise.
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
		testName := fmt.Sprintf("Malicious ID: %q", maliciousID)
		if len(maliciousID) > 100 {
			testName = fmt.Sprintf("Malicious ID: very_long_string_%d_bytes", len(maliciousID))
		}
		t.Run(testName, func(t *testing.T) {
			c := qt.New(t)
			maliciousCtx := userGroupContext(
				context.Background(),
				&models.User{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: maliciousID},
						TenantID: "non-existent-tenant-id",
					},
				},
				&models.LocationGroup{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: "non-existent-group-id"},
						TenantID: "non-existent-tenant-id",
					},
					GroupCurrency: models.Currency("USD"),
				},
			)

			maliciousReg, err := fs.CommodityRegistryFactory.CreateUserRegistry(maliciousCtx)
			if err != nil {
				// If CreateUserRegistry fails, that's an acceptable outcome for a
				// malicious context — denial achieved.
				return
			}

			// Cannot Get the legit commodity.
			_, err = maliciousReg.Get(maliciousCtx, created.ID)
			c.Assert(err, qt.IsNotNil)

			// List must not leak the legit commodity.
			commodities, err := maliciousReg.List(maliciousCtx)
			if err == nil {
				c.Assert(commodities, qt.HasLen, 0)
			}

			// Cannot Update.
			tampered := *created
			tampered.Name = "Hacked Name"
			_, err = maliciousReg.Update(maliciousCtx, tampered)
			c.Assert(err, qt.IsNotNil)

			// Cannot Delete.
			err = maliciousReg.Delete(maliciousCtx, created.ID)
			c.Assert(err, qt.IsNotNil)
		})
	}

	// The legit commodity survives the assault intact.
	legitReg := must.Must(fs.CommodityRegistryFactory.CreateUserRegistry(legit.ctx))
	retrieved, err := legitReg.Get(legit.ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(retrieved.Name, qt.Equals, "Security Test Commodity")
}

// TestUserIsolation_PerformanceRegression tests for performance regressions.
// Excluded from CI via `-skip 'PerformanceRegression'`; kept compiling + correct.
func TestUserIsolation_PerformanceRegression(t *testing.T) {
	c := qt.New(t)
	fs, cleanup := setupTestDatabase(t)
	defer cleanup()

	tenant := createIsolationTenant(c, fs)
	f := newGroupedUser(c, fs, tenant, "perf@example.com", "Perf Group")

	loc := seedLocation(c, fs, f, "Performance Test Location")
	area := seedArea(c, fs, f, loc.ID, "Performance Test Area")

	numCommodities := 1000
	comReg := must.Must(fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))

	for i := range numCommodities {
		_ = seedCommodity(c, fs, f, area.ID,
			fmt.Sprintf("Performance Test Commodity %d", i),
			fmt.Sprintf("PTC%d", i))
	}

	start := time.Now()
	commodities, err := comReg.List(f.ctx)
	duration := time.Since(start)

	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, numCommodities)

	if duration > time.Second {
		t.Errorf("List operation took too long: %v", duration)
	}

	t.Logf("Listed %d commodities in %v", numCommodities, duration)
}
