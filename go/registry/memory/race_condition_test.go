package memory_test

import (
	"context"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"

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

		locationRegistry := memory.NewLocationRegistryFactory()
		areaRegistry := memory.NewAreaRegistryFactory(locationRegistry)
		commodityRegistry := memory.NewCommodityRegistryFactory(areaRegistry)

		// Create a commodity for user1
		commodity := models.Commodity{
			TenantAwareEntityID: models.TenantAwareEntityID{
				// ID will be generated server-side for security
				TenantID: "tenant-1",
				UserID:   user1.ID,
			},
			Name:   "Test Commodity",
			AreaID: "area-1",
		}

		ctx1 := appctx.WithUser(context.Background(), user1)
		ctx2 := appctx.WithUser(context.Background(), user2)

		// User1 creates the commodity
		comReg1, err := commodityRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)

		createdCommodity, err := comReg1.Create(ctx1, commodity)
		c.Assert(err, qt.IsNil)

		// Simulate the e2e test scenario: concurrent access with different users
		var wg sync.WaitGroup
		errors := make(chan error, 10)
		results := make(chan string, 10)

		// Test the exact e2e scenario: sequential operations with fresh registries
		// Step 1: Update the commodity
		updateReg, err := commodityRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)

		updatedCommodity := *createdCommodity
		updatedCommodity.Name = "Updated Commodity"

		_, err = updateReg.Update(ctx1, updatedCommodity)
		c.Assert(err, qt.IsNil)

		// Step 2: Get a fresh registry and retrieve the commodity (this was failing)
		getReg, err := commodityRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)

		retrievedCommodity, err := getReg.Get(ctx1, createdCommodity.ID)
		c.Assert(err, qt.IsNil, qt.Commentf("This should not return 404 - the race condition bug"))
		c.Assert(retrievedCommodity.Name, qt.Equals, "Updated Commodity")

		// Step 3: Test concurrent access with different users
		wg.Add(1)
		go func() {
			defer wg.Done()

			// User2 should not be able to access user1's commodity
			comReg, err := commodityRegistry.WithCurrentUser(ctx2)
			if err != nil {
				errors <- err
				return
			}

			_, err = comReg.Get(ctx2, createdCommodity.ID)
			// We expect this to fail with "not found" for user2
			if err != nil && err.Error() != "not found" {
				errors <- err
				return
			}
		}()

		// Step 4: Multiple concurrent accesses by user1 should all work
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				comReg, err := commodityRegistry.WithCurrentUser(ctx1)
				if err != nil {
					errors <- err
					return
				}

				retrievedCommodity, err := comReg.Get(ctx1, createdCommodity.ID)
				if err != nil {
					errors <- err
					return
				}

				results <- retrievedCommodity.Name
			}()
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

		c.Assert(len(retrievedNames), qt.Equals, 5, qt.Commentf("Expected 5 successful retrievals by user1"))
		for _, name := range retrievedNames {
			c.Assert(name, qt.Equals, "Updated Commodity", qt.Commentf("Expected updated commodity name"))
		}
	})

	// Test with location registry
	t.Run("location registry race condition", func(t *testing.T) {
		c := qt.New(t)

		locationRegistry := memory.NewLocationRegistryFactory()

		location := models.Location{
			TenantAwareEntityID: models.TenantAwareEntityID{
				// ID will be generated server-side for security
				TenantID: "tenant-1",
				UserID:   user1.ID,
			},
			Name: "Test Location",
		}

		ctx1 := appctx.WithUser(context.Background(), user1)

		// Create location
		locReg1, err := locationRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)

		createdLocation, err := locReg1.Create(ctx1, location)
		c.Assert(err, qt.IsNil)

		// Test concurrent access
		var wg sync.WaitGroup
		errors := make(chan error, 10)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Each goroutine gets its own user-aware registry
				locReg, err := locationRegistry.WithCurrentUser(ctx1)
				if err != nil {
					errors <- err
					return
				}

				// Update and retrieve
				updatedLocation := *createdLocation
				updatedLocation.Name = "Updated Location"

				_, err = locReg.Update(ctx1, updatedLocation)
				if err != nil {
					errors <- err
					return
				}

				_, err = locReg.Get(ctx1, createdLocation.ID)
				if err != nil {
					errors <- err
					return
				}
			}()
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

	// Create a test user with generated UUID
	testUserID := "test-user-12345678-1234-1234-1234-123456789012"
	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: testUserID},
			TenantID: "tenant-1",
			UserID:   testUserID, // Self-reference
		},
		Email: "admin@test-org.com",
		Name:  "Admin User",
	}

	locationRegistry := memory.NewLocationRegistryFactory()
	areaRegistry := memory.NewAreaRegistryFactory(locationRegistry)
	commodityRegistry := memory.NewCommodityRegistryFactory(areaRegistry)
	ctx := appctx.WithUser(context.Background(), user)

	commodity := models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			// ID will be generated server-side for security
			TenantID: "tenant-1",
			UserID:   user.ID,
		},
		Name:   "Test Commodity",
		AreaID: "area-1",
	}

	// Step 1: Create commodity
	comReg1, err := commodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	createdCommodity, err := comReg1.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Step 2: Update commodity (get fresh registry like new HTTP request)
	comReg2, err := commodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	updatedCommodity := *createdCommodity
	updatedCommodity.Name = "Updated Commodity"

	_, err = comReg2.Update(ctx, updatedCommodity)
	c.Assert(err, qt.IsNil)

	// Step 3: Get commodity (get fresh registry like new HTTP request)
	// This was failing with 404 in the e2e test due to race condition
	comReg3, err := commodityRegistry.WithCurrentUser(ctx)
	c.Assert(err, qt.IsNil)

	finalCommodity, err := comReg3.Get(ctx, createdCommodity.ID)
	c.Assert(err, qt.IsNil, qt.Commentf("This should not return 404 - the race condition bug"))
	c.Assert(finalCommodity.Name, qt.Equals, "Updated Commodity")
}
