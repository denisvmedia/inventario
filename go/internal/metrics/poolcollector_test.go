package metrics_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/denisvmedia/inventario/internal/metrics"
)

// stubProvider satisfies PoolStatProvider.
//
// NOTE on the test limitation: pgxpool.Stat has only unexported
// fields, no public constructor, and wraps a *puddle.Stat that is nil
// in a zero-value Stat. Calling any accessor (e.g. AcquiredConns) on a
// fabricated &pgxpool.Stat{} panics with a nil-pointer dereference, so
// a unit test cannot exercise PoolCollector.Collect at all. We
// therefore cover Describe (descriptor count), the registration
// duplicate-tolerance contract, and a compile-time interface
// assertion; the Collect path that reads real numbers is left to
// integration tests running against an actual *pgxpool.Pool.
type stubProvider struct{}

func (stubProvider) Stat() *pgxpool.Stat { return &pgxpool.Stat{} }

func TestPoolCollector_Describe(t *testing.T) {
	c := qt.New(t)

	coll := metrics.NewPoolCollector(stubProvider{})

	ch := make(chan *prometheus.Desc, 16)
	coll.Describe(ch)
	close(ch)

	var descs []*prometheus.Desc
	for d := range ch {
		descs = append(descs, d)
	}

	// connections, max_connections, acquire_total, empty_acquire_total,
	// canceled_acquire_total = 5 distinct descriptors.
	c.Assert(descs, qt.HasLen, 5)
}

func TestRegisterPoolCollector_ToleratesDuplicate(t *testing.T) {
	c := qt.New(t)

	unregister1 := metrics.RegisterPoolCollector(stubProvider{})
	c.Assert(unregister1, qt.IsNotNil)
	defer unregister1()

	// A second identical collector must not panic; it returns a
	// no-op unregister.
	unregister2 := metrics.RegisterPoolCollector(stubProvider{})
	c.Assert(unregister2, qt.IsNotNil)
	defer unregister2()
}
