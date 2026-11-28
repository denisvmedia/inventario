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

// registeredDatePass is what one BackfillRegisteredDates pass reports.
type registeredDatePass struct {
	dated   int
	pending bool
	err     error
}

// scriptedDateBackfillRegistry reports the passes in order, then an empty
// catalogue.
type scriptedDateBackfillRegistry struct {
	registry.CommodityRegistry
	passes []registeredDatePass
	calls  int
}

func (r *scriptedDateBackfillRegistry) BackfillRegisteredDates(context.Context) (dated int, pending bool, err error) {
	r.calls++
	if r.calls > len(r.passes) {
		return 0, false, nil
	}
	p := r.passes[r.calls-1]
	return p.dated, p.pending, p.err
}

type scriptedDateBackfillFactory struct {
	registry.CommodityRegistryFactory
	reg *scriptedDateBackfillRegistry
}

func (f scriptedDateBackfillFactory) CreateServiceRegistry() registry.CommodityRegistry {
	return f.reg
}

func runRegisteredDateBackfill(c *qt.C, reg *scriptedDateBackfillRegistry) {
	c.Helper()
	factorySet := &registry.FactorySet{CommodityRegistryFactory: scriptedDateBackfillFactory{reg: reg}}
	ctx, cancel := context.WithTimeout(c.Context(), 5*time.Second)
	defer cancel()
	services.BackfillRegisteredDates(ctx, factorySet, time.Millisecond)
	c.Assert(ctx.Err(), qt.IsNil, qt.Commentf("the backfill did not stop"))
}

func TestBackfillRegisteredDates_RetriesWhileLockedRowsRemain(t *testing.T) {
	c := qt.New(t)
	// A currency migration holds the remaining rows locked for two passes.
	reg := &scriptedDateBackfillRegistry{passes: []registeredDatePass{
		{dated: 5, pending: true},
		{dated: 0, pending: true},
		{dated: 2, pending: false},
	}}

	runRegisteredDateBackfill(c, reg)

	c.Assert(reg.calls, qt.Equals, 3)
}

func TestBackfillRegisteredDates_StopsWhenNothingIsLeft(t *testing.T) {
	c := qt.New(t)
	reg := &scriptedDateBackfillRegistry{}

	runRegisteredDateBackfill(c, reg)

	c.Assert(reg.calls, qt.Equals, 1)
}

func TestBackfillRegisteredDates_RetriesAFailedPass(t *testing.T) {
	c := qt.New(t)
	reg := &scriptedDateBackfillRegistry{passes: []registeredDatePass{
		{err: errors.New("database unavailable")},
	}}

	runRegisteredDateBackfill(c, reg)

	c.Assert(reg.calls, qt.Equals, 2)
}
