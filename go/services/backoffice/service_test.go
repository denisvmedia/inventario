package backoffice_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services/backoffice"
)

// setupMemoryAsPostgres registers the in-memory registry under the `postgres`
// scheme so backoffice.NewService — which routes through registry.GetRegistry —
// resolves a working factory set without a real postgres instance. Returns the
// factory set so a test can introspect post-conditions (row count, identities).
// Mirrors the helper in cmd/inventario/backoffice/bootstrap/bootstrap_test.go.
func setupMemoryAsPostgres(c *qt.C) *registry.FactorySet {
	var captured *registry.FactorySet
	newFn, _ := memory.NewMemoryRegistrySet()
	wrappedNewFn := func(cfg registry.Config) (*registry.FactorySet, error) {
		if captured != nil {
			return captured, nil
		}
		fs, err := newFn(cfg)
		if err != nil {
			return nil, err
		}
		captured = fs
		return fs, nil
	}
	registry.Register("postgres", wrappedNewFn)
	c.Cleanup(func() {
		registry.Unregister("postgres")
	})

	fs, err := wrappedNewFn(registry.Config("postgres://test"))
	c.Assert(err, qt.IsNil)
	return fs
}

// newService builds a backoffice.Service backed by the memory-as-postgres
// registry registered by setupMemoryAsPostgres.
func newService(c *qt.C) *backoffice.Service {
	svc, err := backoffice.NewService(&shared.DatabaseConfig{DBDSN: "postgres://test"})
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() {
		_ = svc.Close()
	})
	return svc
}

// TestService_BootstrapEnsureNoOpOnDifferentEmail pins the #1967 self-heal at
// the service seam: with an existing operator under a different email, Ensure
// returns EnsuredExisting=true, no error, and no second row — and reports the
// existing operator via result.User.
func TestService_BootstrapEnsureNoOpOnDifferentEmail(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)
	svc := newService(c)
	ctx := context.Background()

	first, err := svc.Bootstrap(ctx, backoffice.BootstrapRequest{
		Email: "existing@example.com",
		Name:  "Existing Op",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(first.AlreadyExisted, qt.IsFalse)

	res, err := svc.Bootstrap(ctx, backoffice.BootstrapRequest{
		Email:  "operator@inventario.example",
		Name:   "Chart Operator",
		Ensure: true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(res.EnsuredExisting, qt.IsTrue)
	c.Assert(res.AlreadyExisted, qt.IsFalse)
	c.Assert(res.User, qt.IsNotNil)
	c.Assert(res.User.Email, qt.Equals, "existing@example.com")

	users, err := fs.BackofficeUserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 1)
}

// TestService_BootstrapEnsureCreatesOnFreshInstall pins that Ensure does not
// regress fresh installs (count=0): the operator is still created.
func TestService_BootstrapEnsureCreatesOnFreshInstall(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)
	svc := newService(c)
	ctx := context.Background()

	res, err := svc.Bootstrap(ctx, backoffice.BootstrapRequest{
		Email:  "operator@inventario.example",
		Name:   "Chart Operator",
		Ensure: true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(res.EnsuredExisting, qt.IsFalse)
	c.Assert(res.AlreadyExisted, qt.IsFalse)
	c.Assert(res.User, qt.IsNotNil)
	c.Assert(res.User.Email, qt.Equals, "operator@inventario.example")

	users, err := fs.BackofficeUserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 1)
}

// TestService_BootstrapSameEmailIdempotent pins that a same-email re-run still
// reports AlreadyExisted (not the Ensure no-op), independent of Ensure.
func TestService_BootstrapSameEmailIdempotent(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)
	svc := newService(c)
	ctx := context.Background()

	_, err := svc.Bootstrap(ctx, backoffice.BootstrapRequest{
		Email: "admin@example.com",
		Name:  "Ops Admin",
	})
	c.Assert(err, qt.IsNil)

	res, err := svc.Bootstrap(ctx, backoffice.BootstrapRequest{
		Email:  "admin@example.com",
		Name:   "Ops Admin",
		Ensure: true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(res.AlreadyExisted, qt.IsTrue)
	c.Assert(res.EnsuredExisting, qt.IsFalse)
	c.Assert(res.User, qt.IsNotNil)

	users, err := fs.BackofficeUserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 1)
}

// TestService_BootstrapForceAddsSecond pins that --force is unchanged: it still
// ADDS a second operator (distinct from Ensure, which leaves one untouched).
func TestService_BootstrapForceAddsSecond(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)
	svc := newService(c)
	ctx := context.Background()

	_, err := svc.Bootstrap(ctx, backoffice.BootstrapRequest{
		Email: "first@example.com",
		Name:  "First",
	})
	c.Assert(err, qt.IsNil)

	res, err := svc.Bootstrap(ctx, backoffice.BootstrapRequest{
		Email: "second@example.com",
		Name:  "Second",
		Force: true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(res.EnsuredExisting, qt.IsFalse)
	c.Assert(res.AlreadyExisted, qt.IsFalse)

	users, err := fs.BackofficeUserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 2)
}

// TestService_BootstrapRefusesWithoutEnsureOrForce pins the interactive safety
// net: with an existing operator and neither Ensure nor Force, the count>0
// refusal error is unchanged.
func TestService_BootstrapRefusesWithoutEnsureOrForce(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)
	svc := newService(c)
	ctx := context.Background()

	_, err := svc.Bootstrap(ctx, backoffice.BootstrapRequest{
		Email: "first@example.com",
		Name:  "First",
	})
	c.Assert(err, qt.IsNil)

	_, err = svc.Bootstrap(ctx, backoffice.BootstrapRequest{
		Email: "second@example.com",
		Name:  "Second",
	})
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "--force")

	users, err := fs.BackofficeUserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 1)
}

// TestService_BootstrapRejectsForceAndEnsure pins the mutual-exclusion guard:
// Force and Ensure are opposites, so combining them is a fail-closed error and
// must NOT fall through to Force's create-an-extra path (which would defeat
// Ensure's "never creates a second operator" contract).
func TestService_BootstrapRejectsForceAndEnsure(t *testing.T) {
	c := qt.New(t)
	fs := setupMemoryAsPostgres(c)
	svc := newService(c)
	ctx := context.Background()

	_, err := svc.Bootstrap(ctx, backoffice.BootstrapRequest{
		Email:  "first@example.com",
		Name:   "First",
		Force:  true,
		Ensure: true,
	})
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "mutually exclusive")

	// The contradiction is rejected before any write — the table stays empty.
	users, err := fs.BackofficeUserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 0)
}
