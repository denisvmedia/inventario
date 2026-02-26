package memory_test

import (
	"context"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

// TestMemoryRegistryUserContextRaceCondition tests the race condition that was
// causing the e2e test to fail: multiple user-aware registries modifying the
// same userID field on a shared base registry.
func TestMemoryRegistryUserContextRaceCondition(t *testing.T) {
	// Create test users
	user1 := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
		Email: "user1@test.com",
		Name:  "User 1",
	}

	user2 := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-2"},
			TenantID: "tenant-1",
		},
		Email: "user2@test.com",
		Name:  "User 2",
	}

	// Test the race condition with commodity registry
	t.Run("commodity registry race condition", func(t *testing.T) {
		c := qt.New(t)

		// Create factory set and register users
		factorySet := memory.NewFactorySet()
		serviceRegistrySet := factorySet.CreateServiceRegistrySet()

		// Register users in the system
		u1, err := serviceRegistrySet.UserRegistry.Create(context.Background(), *user1)
		c.Assert(err, qt.IsNil)
		u2, err := serviceRegistrySet.UserRegistry.Create(context.Background(), *user2)
		c.Assert(err, qt.IsNil)

		// Create user contexts
		ctx1 := appctx.WithUser(context.Background(), u1)
		ctx2 := appctx.WithUser(context.Background(), u2)

		// Get user registry set for user1
		registrySet1 := must.Must(factorySet.CreateUserRegistrySet(ctx1))

		// Create a commodity for user1
		commodity := models.Commodity{
			TenantAwareEntityID: models.TenantAwareEntityID{
				// ID will be generated server-side for security
				TenantID: "tenant-1",
				UserID:   u1.ID,
			},
			Name:   "Test Commodity",
			AreaID: "area-1",
		}

		// User1 creates the commodity
		createdCommodity, err := registrySet1.CommodityRegistry.Create(ctx1, commodity)
		c.Assert(err, qt.IsNil)

		// Simulate the e2e test scenario: concurrent access with different users
		var wg sync.WaitGroup
		errors := make(chan error, 10)
		results := make(chan string, 10)

		// Test the exact e2e scenario: sequential operations with fresh registries
		// Step 1: Update the commodity
		updatedCommodity := *createdCommodity
		updatedCommodity.Name = "Updated Commodity"

		_, err = registrySet1.CommodityRegistry.Update(ctx1, updatedCommodity)
		c.Assert(err, qt.IsNil)

		// Step 2: Get a fresh registry and retrieve the commodity (this was failing)
		// Create a new registry set to simulate fresh registry access
		freshRegistrySet1 := must.Must(factorySet.CreateUserRegistrySet(ctx1))

		retrievedCommodity, err := freshRegistrySet1.CommodityRegistry.Get(ctx1, createdCommodity.ID)
		c.Assert(err, qt.IsNil, qt.Commentf("This should not return 404 - the race condition bug"))
		c.Assert(retrievedCommodity.Name, qt.Equals, "Updated Commodity")

		// Step 3: Test concurrent access with different users
		wg.Go(func() {

			// User2 should not be able to access user1's commodity
			// Create a fresh registry set for user2 to simulate concurrent access
			freshRegistrySet2 := must.Must(factorySet.CreateUserRegistrySet(ctx2))

			_, err := freshRegistrySet2.CommodityRegistry.Get(ctx2, createdCommodity.ID)
			// We expect this to fail with "not found" for user2
			if err != nil && err.Error() != "not found" {
				errors <- err
				return
			}
		})

		// Step 4: Multiple concurrent accesses by user1 should all work
		for range 5 {
			wg.Go(func() {

				// Create a fresh registry set for each concurrent access
				concurrentRegistrySet1 := must.Must(factorySet.CreateUserRegistrySet(ctx1))

				retrievedCommodity, err := concurrentRegistrySet1.CommodityRegistry.Get(ctx1, createdCommodity.ID)
				if err != nil {
					errors <- err
					return
				}

				results <- retrievedCommodity.Name
			})
		}

		wg.Wait()
		close(errors)
		close(results)

		// Check for any unexpected errors
		var unexpectedErrors []error
		for err := range errors {
			unexpectedErrors = append(unexpectedErrors, err)
		}

		c.Assert(unexpectedErrors, qt.HasLen, 0, qt.Commentf("Unexpected errors: %v", unexpectedErrors))

		// Check that all concurrent accesses by user1 got the updated commodity
		var retrievedNames []string
		for name := range results {
			retrievedNames = append(retrievedNames, name)
		}

		c.Assert(retrievedNames, qt.HasLen, 5, qt.Commentf("Expected 5 successful retrievals by user1"))
		for _, name := range retrievedNames {
			c.Assert(name, qt.Equals, "Updated Commodity", qt.Commentf("Expected updated commodity name"))
		}
	})

	// Test with location registry
	t.Run("location registry race condition", func(t *testing.T) {
		c := qt.New(t)

		// Create factory set and register users
		factorySet := memory.NewFactorySet()
		serviceRegistrySet := factorySet.CreateServiceRegistrySet()

		// Register users in the system
		u1, err := serviceRegistrySet.UserRegistry.Create(context.Background(), *user1)
		c.Assert(err, qt.IsNil)

		// Create user context
		ctx1 := appctx.WithUser(context.Background(), u1)

		// Get user registry set
		registrySet1 := must.Must(factorySet.CreateUserRegistrySet(ctx1))

		location := models.Location{
			TenantAwareEntityID: models.TenantAwareEntityID{
				// ID will be generated server-side for security
				TenantID: "tenant-1",
				UserID:   u1.ID,
			},
			Name: "Test Location",
		}

		// Create location
		createdLocation, err := registrySet1.LocationRegistry.Create(ctx1, location)
		c.Assert(err, qt.IsNil)

		// Test concurrent access
		var wg sync.WaitGroup
		errors := make(chan error, 10)

		for range 10 {
			wg.Go(func() {

				// Each goroutine gets its own user-aware registry set
				concurrentRegistrySet := must.Must(factorySet.CreateUserRegistrySet(ctx1))

				// Update and retrieve
				updatedLocation := *createdLocation
				updatedLocation.Name = "Updated Location"

				_, err := concurrentRegistrySet.LocationRegistry.Update(ctx1, updatedLocation)
				if err != nil {
					errors <- err
					return
				}

				_, err = concurrentRegistrySet.LocationRegistry.Get(ctx1, createdLocation.ID)
				if err != nil {
					errors <- err
					return
				}
			})
		}

		wg.Wait()
		close(errors)

		// Check for errors
		var allErrors []error
		for err := range errors {
			allErrors = append(allErrors, err)
		}

		c.Assert(allErrors, qt.HasLen, 0, qt.Commentf("Errors in concurrent access: %v", allErrors))
	})
}

