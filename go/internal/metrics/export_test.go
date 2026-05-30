package metrics

// This file exposes selected unexported metric vars and helpers to the
// black-box metrics_test package so tests can assert on them without
// widening the public API. It compiles only under `go test`.

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// HTTP metrics.
var (
	HTTPRequestsTotal    = httpRequestsTotal
	HTTPRequestDuration  = httpRequestDuration
	HTTPRequestsInFlight = httpRequestsInFlight
)

// DB query metrics.
var (
	DBQueryDuration = dbQueryDuration
	DBQueriesTotal  = dbQueriesTotal
)

// Business gauges.
var (
	BusinessTenants            = businessTenants
	BusinessUsers              = businessUsers
	BusinessLocationGroups     = businessLocationGroups
	BusinessLocations          = businessLocations
	BusinessAreas              = businessAreas
	BusinessCommodities        = businessCommodities
	BusinessFiles              = businessFiles
	BusinessFileStorageBytes   = businessFileStorageBytes
	BusinessCollectErrorsTotal = businessCollectErrorsTotal
)

// ParseSQLVerb exposes parseSQLVerb for table tests.
func ParseSQLVerb(sql string) string {
	return parseSQLVerb(sql)
}

// StatusClass exposes statusClass for tests.
func StatusClass(code int) string {
	return statusClass(code)
}

// NormalizeMethod exposes normalizeMethod for tests.
func NormalizeMethod(m string) string {
	return normalizeMethod(m)
}

// CollectOnceForTest drives a single business collection sweep
// synchronously, bypassing the goroutine, so tests can assert on the
// gauge side effects deterministically.
func (c *BusinessCollector) CollectOnceForTest(ctx context.Context) {
	c.collectOnce(ctx)
}

// Storage category label values, re-exported for assertion convenience.
const (
	StorageCategoryImages    = storageCategoryImages
	StorageCategoryDocuments = storageCategoryDocuments
	StorageCategoryOther     = storageCategoryOther
	StorageCategoryExports   = storageCategoryExports
)

// HistogramSampleCount returns the cumulative sample count of a
// single-series histogram (or a HistogramVec series) by collecting it
// directly, without a registry. Tests use it to assert that an
// observation landed.
func HistogramSampleCount(c prometheus.Collector) uint64 {
	ch := make(chan prometheus.Metric, 8)
	c.Collect(ch)
	close(ch)

	var total uint64
	for m := range ch {
		var pb dto.Metric
		if err := m.Write(&pb); err != nil {
			continue
		}
		if pb.Histogram != nil {
			total += pb.Histogram.GetSampleCount()
		}
	}
	return total
}
