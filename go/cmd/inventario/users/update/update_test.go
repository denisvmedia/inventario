package update_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/cmd/inventario/users/update"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

const testDSN = "postgres://test:test@localhost/test"

// TestMain lowers the bcrypt cost factor used by models.User.SetPassword for
// this test binary so the password-change fixtures don't pay the production
// bcrypt.DefaultCost. Production CLI callers keep DefaultCost.
func TestMain(m *testing.M) {
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)
	os.Exit(m.Run())
}

// seededRegistry registers a memory registry under the "postgres" name and
// pre-populates a tenant plus a single user. The same in-memory store backs
// every FactorySet the registered factory hands out (including the one the
// admin service opens), so tests can read the user back through `fs` to assert
// on the persisted state. The created user's server-generated ID is returned
// because UserRegistry.Create ignores any caller-supplied ID.
func seededRegistry(c *qt.C) (fs *registry.FactorySet, userID string) {
	newFn, _ := memory.NewMemoryRegistrySet()

	// Build and seed the store exactly once, then hand the same FactorySet to
	// every caller of the registered factory so the admin service shares it.
	factorySet, err := newFn(registry.Config(testDSN))
	c.Assert(err, qt.IsNil)

	serviceRegistrySet := factorySet.CreateServiceRegistrySet()
	ctx := context.Background()

	_, err = serviceRegistrySet.TenantRegistry.Create(ctx, models.Tenant{
		EntityID: models.EntityID{ID: "test-tenant"},
		Name:     "Test Tenant",
		Slug:     "test-tenant",
		Status:   models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant",
		},
		Email:    "user@example.com",
		Name:     "Original Name",
		IsActive: true,
	}
	err = user.SetPassword("OriginalPass123")
	c.Assert(err, qt.IsNil)

	created, err := serviceRegistrySet.UserRegistry.Create(ctx, user)
	c.Assert(err, qt.IsNil)

	wrappedNewFn := func(_ registry.Config) (*registry.FactorySet, error) {
		return factorySet, nil
	}
	registry.Register("postgres", wrappedNewFn)
	c.Cleanup(func() {
		registry.Unregister("postgres")
	})

	return factorySet, created.ID
}

func readUser(c *qt.C, fs *registry.FactorySet, id string) *models.User {
	user, err := fs.CreateServiceRegistrySet().UserRegistry.Get(context.Background(), id)
	c.Assert(err, qt.IsNil)
	return user
}

func TestCommand_New(t *testing.T) {
	c := qt.New(t)

	seededRegistry(c)

	cmd := update.New(&shared.DatabaseConfig{DBDSN: testDSN})
	c.Assert(cmd, qt.IsNotNil)
	c.Assert(cmd.Cmd(), qt.IsNotNil)
	c.Assert(cmd.Cmd().Use, qt.Equals, "update <user-id-or-email>")
	c.Assert(cmd.Cmd().Short, qt.Equals, "Update an existing user")
}

func TestCommand_Flags(t *testing.T) {
	c := qt.New(t)

	seededRegistry(c)

	cmd := update.New(&shared.DatabaseConfig{DBDSN: testDSN})
	cobraCmd := cmd.Cmd()

	expectedFlags := []string{
		"dry-run",
		"email",
		"name",
		"active",
		"tenant",
		"password",
		"interactive",
	}
	for _, flagName := range expectedFlags {
		flag := cobraCmd.Flags().Lookup(flagName)
		c.Assert(flag, qt.IsNotNil, qt.Commentf("Flag %s should exist", flagName))
	}

	// The dead --role flag must NOT exist.
	c.Assert(cobraCmd.Flags().Lookup("role"), qt.IsNil)
}

// TestCommand_LiveUpdate_Mutates is the regression guard against the old no-op
// stub: a live update must actually persist the requested changes.
func TestCommand_LiveUpdate_Mutates(t *testing.T) {
	c := qt.New(t)

	fs, userID := seededRegistry(c)

	cmd := update.New(&shared.DatabaseConfig{DBDSN: testDSN})
	cobraCmd := cmd.Cmd()

	var out bytes.Buffer
	cobraCmd.SetOut(&out)
	cobraCmd.SetArgs([]string{
		"user@example.com",
		"--name=New Name",
		"--active=false",
	})

	err := cobraCmd.Execute()
	c.Assert(err, qt.IsNil)

	updated := readUser(c, fs, userID)
	c.Assert(updated.Name, qt.Equals, "New Name")
	c.Assert(updated.IsActive, qt.IsFalse)
	// Untouched fields are preserved.
	c.Assert(updated.Email, qt.Equals, "user@example.com")
}

// TestCommand_DryRun_MutatesNothing verifies the dry-run path reports the
// changes but leaves the persisted user untouched.
func TestCommand_DryRun_MutatesNothing(t *testing.T) {
	c := qt.New(t)

	fs, userID := seededRegistry(c)

	cmd := update.New(&shared.DatabaseConfig{DBDSN: testDSN})
	cobraCmd := cmd.Cmd()

	var out bytes.Buffer
	cobraCmd.SetOut(&out)
	cobraCmd.SetArgs([]string{
		"user@example.com",
		"--name=Should Not Persist",
		"--dry-run",
	})

	err := cobraCmd.Execute()
	c.Assert(err, qt.IsNil)
	c.Assert(out.String(), qt.Contains, "DRY RUN")

	unchanged := readUser(c, fs, userID)
	c.Assert(unchanged.Name, qt.Equals, "Original Name")
	c.Assert(unchanged.IsActive, qt.IsTrue)
}

// TestCommand_NoFlags_NoChange verifies that running with no field flags makes
// no change and reports that nothing was requested.
func TestCommand_NoFlags_NoChange(t *testing.T) {
	c := qt.New(t)

	fs, userID := seededRegistry(c)

	cmd := update.New(&shared.DatabaseConfig{DBDSN: testDSN})
	cobraCmd := cmd.Cmd()

	var out bytes.Buffer
	cobraCmd.SetOut(&out)
	cobraCmd.SetArgs([]string{"user@example.com"})

	err := cobraCmd.Execute()
	c.Assert(err, qt.IsNil)
	c.Assert(out.String(), qt.Contains, "No changes requested")

	unchanged := readUser(c, fs, userID)
	c.Assert(unchanged.Name, qt.Equals, "Original Name")
}

func TestCommand_MemoryDatabaseRejected(t *testing.T) {
	c := qt.New(t)

	cmd := update.New(&shared.DatabaseConfig{DBDSN: "memory://"})
	cobraCmd := cmd.Cmd()

	var out bytes.Buffer
	cobraCmd.SetOut(&out)
	cobraCmd.SetArgs([]string{"user@example.com", "--name=New Name"})

	err := cobraCmd.Execute()
	c.Assert(err, qt.IsNotNil)
	// dbConfig.Validate() rejects non-PostgreSQL DSNs before the command's own
	// explicit memory:// guard is reached, so the operator gets a clear
	// PostgreSQL-only error either way.
	c.Assert(err.Error(), qt.Contains, "only support PostgreSQL")
}
