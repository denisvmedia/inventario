package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// flakyBackfillRegistry fails the first `failures` calls, then succeeds.
type flakyBackfillRegistry struct {
	registry.CommodityRegistry
	failures int
	calls    int
}

func (r *flakyBackfillRegistry) BackfillAssetNumbers(context.Context) (int, error) {
	r.calls++
	if r.calls <= r.failures {
		return 0, errors.New("database unavailable")
	}
	return 3, nil
}

type flakyBackfillFactory struct {
	registry.CommodityRegistryFactory
	reg *flakyBackfillRegistry
}

func (f flakyBackfillFactory) CreateServiceRegistry() registry.CommodityRegistry {
	return f.reg
}

func TestBackfillAssetNumbers_RetriesUntilAPassSucceeds(t *testing.T) {
	c := qt.New(t)
	reg := &flakyBackfillRegistry{failures: 2}
	factorySet := &registry.FactorySet{CommodityRegistryFactory: flakyBackfillFactory{reg: reg}}

	services.BackfillAssetNumbers(context.Background(), factorySet, time.Millisecond)

	c.Assert(reg.calls, qt.Equals, 3)
}

func TestBackfillAssetNumbers_StopsWhenContextEnds(t *testing.T) {
	c := qt.New(t)
	reg := &flakyBackfillRegistry{failures: 1 << 30}
	factorySet := &registry.FactorySet{CommodityRegistryFactory: flakyBackfillFactory{reg: reg}}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	done := make(chan struct{})
	go func() {
		services.BackfillAssetNumbers(ctx, factorySet, time.Millisecond)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		c.Fatal("backfill kept retrying after its context ended")
	}
	c.Assert(reg.calls > 0, qt.IsTrue)
}

func TestBackfillAssetNumbers_NoFactorySetIsANoOp(t *testing.T) {
	services.BackfillAssetNumbers(context.Background(), nil, time.Millisecond)
	services.BackfillAssetNumbers(context.Background(), &registry.FactorySet{}, time.Millisecond)
}
