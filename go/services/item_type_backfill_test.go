package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// flakyItemTypeRegistry fails the first `failures` backfill calls, then
// succeeds.
type flakyItemTypeRegistry struct {
	registry.ItemTypeRegistry
	failures int
	calls    int
}

func (r *flakyItemTypeRegistry) BackfillBuiltins(context.Context) (int, error) {
	r.calls++
	if r.calls <= r.failures {
		return 0, errors.New("database unavailable")
	}
	return 6, nil
}

type flakyItemTypeFactory struct {
	registry.ItemTypeRegistryFactory
	reg *flakyItemTypeRegistry
}

func (f flakyItemTypeFactory) CreateServiceRegistry() registry.ItemTypeRegistry {
	return f.reg
}

func TestBackfillItemTypes_RetriesUntilAPassSucceeds(t *testing.T) {
	c := qt.New(t)
	reg := &flakyItemTypeRegistry{failures: 2}
	factorySet := &registry.FactorySet{ItemTypeRegistryFactory: flakyItemTypeFactory{reg: reg}}

	services.BackfillItemTypes(context.Background(), factorySet, time.Millisecond)

	c.Assert(reg.calls, qt.Equals, 3)
}

func TestBackfillItemTypes_StopsWhenContextEnds(t *testing.T) {
	c := qt.New(t)
	reg := &flakyItemTypeRegistry{failures: 1 << 30}
	factorySet := &registry.FactorySet{ItemTypeRegistryFactory: flakyItemTypeFactory{reg: reg}}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		services.BackfillItemTypes(ctx, factorySet, time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		c.Fatal("backfill kept retrying after its context ended")
	}
	c.Assert(reg.calls > 0, qt.IsTrue)
}

func TestBackfillItemTypes_NoFactorySetIsANoOp(t *testing.T) {
	services.BackfillItemTypes(context.Background(), nil, time.Millisecond)
	services.BackfillItemTypes(context.Background(), &registry.FactorySet{}, time.Millisecond)
}

func TestBackfillItemTypes_SeedsEveryGroupWithoutTypes(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()
	var groups []*models.LocationGroup
	for _, slug := range []string{"home", "office"} {
		g, err := fs.LocationGroupRegistry.Create(t.Context(), models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
			Name:                slug,
			Slug:                slug,
			Status:              models.LocationGroupStatusActive,
			CreatedBy:           "owner",
			GroupCurrency:       "USD",
		})
		c.Assert(err, qt.IsNil)
		groups = append(groups, g)
	}

	services.BackfillItemTypes(t.Context(), fs, time.Millisecond)
	services.BackfillItemTypes(t.Context(), fs, time.Millisecond)

	user := &models.User{TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "owner"}, TenantID: "tenant-1"}}
	for _, g := range groups {
		ctx := appctx.WithGroup(appctx.WithUser(t.Context(), user), g)
		count, err := fs.ItemTypeRegistryFactory.MustCreateUserRegistry(ctx).Count(ctx)
		c.Assert(err, qt.IsNil)
		c.Check(count, qt.Equals, len(models.BuiltinItemTypes), qt.Commentf("group %s", g.Slug))
	}
}
