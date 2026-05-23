package postgres_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// createGrantTestUser creates a second user in the test tenant so the
// grant tests can flip grants on a separate row from the seeded
// admin@test-org.com user. Returns the new user's ID.
func createGrantTestUser(c *qt.C, registrySet *registry.Set, email string) string {
	c.Helper()
	ctx := context.Background()

	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.Not(qt.HasLen), 0)
	seedTenantID := users[0].TenantID

	u := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: seedTenantID},
		Email:               email,
		Name:                "Grant Test " + email,
		IsActive:            true,
	}
	c.Assert(u.SetPassword("TestPassword123"), qt.IsNil)
	created, err := registrySet.UserRegistry.Create(ctx, u)
	c.Assert(err, qt.IsNil)
	return created.ID
}

// resetGrants wipes the system_admin_grants table between subtests so
// the last-admin guard and ordering tests can rely on a clean slate
// without re-running the entire migration suite. allowZero=true makes
// the call legal regardless of current grant count.
func resetGrants(c *qt.C, registrySet *registry.Set) {
	c.Helper()
	ctx := context.Background()
	grants, err := registrySet.SystemAdminGrantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	for _, g := range grants {
		_, err := registrySet.SystemAdminGrantRegistry.RevokeAtomic(ctx, g.UserID, true)
		c.Assert(err, qt.IsNil)
	}
}

func TestSystemAdminGrantRegistry_Postgres_Grant_Idempotent(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	resetGrants(c, registrySet)
	userID := createGrantTestUser(c, registrySet, "grant-idempotent@test-org.com")

	hadGrant, err := registrySet.SystemAdminGrantRegistry.Grant(ctx, userID, nil)
	c.Assert(err, qt.IsNil)
	c.Assert(hadGrant, qt.IsFalse)

	hadGrant, err = registrySet.SystemAdminGrantRegistry.Grant(ctx, userID, nil)
	c.Assert(err, qt.IsNil)
	c.Assert(hadGrant, qt.IsTrue)

	grants, err := registrySet.SystemAdminGrantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(grants, qt.HasLen, 1)
	c.Assert(grants[0].UserID, qt.Equals, userID)
}

func TestSystemAdminGrantRegistry_Postgres_Exists_AfterGrantAndRevoke(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	resetGrants(c, registrySet)
	keeperID := createGrantTestUser(c, registrySet, "grant-exists-keeper@test-org.com")
	subjectID := createGrantTestUser(c, registrySet, "grant-exists-subject@test-org.com")

	// Seed a keeper so the revoke under test doesn't trip the
	// last-admin guard.
	_, err := registrySet.SystemAdminGrantRegistry.Grant(ctx, keeperID, nil)
	c.Assert(err, qt.IsNil)

	exists, err := registrySet.SystemAdminGrantRegistry.Exists(ctx, subjectID)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)

	_, err = registrySet.SystemAdminGrantRegistry.Grant(ctx, subjectID, nil)
	c.Assert(err, qt.IsNil)

	exists, err = registrySet.SystemAdminGrantRegistry.Exists(ctx, subjectID)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)

	hadGrant, err := registrySet.SystemAdminGrantRegistry.RevokeAtomic(ctx, subjectID, false)
	c.Assert(err, qt.IsNil)
	c.Assert(hadGrant, qt.IsTrue)

	exists, err = registrySet.SystemAdminGrantRegistry.Exists(ctx, subjectID)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
}

func TestSystemAdminGrantRegistry_Postgres_RevokeAtomic_NoGrant(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	resetGrants(c, registrySet)
	userID := createGrantTestUser(c, registrySet, "grant-revoke-nograntee@test-org.com")

	hadGrant, err := registrySet.SystemAdminGrantRegistry.RevokeAtomic(ctx, userID, false)
	c.Assert(err, qt.IsNil)
	c.Assert(hadGrant, qt.IsFalse)
}

func TestSystemAdminGrantRegistry_Postgres_RevokeAtomic_LastAdminGuard(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	resetGrants(c, registrySet)
	userID := createGrantTestUser(c, registrySet, "grant-last-admin@test-org.com")

	_, err := registrySet.SystemAdminGrantRegistry.Grant(ctx, userID, nil)
	c.Assert(err, qt.IsNil)

	hadGrant, err := registrySet.SystemAdminGrantRegistry.RevokeAtomic(ctx, userID, false)
	c.Assert(errors.Is(err, registry.ErrLastSystemAdmin), qt.IsTrue,
		qt.Commentf("expected ErrLastSystemAdmin, got %v", err))
	c.Assert(hadGrant, qt.IsTrue)

	exists, err := registrySet.SystemAdminGrantRegistry.Exists(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)
}

