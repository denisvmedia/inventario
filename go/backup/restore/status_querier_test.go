package restore_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/backup/restore"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

// seedUserRegistrySet provisions a memory-backed registry set with a single
// tenant and user context suitable for restore-operation CRUD in tests.
func seedUserRegistrySet(c *qt.C) (*registry.Set, context.Context) {
	c.Helper()

	factorySet := memory.NewFactorySet()

	// Create generates the id server-side and ignores the one supplied, so the
	// user has to be built from what came back. Hard-coding both sides left a
	// user pointing at a tenant that does not exist, which only went unnoticed
	// because the memory registries do not enforce the reference the way
	// Postgres does (#1314).
	tenant := must.Must(factorySet.TenantRegistry.Create(c.Context(), models.Tenant{
		Name: "Test Tenant",
	}))

	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
			EntityID: models.EntityID{ID: "test-user-id"},
		},
		Name:  "Test User",
		Email: "querier@example.com",
	}
	createdUser, err := factorySet.UserRegistry.Create(c.Context(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(c.Context(), createdUser)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))
	return registrySet, ctx
}

func seedRestoreOps(c *qt.C, registrySet *registry.Set, ctx context.Context, statuses ...models.RestoreStatus) {
	c.Helper()
	for _, status := range statuses {
		op := models.RestoreOperation{
			ExportID:    "test-export-id",
			Description: "seeded " + string(status),
			Status:      status,
			Options: models.RestoreOptions{
				Strategy:        "merge_update",
				IncludeFileData: false,
				DryRun:          false,
			},
			CreatedDate: models.PNow(),
		}
		_, err := registrySet.RestoreOperationRegistry.Create(ctx, op)
		c.Assert(err, qt.IsNil)
	}
}

func TestRegistryStatusQuerier_HasRunningRestores_HappyPath(t *testing.T) {
	cases := []struct {
		name     string
		seed     []models.RestoreStatus
		expected bool
	}{
		{name: "empty registry reports false", seed: nil, expected: false},
		{name: "only completed reports false", seed: []models.RestoreStatus{models.RestoreStatusCompleted}, expected: false},
		{name: "only failed reports false", seed: []models.RestoreStatus{models.RestoreStatusFailed}, expected: false},
		{name: "mixed terminal reports false", seed: []models.RestoreStatus{models.RestoreStatusCompleted, models.RestoreStatusFailed}, expected: false},
		{name: "single pending reports true", seed: []models.RestoreStatus{models.RestoreStatusPending}, expected: true},
		{name: "single running reports true", seed: []models.RestoreStatus{models.RestoreStatusRunning}, expected: true},
		{name: "pending with completed reports true", seed: []models.RestoreStatus{models.RestoreStatusCompleted, models.RestoreStatusPending}, expected: true},
		{name: "running with failed reports true", seed: []models.RestoreStatus{models.RestoreStatusFailed, models.RestoreStatusRunning}, expected: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			registrySet, ctx := seedUserRegistrySet(c)
			seedRestoreOps(c, registrySet, ctx, tc.seed...)

			querier := restore.NewRegistryStatusQuerier(registrySet)
			got, err := querier.HasRunningRestores(ctx)
			c.Assert(err, qt.IsNil)
			c.Assert(got, qt.Equals, tc.expected)
		})
	}
}

// failingRestoreOperationRegistry is a minimal stub used to verify that
// registry errors are propagated by RegistryStatusQuerier.
type failingRestoreOperationRegistry struct {
	registry.RestoreOperationRegistry
	queryErr error
}

func (f *failingRestoreOperationRegistry) HasActive(context.Context) (bool, error) {
	return false, f.queryErr
}

func TestRegistryStatusQuerier_HasRunningRestores_PropagatesRegistryError(t *testing.T) {
	c := qt.New(t)

	sentinel := errors.New("registry unavailable")
	registrySet := &registry.Set{
		RestoreOperationRegistry: &failingRestoreOperationRegistry{queryErr: sentinel},
	}

	querier := restore.NewRegistryStatusQuerier(registrySet)
	got, err := querier.HasRunningRestores(context.Background())
	c.Assert(err, qt.ErrorIs, sentinel)
	c.Assert(got, qt.IsFalse)
}

// #1314: the querier used to read every restore operation — and in the
// Postgres implementation every operation's steps — to learn one bit. It now
// asks the registry directly, so this pins that it does not go back to List.
func TestRegistryStatusQuerier_DoesNotListEveryOperation(t *testing.T) {
	c := qt.New(t)

	spy := &countingRestoreOperationRegistry{}
	querier := restore.NewRegistryStatusQuerier(&registry.Set{RestoreOperationRegistry: spy})

	got, err := querier.HasRunningRestores(context.Background())
	c.Assert(err, qt.IsNil)
	c.Check(got, qt.IsTrue)
	c.Check(spy.hasActiveCalls, qt.Equals, 1)
	c.Check(spy.listCalls, qt.Equals, 0,
		qt.Commentf("List reads every operation and, on Postgres, its steps"))
}

type countingRestoreOperationRegistry struct {
	registry.RestoreOperationRegistry
	hasActiveCalls int
	listCalls      int
}

func (s *countingRestoreOperationRegistry) HasActive(context.Context) (bool, error) {
	s.hasActiveCalls++
	return true, nil
}

func (s *countingRestoreOperationRegistry) List(context.Context) ([]*models.RestoreOperation, error) {
	s.listCalls++
	return nil, nil
}

// NoopStatusQuerier is what a caller passes when it does not want the
// one-restore-at-a-time guard; it must never claim something is running.
func TestNoopStatusQuerier(t *testing.T) {
	c := qt.New(t)

	got, err := restore.NoopStatusQuerier{}.HasRunningRestores(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsFalse)
}
