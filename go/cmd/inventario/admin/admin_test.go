package admin_test

import (
	"bytes"
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/admin"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	adminservice "github.com/denisvmedia/inventario/services/admin"
)

// adminTestFixture seeds two users into a memory registry registered as
// "postgres" so the CLI's PostgreSQL-only guard treats it like the real
// thing. It returns the IDs so individual tests can target them.
type adminTestFixture struct {
	factorySet *registry.FactorySet
	tenantID   string
	user1ID    string
	user1Email string
	user2ID    string
	user2Email string
}

func setupAdminTestFixture(c *qt.C) *adminTestFixture {
	// NewMemoryRegistrySet returns (constructor, cleanup) — the cleanup
	// closer is a no-op for the memory backend, so it's intentionally
	// discarded here.
	newFn, _ := memory.NewMemoryRegistrySet()

	fx := &adminTestFixture{}

	// Build ONE shared factory set + seed users. Subsequent calls to the
	// registry constructor return the SAME factory set — otherwise every
	// `admin.NewService(dbConfig)` from a different CLI invocation builds
	// a fresh empty store and the second call can't find the user that
	// the first call just touched.
	fs, err := newFn(registry.Config("postgres://test:test@localhost/test"))
	c.Assert(err, qt.IsNil)
	fx.factorySet = fs

	serviceSet := fs.CreateServiceRegistrySet()
	tenant, terr := serviceSet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Test Tenant",
		Slug:   "test-tenant",
		Status: models.TenantStatusActive,
	})
	c.Assert(terr, qt.IsNil)
	fx.tenantID = tenant.ID

	mkUser := func(email, name string) *models.User {
		u := models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
			Email:               email,
			Name:                name,
			IsActive:            true,
		}
		c.Assert(u.SetPassword("Password123"), qt.IsNil)
		created, cerr := fs.UserRegistry.Create(context.Background(), u)
		c.Assert(cerr, qt.IsNil)
		return created
	}

	u1 := mkUser("alice@example.com", "Alice")
	fx.user1ID = u1.ID
	fx.user1Email = u1.Email
	u2 := mkUser("bob@example.com", "Bob")
	fx.user2ID = u2.ID
	fx.user2Email = u2.Email

	registry.Register("postgres", func(_ registry.Config) (*registry.FactorySet, error) {
		return fs, nil
	})
	c.Cleanup(func() {
		registry.Unregister("postgres")
	})

	return fx
}

// runAdminCommand executes the admin command with the provided args and
// captures its combined output. Returns (stdout+stderr, err).
func runAdminCommand(c *qt.C, args ...string) (string, error) {
	c.Helper()

	dbConfig := &shared.DatabaseConfig{
		DBDSN: "postgres://test:test@localhost/test",
	}
	cmd := admin.New(dbConfig)

	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

// hasSystemAdminGrant asserts the post-state via the dedicated grant
// store (#1784) — the legacy IsSystemAdmin column on users is gone, so
// "is this user an admin?" now means "is there a row in
// system_admin_grants?".
func hasSystemAdminGrant(c *qt.C, fx *adminTestFixture, id string) bool {
	c.Helper()
	ok, err := fx.factorySet.SystemAdminGrantRegistry.Exists(context.Background(), id)
	c.Assert(err, qt.IsNil)
	return ok
}

func TestAdmin_GrantListRevoke_HappyPath(t *testing.T) {
	c := qt.New(t)
	fx := setupAdminTestFixture(c)

	// Grant alice
	out, err := runAdminCommand(c, "grant-system-admin", "--email", fx.user1Email)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Granted system-admin")
	c.Assert(hasSystemAdminGrant(c, fx, fx.user1ID), qt.IsTrue)

	// Grant bob so we have two admins (so we can revoke without --allow-zero).
	out, err = runAdminCommand(c, "grant-system-admin", "--email", fx.user2Email)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Granted system-admin")

	// List shows both
	out, err = runAdminCommand(c, "list-system-admins")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, fx.user1Email)
	c.Assert(out, qt.Contains, fx.user2Email)

	// Revoke alice — bob remains, so the guard allows it.
	out, err = runAdminCommand(c, "revoke-system-admin", "--email", fx.user1Email)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Revoked system-admin")
	c.Assert(hasSystemAdminGrant(c, fx, fx.user1ID), qt.IsFalse)
}

func TestAdmin_RevokeLastAdmin_RefusedWithoutAllowZero(t *testing.T) {
	c := qt.New(t)
	fx := setupAdminTestFixture(c)

	_, err := runAdminCommand(c, "grant-system-admin", "--email", fx.user1Email)
	c.Assert(err, qt.IsNil)

	// Revoke without --allow-zero. The CLI prints the friendly hint AND
	// returns the sentinel — the test asserts both.
	out, err := runAdminCommand(c, "revoke-system-admin", "--email", fx.user1Email)
	c.Assert(err, qt.IsNotNil)
	c.Assert(out, qt.Contains, "last system administrator")
	c.Assert(err.Error(), qt.Contains, adminservice.ErrLastSystemAdmin.Error())

	// User flag must still be true — the guard must not have flipped the row.
	c.Assert(hasSystemAdminGrant(c, fx, fx.user1ID), qt.IsTrue)
}

func TestAdmin_RevokeLastAdmin_AllowedWithFlag(t *testing.T) {
	c := qt.New(t)
	fx := setupAdminTestFixture(c)

	_, err := runAdminCommand(c, "grant-system-admin", "--email", fx.user1Email)
	c.Assert(err, qt.IsNil)

	out, err := runAdminCommand(c, "revoke-system-admin", "--email", fx.user1Email, "--allow-zero")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "Revoked system-admin")
	c.Assert(hasSystemAdminGrant(c, fx, fx.user1ID), qt.IsFalse)
}

func TestAdmin_GrantIdempotent(t *testing.T) {
	c := qt.New(t)
	fx := setupAdminTestFixture(c)

	_, err := runAdminCommand(c, "grant-system-admin", "--email", fx.user1Email)
	c.Assert(err, qt.IsNil)

	// Second grant should succeed and report idempotent.
	out, err := runAdminCommand(c, "grant-system-admin", "--email", fx.user1Email)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "already a system administrator")
}

func TestAdmin_RevokeNonAdmin_NoOp(t *testing.T) {
	c := qt.New(t)
	fx := setupAdminTestFixture(c)

	// alice has never been granted. Revoke must succeed and report no-op.
	out, err := runAdminCommand(c, "revoke-system-admin", "--email", fx.user1Email)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.Contains, "not a system administrator")
}

func TestAdmin_RejectsMemoryDSN(t *testing.T) {
	c := qt.New(t)
	setupAdminTestFixture(c)

	// shared.DatabaseConfig.Validate() rejects non-PostgreSQL DSNs up
	// front; that is the layer that surfaces in the CLI's error path,
	// so we assert against its sentinel message rather than the deeper
	// guard inside the admin run() helpers.
	dbConfig := &shared.DatabaseConfig{DBDSN: "memory://"}
	cmd := admin.New(dbConfig)
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"grant-system-admin", "--email", "alice@example.com"})
	err := cmd.Execute()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "only support PostgreSQL")
}
