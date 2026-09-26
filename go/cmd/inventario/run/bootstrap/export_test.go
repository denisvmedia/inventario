package bootstrap

import (
	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/registry"
)

// SystemStatsToBusinessStats exposes the unexported adapter copy to the
// black-box bootstrap_test package so the field mapping can be asserted
// without a database or a running collector. It compiles only under
// `go test`.
func SystemStatsToBusinessStats(s registry.SystemStats) metrics.BusinessStats {
	return systemStatsToBusinessStats(s)
}

// WireTenantBaseDomain exposes the unexported resolver wiring to the black-box
// bootstrap_test package. It compiles only under `go test`.
func WireTenantBaseDomain(cfg *Config, params *apiserver.Params) {
	wireTenantBaseDomain(cfg, params)
}
