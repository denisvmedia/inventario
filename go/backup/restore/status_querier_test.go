package restore_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// seedUserRegistrySet provisions a memory-backed registry set with a single
// tenant and user context suitable for restore-operation CRUD in tests.
func seedUserRegistrySet(c *qt.C) (*registry.Set, context.Context) {
	c.Helper()

	factorySet := memory.NewFactorySet()

	tenant := models.Tenant{
		EntityID: models.EntityID{ID: "test-tenant-id"},
		Name:     "Test Tenant",
	}
	must.Must(factorySet.TenantRegistry.Create(c.Context(), tenant))

	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
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
// registry errors from List are propagated by RegistryStatusQuerier.
type failingRestoreOperationRegistry struct {
	registry.RestoreOperationRegistry
	listErr error
}

func (f *failingRestoreOperationRegistry) List(context.Context) ([]*models.RestoreOperation, error) {
	return nil, f.listErr
}

func TestRegistryStatusQuerier_HasRunningRestores_PropagatesRegistryError(t *testing.T) {
	c := qt.New(t)

	sentinel := errors.New("registry unavailable")
	registrySet := &registry.Set{
		RestoreOperationRegistry: &failingRestoreOperationRegistry{listErr: sentinel},
	}

	querier := restore.NewRegistryStatusQuerier(registrySet)
	got, err := querier.HasRunningRestores(context.Background())
	c.Assert(err, qt.ErrorIs, sentinel)
	c.Assert(got, qt.IsFalse)
}
