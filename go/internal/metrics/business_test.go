package metrics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/denisvmedia/inventario/internal/metrics"
)

func TestBusinessCollector_CollectOnceSetsGauges(t *testing.T) {
	c := qt.New(t)

	stats := metrics.BusinessStats{
		Tenants:        3,
		Users:          12,
		LocationGroups: 4,
		Locations:      9,
		Areas:          25,
		Commodities:    140,
		Files:          77,

		StorageImages:    1000,
		StorageDocuments: 2000,
		StorageOther:     3000,
		StorageExports:   4000,
	}

	collector := metrics.NewBusinessCollector(func(context.Context) (metrics.BusinessStats, error) {
		return stats, nil
	}, time.Hour)

	collector.CollectOnceForTest(context.Background())

	c.Assert(testutil.ToFloat64(metrics.BusinessTenants), qt.Equals, float64(3))
	c.Assert(testutil.ToFloat64(metrics.BusinessUsers), qt.Equals, float64(12))
	c.Assert(testutil.ToFloat64(metrics.BusinessLocationGroups), qt.Equals, float64(4))
	c.Assert(testutil.ToFloat64(metrics.BusinessLocations), qt.Equals, float64(9))
	c.Assert(testutil.ToFloat64(metrics.BusinessAreas), qt.Equals, float64(25))
	c.Assert(testutil.ToFloat64(metrics.BusinessCommodities), qt.Equals, float64(140))
	c.Assert(testutil.ToFloat64(metrics.BusinessFiles), qt.Equals, float64(77))

	c.Assert(testutil.ToFloat64(
		metrics.BusinessFileStorageBytes.WithLabelValues(metrics.StorageCategoryImages)),
		qt.Equals, float64(1000))
	c.Assert(testutil.ToFloat64(
		metrics.BusinessFileStorageBytes.WithLabelValues(metrics.StorageCategoryDocuments)),
		qt.Equals, float64(2000))
	c.Assert(testutil.ToFloat64(
		metrics.BusinessFileStorageBytes.WithLabelValues(metrics.StorageCategoryOther)),
		qt.Equals, float64(3000))
	c.Assert(testutil.ToFloat64(
		metrics.BusinessFileStorageBytes.WithLabelValues(metrics.StorageCategoryExports)),
		qt.Equals, float64(4000))
}

func TestBusinessCollector_ErrorLeavesGaugesUntouched(t *testing.T) {
	c := qt.New(t)

	// Seed a known good value first.
	good := metrics.NewBusinessCollector(func(context.Context) (metrics.BusinessStats, error) {
		return metrics.BusinessStats{Tenants: 42}, nil
	}, time.Hour)
	good.CollectOnceForTest(context.Background())
	c.Assert(testutil.ToFloat64(metrics.BusinessTenants), qt.Equals, float64(42))

	errBefore := testutil.ToFloat64(metrics.BusinessCollectErrorsTotal)

	failing := metrics.NewBusinessCollector(func(context.Context) (metrics.BusinessStats, error) {
		return metrics.BusinessStats{Tenants: 999}, errors.New("db down")
	}, time.Hour)
	failing.CollectOnceForTest(context.Background())

	// Error counter incremented...
	c.Assert(testutil.ToFloat64(metrics.BusinessCollectErrorsTotal)-errBefore, qt.Equals, float64(1))
	// ...and the gauge kept its last good value rather than 999.
	c.Assert(testutil.ToFloat64(metrics.BusinessTenants), qt.Equals, float64(42))
}

func TestBusinessCollector_StartStopDoesNotDeadlock(t *testing.T) {
	c := qt.New(t)

	collector := metrics.NewBusinessCollector(func(context.Context) (metrics.BusinessStats, error) {
		return metrics.BusinessStats{}, nil
	}, time.Hour)

	collector.Start(t.Context())

	done := make(chan struct{})
	go func() {
		collector.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		c.Fatal("BusinessCollector.Stop deadlocked")
	}
}