// TestE2EScenarioSimulation simulates the exact e2e test scenario
func TestE2EScenarioSimulation(t *testing.T) {
	c := qt.New(t)

	// Create factory set and user
	factorySet := memory.NewFactorySet()
	serviceRegistrySet := factorySet.CreateServiceRegistrySet()

	// Create a test user with generated UUID
	testUserID := "test-user-12345678-1234-1234-1234-123456789012"
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: testUserID},
			TenantID: "tenant-1",
			UserID:   testUserID, // Self-reference
		},
		Email: "admin@test-org.com",
		Name:  "Admin User",
	}

	// Register user in the system
	u, err := serviceRegistrySet.UserRegistry.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	commodity := models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			// ID will be generated server-side for security
			TenantID: "tenant-1",
			UserID:   u.ID,
		},
		Name:   "Test Commodity",
		AreaID: "area-1",
	}

	// Step 1: Create commodity
	createdCommodity, err := registrySet.CommodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Step 2: Update commodity (get fresh registry like new HTTP request)
	freshRegistrySet2 := must.Must(factorySet.CreateUserRegistrySet(ctx))

	updatedCommodity := *createdCommodity
	updatedCommodity.Name = "Updated Commodity"

	_, err = freshRegistrySet2.CommodityRegistry.Update(ctx, updatedCommodity)
	c.Assert(err, qt.IsNil)

	// Step 3: Get commodity (get fresh registry like new HTTP request)
	// This was failing with 404 in the e2e test due to race condition
	freshRegistrySet3 := must.Must(factorySet.CreateUserRegistrySet(ctx))

	finalCommodity, err := freshRegistrySet3.CommodityRegistry.Get(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNil, qt.Commentf("This should not return 404 - the race condition bug"))
	c.Assert(finalCommodity.Name, qt.Equals, "Updated Commodity")
}