func TestSystemAdminGrantRegistry_Postgres_RevokeAtomic_AllowZeroBypass(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	resetGrants(c, registrySet)
	userID := createGrantTestUser(c, registrySet, "grant-allow-zero@test-org.com")

	_, err := registrySet.SystemAdminGrantRegistry.Grant(ctx, userID, nil)
	c.Assert(err, qt.IsNil)

	hadGrant, err := registrySet.SystemAdminGrantRegistry.RevokeAtomic(ctx, userID, true)
	c.Assert(err, qt.IsNil)
	c.Assert(hadGrant, qt.IsTrue)

	grants, err := registrySet.SystemAdminGrantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(grants, qt.HasLen, 0)
}

// TestSystemAdminGrantRegistry_Postgres_RevokeAtomic_NoConcurrentLastAdminRevoke
// pins the postgres race-safety guarantee. The advisory lock + FOR
// UPDATE on the candidate row serialise the two goroutines so exactly
// one ends with hadGrant=true and exactly one hits ErrLastSystemAdmin.
func TestSystemAdminGrantRegistry_Postgres_RevokeAtomic_NoConcurrentLastAdminRevoke(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	resetGrants(c, registrySet)
	aliceID := createGrantTestUser(c, registrySet, "grant-race-alice@test-org.com")
	bobID := createGrantTestUser(c, registrySet, "grant-race-bob@test-org.com")

	_, err := registrySet.SystemAdminGrantRegistry.Grant(ctx, aliceID, nil)
	c.Assert(err, qt.IsNil)
	_, err = registrySet.SystemAdminGrantRegistry.Grant(ctx, bobID, nil)
	c.Assert(err, qt.IsNil)

	var wg sync.WaitGroup
	results := make([]struct {
		hadGrant bool
		err      error
	}, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		results[0].hadGrant, results[0].err = registrySet.SystemAdminGrantRegistry.RevokeAtomic(ctx, aliceID, false)
	}()
	go func() {
		defer wg.Done()
		results[1].hadGrant, results[1].err = registrySet.SystemAdminGrantRegistry.RevokeAtomic(ctx, bobID, false)
	}()
	wg.Wait()

	successes := 0
	rejections := 0
	for _, res := range results {
		switch {
		case res.err == nil && res.hadGrant:
			successes++
		case errors.Is(res.err, registry.ErrLastSystemAdmin):
			rejections++
		}
	}
	c.Assert(successes, qt.Equals, 1)
	c.Assert(rejections, qt.Equals, 1)

	grants, err := registrySet.SystemAdminGrantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(grants, qt.HasLen, 1)
}

func TestSystemAdminGrantRegistry_Postgres_List_OrderedByGrantedAt(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	resetGrants(c, registrySet)
	first := createGrantTestUser(c, registrySet, "grant-order-1@test-org.com")
	second := createGrantTestUser(c, registrySet, "grant-order-2@test-org.com")
	third := createGrantTestUser(c, registrySet, "grant-order-3@test-org.com")

	for _, id := range []string{first, second, third} {
		_, err := registrySet.SystemAdminGrantRegistry.Grant(ctx, id, nil)
		c.Assert(err, qt.IsNil)
	}

	grants, err := registrySet.SystemAdminGrantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(grants, qt.HasLen, 3)
	ids := []string{grants[0].UserID, grants[1].UserID, grants[2].UserID}
	c.Assert(ids, qt.DeepEquals, []string{first, second, third})
}

func TestSystemAdminGrantRegistry_Postgres_Exists_EmptyUserID(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	resetGrants(c, registrySet)

	_, err := registrySet.SystemAdminGrantRegistry.Exists(ctx, "")
	c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue,
		qt.Commentf("expected ErrFieldRequired, got %v", err))
}

// TestSystemAdminGrantRegistry_Postgres_UserDelete_CascadesGrant pins
// the FK ON DELETE CASCADE on user_id: hard-deleting a user MUST
// remove their grant row so there are no dangling FK references.
func TestSystemAdminGrantRegistry_Postgres_UserDelete_CascadesGrant(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	resetGrants(c, registrySet)
	keeperID := createGrantTestUser(c, registrySet, "grant-cascade-keeper@test-org.com")
	subjectID := createGrantTestUser(c, registrySet, "grant-cascade-subject@test-org.com")

	_, err := registrySet.SystemAdminGrantRegistry.Grant(ctx, keeperID, nil)
	c.Assert(err, qt.IsNil)
	_, err = registrySet.SystemAdminGrantRegistry.Grant(ctx, subjectID, nil)
	c.Assert(err, qt.IsNil)

	// Hard-delete the subject. The cascade should drop the grant row.
	err = registrySet.UserRegistry.Delete(ctx, subjectID)
	c.Assert(err, qt.IsNil)

	exists, err := registrySet.SystemAdminGrantRegistry.Exists(ctx, subjectID)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)

	grants, err := registrySet.SystemAdminGrantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(grants, qt.HasLen, 1)
	c.Assert(grants[0].UserID, qt.Equals, keeperID)
}
