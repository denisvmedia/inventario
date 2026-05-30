package bootstrap

import (
	"github.com/denisvmedia/inventario/internal/metrics"
	"github.com/denisvmedia/inventario/registry"
)

// SystemStatsToBusinessStats exposes the unexported adapter copy to the
// black-box bootstrap_test package so the field mapping can be asserted
// without a database or a running collector. It compiles only under
// `go test`.
func SystemStatsToBusinessStats(s registry.SystemStats) metrics.BusinessStats {
	return systemStatsToBusinessStats(s)
}
