package metrics_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/denisvmedia/inventario/internal/metrics"
)

func TestQueryTracer_RecordsSuccess(t *testing.T) {
	c := qt.New(t)

	tracer := metrics.NewQueryTracer()

	countBefore := testutil.ToFloat64(metrics.DBQueriesTotal.WithLabelValues("select", "ok"))
	durBefore := histogramCount(c, "select")

	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{
		SQL: "SELECT * FROM commodities",
	})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: nil})

	countAfter := testutil.ToFloat64(metrics.DBQueriesTotal.WithLabelValues("select", "ok"))
	c.Assert(countAfter-countBefore, qt.Equals, float64(1))
	c.Assert(histogramCount(c, "select")-durBefore, qt.Equals, uint64(1))
}

func TestQueryTracer_RecordsError(t *testing.T) {
	c := qt.New(t)

	tracer := metrics.NewQueryTracer()

	countBefore := testutil.ToFloat64(metrics.DBQueriesTotal.WithLabelValues("update", "error"))

	ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{
		SQL: "UPDATE files SET path = $1",
	})
	tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: errors.New("boom")})

	countAfter := testutil.ToFloat64(metrics.DBQueriesTotal.WithLabelValues("update", "error"))
	c.Assert(countAfter-countBefore, qt.Equals, float64(1))
}

func TestQueryTracer_MissingStartContextNoOps(t *testing.T) {
	c := qt.New(t)

	tracer := metrics.NewQueryTracer()
	// Call End without a corresponding Start context: must not panic.
	c.Assert(func() {
		tracer.TraceQueryEnd(context.Background(), nil, pgx.TraceQueryEndData{})
	}, qt.Not(qt.PanicMatches), ".*")
}

// histogramCount returns the cumulative sample count of the
// db_query_duration histogram series for the given operation.
func histogramCount(c *qt.C, operation string) uint64 {
	obs, err := metrics.DBQueryDuration.GetMetricWithLabelValues(operation)
	c.Assert(err, qt.IsNil)
	coll, ok := obs.(prometheus.Collector)
	c.Assert(ok, qt.IsTrue)
	return metrics.HistogramSampleCount(coll)
}
