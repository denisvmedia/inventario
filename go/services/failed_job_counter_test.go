package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/internal/aivision"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// deadLettered returns how many jobs of the type were counted dead-lettered
// since it was called.
func deadLettered(jobType models.WorkerType) func() float64 {
	counter := metrics.JobsDeadLetteredFor(string(jobType))
	before := testutil.ToFloat64(counter)
	return func() float64 { return testutil.ToFloat64(counter) - before }
}

// A scan that uses up its attempts is counted once, when the worker fails
// it; the retries before it are not counted (#1307).
func TestIntakeWorker_FailedScanIsCountedOnce(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	session := f.create(c, "counted", source(f.photo(c, "kettle")))
	f.provider.failNext(aivision.ErrProviderUnavailable, aivision.ErrProviderUnavailable, aivision.ErrProviderUnavailable)
	counted := deadLettered(models.WorkerTypeIntake)

	c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	c.Assert(counted(), qt.Equals, float64(0), qt.Commentf("a retry is not a failure"))
	for range 2 {
		f.ahead += 10 * time.Minute
		c.Assert(f.worker.Tick(context.Background()), qt.Equals, 1)
	}
	c.Assert(f.get(c, f.ctx, session.Session.ID).Sources[0].Source.Status, qt.Equals, models.IntakeSourceFailed)
	c.Assert(counted(), qt.Equals, float64(1))

	f.ahead += time.Hour
	f.worker.Tick(context.Background())
	c.Assert(counted(), qt.Equals, float64(1), qt.Commentf("a failed source is not counted again"))
}

// A scan whose worker died on its last attempt is failed by the sweep and
// counted once (#1307).
func TestIntakeWorker_SweptScanIsCountedOnce(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	session := f.create(c, "swept", source(f.photo(c, "vase")))
	reg := f.factorySet.IntakeRegistryFactory.CreateServiceRegistry()
	ctx := context.Background()

	// Three workers claim the source and die; the first two claims are
	// swept back into the queue without failing it.
	for i := range 3 {
		now := time.Now()
		_, err := reg.ClaimNextSource(ctx, now)
		c.Assert(err, qt.IsNil, qt.Commentf("claim %d", i+1))
		if i < 2 {
			_, err = reg.SweepStuckSources(ctx, now.Add(time.Second), now, 0, "")
			c.Assert(err, qt.IsNil)
		}
	}
	counted := deadLettered(models.WorkerTypeIntake)

	f.ahead = time.Hour
	f.worker.Tick(ctx)
	src := f.get(c, f.ctx, session.Session.ID).Sources[0].Source
	c.Assert(src.Status, qt.Equals, models.IntakeSourceFailed)
	c.Assert(src.ErrorCode, qt.Equals, services.IntakeErrorProviderError)
	c.Assert(counted(), qt.Equals, float64(1))

	f.worker.Tick(ctx)
	c.Assert(counted(), qt.Equals, float64(1), qt.Commentf("a failed source is not swept again"))
}

// Each migration the recovery sweep fails is counted once (#1307).
func TestCurrencyMigrationWorker_SweptMigrationIsCountedOnce(t *testing.T) {
	c := qt.New(t)
	stuck := &models.CurrencyMigration{
		Status:       models.CurrencyMigrationStatusFailed,
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.RequireFromString("0.9"),
		ErrorMessage: "worker crashed or stalled",
	}
	stuck.SetTenantID("tenant-1")
	stuck.SetGroupID("group-1")
	stuck.SetCreatedByUserID("user-1")
	stuck.ID = "stuck-counted"
	reg := &fakeMigrationRegistry{sweepReturn: []*models.CurrencyMigration{stuck}}
	proc := &fakeProcessor{}
	counted := deadLettered(models.WorkerTypeCurrencyMigration)

	worker := services.NewCurrencyMigrationWorker(reg, proc,
		services.WithCurrencyMigrationActiveInterval(10*time.Millisecond),
		services.WithCurrencyMigrationIdleInterval(10*time.Millisecond),
	)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	worker.Start(ctx)
	c.Assert(eventually(c, time.Second, func() bool { return reg.sweepCalls.Load() >= 3 }), qt.IsTrue)
	worker.Stop()

	c.Assert(proc.sweptIDs(), qt.DeepEquals, []string{"stuck-counted"})
	c.Assert(counted(), qt.Equals, float64(1))
}
