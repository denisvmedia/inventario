package apiserver_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// #1035: a Host that resolves to no tenant can be answered with a configured
// catch-all instead of an error. Off by default, because with it on every host
// that reaches the server is served that one tenant — which is the point for a
// single-tenant install behind an arbitrary hostname, and wrong for a
// deployment that separates tenants by domain.

// catchAllEnv builds a two-tenant registry behind PublicTenantMiddleware in
// multi-tenant mode, and returns a caller that reports the status and the
// tenant the handler saw.
func catchAllEnv(c *qt.C, catchAll string) func(host string) (int, string) {
	c.Helper()

	params, _, _ := newParams()
	// newParams already made "test-org" the default tenant; add the one the
	// catch-all names and a second real tenant to prove the fallback does not
	// simply serve whichever tenant is first.
	for _, slug := range []string{"fallback-org", "other-org"} {
		must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
			Name:   slug,
			Slug:   slug,
			Status: models.TenantStatusActive,
		}))
	}

	resolver := &apiserver.HostTenantResolver{BaseDomain: "example.test"}
	mw := apiserver.PublicTenantMiddleware(resolver, params.FactorySet.TenantRegistry, catchAll)

	var seen string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tenant := apiserver.TenantFromContext(r.Context())
		if tenant != nil {
			seen = tenant.Slug
		}
		w.WriteHeader(http.StatusOK)
	}))

	return func(host string) (int, string) {
		seen = ""
		req := httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil)
		req.Host = host
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code, seen
	}
}

func TestPublicTenantMiddleware_CatchAllDisabled(t *testing.T) {
	c := qt.New(t)
	call := catchAllEnv(c, "")

	c.Run("a known host resolves to its own tenant", func(c *qt.C) {
		code, slug := call("other-org.example.test")
		c.Assert(code, qt.Equals, http.StatusOK)
		c.Assert(slug, qt.Equals, "other-org")
	})

	c.Run("a subdomain with no tenant is not found", func(c *qt.C) {
		code, _ := call("nobody.example.test")
		c.Assert(code, qt.Equals, http.StatusNotFound)
	})

	c.Run("a host outside the base domain is refused", func(c *qt.C) {
		code, _ := call("localhost")
		c.Assert(code, qt.Equals, http.StatusServiceUnavailable)
	})
}

func TestPublicTenantMiddleware_CatchAllEnabled(t *testing.T) {
	c := qt.New(t)
	call := catchAllEnv(c, "fallback-org")

	// The fallback must not shadow resolution that works.
	c.Run("a known host still resolves to its own tenant", func(c *qt.C) {
		code, slug := call("other-org.example.test")
		c.Assert(code, qt.Equals, http.StatusOK)
		c.Assert(slug, qt.Equals, "other-org")
	})

	c.Run("a subdomain with no tenant falls back", func(c *qt.C) {
		code, slug := call("nobody.example.test")
		c.Assert(code, qt.Equals, http.StatusOK)
		c.Assert(slug, qt.Equals, "fallback-org")
	})

	// The case the issue is about: local development against localhost or an
	// /etc/hosts name, which is not a subdomain of anything.
	c.Run("a host outside the base domain falls back", func(c *qt.C) {
		code, slug := call("localhost")
		c.Assert(code, qt.Equals, http.StatusOK)
		c.Assert(slug, qt.Equals, "fallback-org")
	})

	c.Run("the root domain still uses the default tenant", func(c *qt.C) {
		code, slug := call("example.test")
		c.Assert(code, qt.Equals, http.StatusOK)
		c.Assert(slug, qt.Equals, "test-org", qt.Commentf("GetDefault, not the catch-all"))
	})
}

// A catch-all naming a tenant that does not exist must not turn every request
// into a 200 against nothing: it answers not-found, the same as before.
func TestPublicTenantMiddleware_CatchAllNamesNoTenant(t *testing.T) {
	c := qt.New(t)
	call := catchAllEnv(c, "no-such-tenant")

	code, _ := call("nobody.example.test")
	c.Assert(code, qt.Equals, http.StatusNotFound)

	code, _ = call("localhost")
	c.Assert(code, qt.Equals, http.StatusNotFound)
}

// Host headers legally carry a port, mixed case and a trailing dot, and none
// of them changes which host is meant.
func TestHostTenantResolver_NormalizesTheHost(t *testing.T) {
	c := qt.New(t)
	resolver := &apiserver.HostTenantResolver{BaseDomain: "example.test"}

	for _, host := range []string{
		"acme.example.test",
		"acme.example.test:8080",
		"ACME.Example.Test",
		"acme.example.test.",
		"ACME.EXAMPLE.TEST.:443",
	} {
		c.Run(host, func(c *qt.C) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = host
			slug, err := resolver.ResolveTenant(req)
			c.Assert(err, qt.IsNil)
			c.Assert(slug, qt.Equals, "acme")
		})
	}
}

// failingTenantRegistry answers GetBySlug with a database error for one slug
// and delegates the rest, by embedding the interface and overriding the one
// method the middleware calls.
//
// Failing only the requested slug is what makes the test discriminating: a
// registry that failed every lookup would answer not-found either way, and
// could not tell "fell back" from "did not".
type failingTenantRegistry struct {
	registry.TenantRegistry
	failFor string
}

func (f failingTenantRegistry) GetBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	if slug == f.failFor {
		return nil, errors.New("connection refused")
	}
	return f.TenantRegistry.GetBySlug(ctx, slug)
}

// A database that is down must not be answered with the catch-all tenant. The
// two cases look the same from the call site and are not the same thing: one
// says this host has no tenant, the other says we do not know, and serving
// somebody else's tenant to the second turns an outage into a mis-route.
func TestPublicTenantMiddleware_CatchAllDoesNotSwallowADatabaseError(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	// The catch-all resolves fine; only the host's own tenant lookup fails.
	must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "fallback-org",
		Slug:   "fallback-org",
		Status: models.TenantStatusActive,
	}))
	resolver := &apiserver.HostTenantResolver{BaseDomain: "example.test"}
	mw := apiserver.PublicTenantMiddleware(
		resolver,
		failingTenantRegistry{TenantRegistry: params.FactorySet.TenantRegistry, failFor: "acme"},
		"fallback-org",
	)

	served := false
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		served = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil)
	req.Host = "acme.example.test"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	c.Assert(served, qt.IsFalse, qt.Commentf("no handler may run without a resolved tenant"))
}
