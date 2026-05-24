package apiserver

import (
	"net/http"
)

// TestTenantHeaderName is the request header consulted by TestHeaderTenantResolver.
// It is intentionally namespaced with the Inventario prefix so it cannot
// collide with any user-supplied header.
const TestTenantHeaderName = "X-Inventario-Test-Tenant"

// TestHeaderTenantResolver wraps an inner TenantResolver and consults a
// test-only request header before delegating. When the header is present and
// non-empty, its trimmed value is returned as the tenant slug. When the header
// is absent (or trims to empty) the inner resolver runs unchanged.
//
// SECURITY: this resolver bypasses Host-based tenant resolution and MUST NOT
// be installed in production. The bootstrap layer guards it behind the
// explicit INVENTARIO_RUN_TEST_TENANT_HEADER_ENABLED flag and emits a
// warning at startup when enabled.
type TestHeaderTenantResolver struct {
	Inner TenantResolver
}

// ResolveTenant returns the test-header value when set, else delegates to Inner.
func (t *TestHeaderTenantResolver) ResolveTenant(r *http.Request) (string, error) {
	if v := r.Header.Get(TestTenantHeaderName); v != "" {
		return v, nil
	}
	if t.Inner == nil {
		return "", nil
	}
	return t.Inner.ResolveTenant(r)
}
