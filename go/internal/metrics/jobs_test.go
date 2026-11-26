package metrics_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"go.5x5.cz/inventario/internal/metrics"
)

func TestJobDeadLettered_CountsPerType(t *testing.T) {
	c := qt.New(t)
	metrics.InitJobTypes("export-test", "restore-test")
	c.Assert(testutil.ToFloat64(metrics.JobsDeadLetteredTotal.WithLabelValues("export-test")), qt.Equals, float64(0))

	metrics.RecordJobDeadLettered("export-test")
	metrics.RecordJobDeadLettered("export-test")
	c.Assert(testutil.ToFloat64(metrics.JobsDeadLetteredTotal.WithLabelValues("export-test")), qt.Equals, float64(2))
	c.Assert(testutil.ToFloat64(metrics.JobsDeadLetteredTotal.WithLabelValues("restore-test")), qt.Equals, float64(0))
}

func TestJobDepthCollector_SetsGaugePerType(t *testing.T) {
	c := qt.New(t)
	depths := map[string]int{"thumbnail-test": 4, "intake-test": 0}
	collector := metrics.NewJobDepthCollector(func(context.Context) (map[string]int, error) {
		return depths, nil
	}, time.Hour)

	collector.CollectOnce(context.Background())
	c.Assert(testutil.ToFloat64(metrics.JobsDeadLetterDepth.WithLabelValues("thumbnail-test")), qt.Equals, float64(4))
	c.Assert(testutil.ToFloat64(metrics.JobsDeadLetterDepth.WithLabelValues("intake-test")), qt.Equals, float64(0))

	depths["thumbnail-test"] = 1
	collector.CollectOnce(context.Background())
	c.Assert(testutil.ToFloat64(metrics.JobsDeadLetterDepth.WithLabelValues("thumbnail-test")), qt.Equals, float64(1))
}

func TestJobDepthCollector_ErrorKeepsLastValues(t *testing.T) {
	c := qt.New(t)
	fail := false
	collector := metrics.NewJobDepthCollector(func(context.Context) (map[string]int, error) {
		if fail {
			return nil, errors.New("database down")
		}
		return map[string]int{"export-depth-test": 3}, nil
	}, time.Hour)

	collector.CollectOnce(context.Background())
	before := testutil.ToFloat64(metrics.JobsDeadLetterCollectErrorsTotal)
	fail = true
	collector.CollectOnce(context.Background())
	c.Assert(testutil.ToFloat64(metrics.JobsDeadLetterDepth.WithLabelValues("export-depth-test")), qt.Equals, float64(3))
	c.Assert(testutil.ToFloat64(metrics.JobsDeadLetterCollectErrorsTotal), qt.Equals, before+1)
}

// Start collects at once, and Stop is safe to call twice.
func TestJobDepthCollector_StartStop(t *testing.T) {
	called := make(chan struct{}, 1)
	collector := metrics.NewJobDepthCollector(func(context.Context) (map[string]int, error) {
		select {
		case called <- struct{}{}:
		default:
		}
		return map[string]int{}, nil
	}, time.Hour)
	collector.Start(context.Background())
	<-called
	collector.Stop()
	collector.Stop()
}
