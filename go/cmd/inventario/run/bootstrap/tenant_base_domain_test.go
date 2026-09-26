package bootstrap_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/cmd/inventario/run/bootstrap"
)

// #1036: subdomain-per-tenant resolution is opt-in. Left unset the server stays
// in single-tenant mode, which is what every self-hosted install behind one
// hostname wants — and what the redirect to a tenant's custom domain must not
// fire in, since that host is somebody's localhost.
func TestWireTenantBaseDomain(t *testing.T) {
	c := qt.New(t)

	c.Run("unset leaves the resolver alone", func(c *qt.C) {
		var params apiserver.Params
		bootstrap.WireTenantBaseDomain(&bootstrap.Config{}, &params)
		c.Assert(params.TenantResolver, qt.IsNil,
			qt.Commentf("a nil resolver is what selects single-tenant mode"))
	})

	c.Run("whitespace only is still unset", func(c *qt.C) {
		var params apiserver.Params
		bootstrap.WireTenantBaseDomain(&bootstrap.Config{TenantBaseDomain: "   "}, &params)
		c.Assert(params.TenantResolver, qt.IsNil)
	})

	// The resolver compares a lowercased, port-stripped host against this
	// value, so a base domain that is not lowercase would match nothing.
	for _, tc := range []struct{ in, want string }{
		{in: "inventario.com", want: "inventario.com"},
		{in: "  Inventario.COM  ", want: "inventario.com"},
	} {
		c.Run(tc.in, func(c *qt.C) {
			var params apiserver.Params
			bootstrap.WireTenantBaseDomain(&bootstrap.Config{TenantBaseDomain: tc.in}, &params)
			resolver, ok := params.TenantResolver.(*apiserver.HostTenantResolver)
			c.Assert(ok, qt.IsTrue, qt.Commentf("got %T", params.TenantResolver))
			c.Assert(resolver.BaseDomain, qt.Equals, tc.want)
		})
	}
}
