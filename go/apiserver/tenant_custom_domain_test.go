package apiserver_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// #1036: a tenant can bring its own domain. Such a host carries no slug and
// lies outside the base domain, so slug resolution cannot see it — the domain
// column is consulted first, and a request that arrives on the slug host
// instead is redirected to the domain the tenant configured.

// domainEnv builds a registry with one slug-only tenant and one that also has
// a custom domain, behind PublicTenantMiddleware in multi-tenant mode. The
// returned caller reports the status and the tenant the handler saw.
func domainEnv(c *qt.C, catchAll string) func(host string) (int, string) {
	c.Helper()

	params, _, _ := newParams()
	must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "acme",
		Slug:   "acme",
		Domain: new("inventory.acme.test"),
		Status: models.TenantStatusActive,
	}))
	must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "other-org",
		Slug:   "other-org",
		Status: models.TenantStatusActive,
	}))

	resolver := &apiserver.HostTenantResolver{BaseDomain: "example.test"}
	mw := apiserver.PublicTenantMiddleware(resolver, params.FactorySet.TenantRegistry, catchAll)

	var seen string
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if tenant := apiserver.TenantFromContext(r.Context()); tenant != nil {
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

func TestPublicTenantMiddleware_ResolvesACustomDomain(t *testing.T) {
	c := qt.New(t)
	call := domainEnv(c, "")

	c.Run("the custom domain resolves to its tenant", func(c *qt.C) {
		code, slug := call("inventory.acme.test")
		c.Assert(code, qt.Equals, http.StatusOK)
		c.Assert(slug, qt.Equals, "acme")
	})

	// The same three Host-header variations the slug path already tolerates.
	// The column is matched exactly, so normalizing the host is the whole
	// reason these resolve.
	for _, host := range []string{
		"inventory.acme.test:8443",
		"Inventory.Acme.Test",
		"inventory.acme.test.",
	} {
		c.Run(host, func(c *qt.C) {
			code, slug := call(host)
			c.Assert(code, qt.Equals, http.StatusOK)
			c.Assert(slug, qt.Equals, "acme")
		})
	}

	c.Run("the slug host keeps resolving by slug", func(c *qt.C) {
		code, slug := call("other-org.example.test")
		c.Assert(code, qt.Equals, http.StatusOK)
		c.Assert(slug, qt.Equals, "other-org")
	})

	// A tenant with a custom domain is still reachable by its slug host; the
	// domain is an addition, not a replacement. Moving the browser to the
	// domain is the redirect's job, and the redirect is not on this route.
	c.Run("the tenant's own slug host still resolves", func(c *qt.C) {
		code, slug := call("acme.example.test")
		c.Assert(code, qt.Equals, http.StatusOK)
		c.Assert(slug, qt.Equals, "acme")
	})

	c.Run("a host nobody owns is still refused", func(c *qt.C) {
		code, _ := call("nothing.invalid.test")
		c.Assert(code, qt.Equals, http.StatusServiceUnavailable)
	})
}

// With a catch-all configured, an unknown host still falls back — but a host
// that is a real custom domain must reach its own tenant rather than the
// catch-all, which is the ordering this whole change is about.
func TestPublicTenantMiddleware_CustomDomainBeatsTheCatchAll(t *testing.T) {
	c := qt.New(t)
	call := domainEnv(c, "other-org")

	code, slug := call("inventory.acme.test")
	c.Assert(code, qt.Equals, http.StatusOK)
	c.Assert(slug, qt.Equals, "acme")

	code, slug = call("nothing.invalid.test")
	c.Assert(code, qt.Equals, http.StatusOK)
	c.Assert(slug, qt.Equals, "other-org")
}

// A suspended tenant is refused whichever host it was reached by. Without
// this the custom domain would be a way around the check.
func TestPublicTenantMiddleware_CustomDomainOfASuspendedTenant(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "acme",
		Slug:   "acme",
		Domain: new("inventory.acme.test"),
		Status: models.TenantStatusSuspended,
	}))

	mw := apiserver.PublicTenantMiddleware(
		&apiserver.HostTenantResolver{BaseDomain: "example.test"},
		params.FactorySet.TenantRegistry,
		"",
	)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/anything", nil)
	req.Host = "inventory.acme.test"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
}

// failingDomainRegistry fails GetByDomain and delegates everything else.
type failingDomainRegistry struct {
	registry.TenantRegistry
}

func (f failingDomainRegistry) GetByDomain(context.Context, string) (*models.Tenant, error) {
	return nil, errors.New("connection refused")
}

// A domain lookup that fails is not the same as a host nobody owns. Falling
// through would serve whichever tenant the slug path or the catch-all picks,
// so an outage would become a mis-route — the same reasoning the catch-all
// already applies to GetBySlug.
func TestPublicTenantMiddleware_DomainLookupFailureIsNotAFallThrough(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "fallback-org",
		Slug:   "fallback-org",
		Status: models.TenantStatusActive,
	}))

	mw := apiserver.PublicTenantMiddleware(
		&apiserver.HostTenantResolver{BaseDomain: "example.test"},
		failingDomainRegistry{TenantRegistry: params.FactorySet.TenantRegistry},
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

	c.Assert(rr.Code, qt.Equals, http.StatusServiceUnavailable)
	c.Assert(served, qt.IsFalse, qt.Commentf("a database error served a tenant"))
}

// redirectEnv puts CanonicalDomainRedirect in front of a handler that reports
// it was reached, and returns a caller that reports the status and Location.
func redirectEnv(c *qt.C, baseDomain string) func(host, target string, headers map[string]string) (int, string) {
	c.Helper()

	params, _, _ := newParams()
	must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "acme",
		Slug:   "acme",
		Domain: new("inventory.acme.test"),
		Status: models.TenantStatusActive,
	}))
	must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "plain",
		Slug:   "plain",
		Status: models.TenantStatusActive,
	}))

	mw := apiserver.CanonicalDomainRedirect(
		&apiserver.HostTenantResolver{BaseDomain: baseDomain},
		params.FactorySet.TenantRegistry,
	)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	return func(host, target string, headers map[string]string) (int, string) {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		req.Host = host
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr.Code, rr.Header().Get("Location")
	}
}

func TestCanonicalDomainRedirect(t *testing.T) {
	c := qt.New(t)
	call := redirectEnv(c, "example.test")

	c.Run("the slug host moves to the custom domain, path and query intact", func(c *qt.C) {
		code, location := call("acme.example.test", "/items?page=2&q=drill+bit", nil)
		c.Assert(code, qt.Equals, http.StatusFound)
		c.Assert(location, qt.Equals, "http://inventory.acme.test/items?page=2&q=drill+bit")
	})

	c.Run("X-Forwarded-Proto decides the scheme behind a terminating proxy", func(c *qt.C) {
		code, location := call("acme.example.test", "/", map[string]string{"X-Forwarded-Proto": "https"})
		c.Assert(code, qt.Equals, http.StatusFound)
		c.Assert(location, qt.Equals, "https://inventory.acme.test/")
	})

	c.Run("a proxy chain reports the client-facing hop first", func(c *qt.C) {
		code, location := call("acme.example.test", "/", map[string]string{"X-Forwarded-Proto": "https, http"})
		c.Assert(code, qt.Equals, http.StatusFound)
		c.Assert(location, qt.Equals, "https://inventory.acme.test/")
	})

	// Every one of these has to be served where it is, or the redirect either
	// loops or fires where there is nothing to move to.
	c.Run("the custom domain itself is not redirected", func(c *qt.C) {
		code, location := call("inventory.acme.test", "/items", nil)
		c.Assert(code, qt.Equals, http.StatusOK)
		c.Assert(location, qt.Equals, "")
	})

	c.Run("a tenant with no custom domain is not redirected", func(c *qt.C) {
		code, _ := call("plain.example.test", "/items", nil)
		c.Assert(code, qt.Equals, http.StatusOK)
	})

	c.Run("an unknown slug is left to the handler", func(c *qt.C) {
		code, _ := call("nobody.example.test", "/items", nil)
		c.Assert(code, qt.Equals, http.StatusOK)
	})

	c.Run("the root domain is not redirected", func(c *qt.C) {
		code, _ := call("example.test", "/", nil)
		c.Assert(code, qt.Equals, http.StatusOK)
	})

	c.Run("a host outside the base domain is not redirected", func(c *qt.C) {
		code, _ := call("localhost", "/", nil)
		c.Assert(code, qt.Equals, http.StatusOK)
	})
}

// A tenant whose custom domain IS its slug host is a configuration an
// operator can enter, and it is the one case where the redirect would point at
// the host it came from. Without the guard the browser loops until it gives
// up.
func TestCanonicalDomainRedirect_DomainEqualToTheSlugHostDoesNotLoop(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "acme",
		Slug:   "acme",
		Domain: new("acme.example.test"),
		Status: models.TenantStatusActive,
	}))

	mw := apiserver.CanonicalDomainRedirect(
		&apiserver.HostTenantResolver{BaseDomain: "example.test"},
		params.FactorySet.TenantRegistry,
	)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// The port and the case differ from the stored value, which is exactly
	// how a loop would sneak past a comparison that did not normalize.
	for _, host := range []string{"acme.example.test", "ACME.example.test:8443"} {
		c.Run(host, func(c *qt.C) {
			req := httptest.NewRequest(http.MethodGet, "/items", nil)
			req.Host = host
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			c.Assert(rr.Code, qt.Equals, http.StatusOK)
			c.Assert(rr.Header().Get("Location"), qt.Equals, "")
		})
	}
}

// The redirect target is assembled from a database value and a request path.
// The path is the only request-controlled part, and it must not be able to move
// the redirect to another origin — the failure an open-redirect finding
// describes. These are the shapes that try.
//
// A server-parsed request path always begins with "/", so these hold however
// the URL is assembled. They pin the property rather than one implementation of
// it; the case that separates the two is below.
func TestCanonicalDomainRedirect_KeepsTheHostWhateverThePathSays(t *testing.T) {
	c := qt.New(t)
	call := redirectEnv(c, "example.test")

	for _, target := range []string{
		"//evil.test/",
		"//evil.test/items?x=1",
		"/items/..%2f..%2fevil.test",
		"/@evil.test",
		"/items?next=https://evil.test",
		"/items#//evil.test",
	} {
		c.Run(target, func(c *qt.C) {
			code, location := call("acme.example.test", target, nil)
			c.Assert(code, qt.Equals, http.StatusFound)

			redirect, err := url.Parse(location)
			c.Assert(err, qt.IsNil)
			c.Assert(redirect.Host, qt.Equals, "inventory.acme.test",
				qt.Commentf("location was %q", location))
		})
	}
}

// hostileDomainRegistry hands back a tenant whose stored domain is not a bare
// hostname. Validation refuses to write one, so this is the shape of a row that
// predates the rule or was written by hand.
type hostileDomainRegistry struct {
	registry.TenantRegistry
	domain string
}

func (h hostileDomainRegistry) GetBySlug(context.Context, string) (*models.Tenant, error) {
	return &models.Tenant{
		Name:   "acme",
		Slug:   "acme",
		Domain: new(h.domain),
		Status: models.TenantStatusActive,
	}, nil
}

// This is the case that makes the assembly matter. normalizeHost lowercases and
// strips a port and a trailing dot; it does not remove a slash, so a stored
// "evil.test/x" concatenated into a URL string is a redirect to evil.test.
// Going through url.URL escapes it into the host component instead, and the
// browser is never sent to another origin.
func TestCanonicalDomainRedirect_ARowThatIsNotAHostnameCannotRedirectAway(t *testing.T) {
	c := qt.New(t)

	for _, domain := range []string{
		"evil.test/x",
		"evil.test/x?y=1",
		"evil.test\\x",
	} {
		c.Run(domain, func(c *qt.C) {
			params, _, _ := newParams()
			mw := apiserver.CanonicalDomainRedirect(
				&apiserver.HostTenantResolver{BaseDomain: "example.test"},
				hostileDomainRegistry{TenantRegistry: params.FactorySet.TenantRegistry, domain: domain},
			)
			handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/items", nil)
			req.Host = "acme.example.test"
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			if rr.Code == http.StatusOK {
				return // not redirecting at all is also a correct answer
			}
			location := rr.Header().Get("Location")
			redirect, err := url.Parse(location)
			c.Assert(err, qt.IsNil)
			c.Assert(redirect.Host, qt.Not(qt.Equals), "evil.test",
				qt.Commentf("location was %q", location))
		})
	}
}

// countingTenantRegistry records how many slug lookups reached it.
type countingTenantRegistry struct {
	registry.TenantRegistry
	slugLookups int
}

func (t *countingTenantRegistry) GetBySlug(ctx context.Context, slug string) (*models.Tenant, error) {
	t.slugLookups++
	return t.TenantRegistry.GetBySlug(ctx, slug)
}

// Single-tenant mode resolves every host to the one default tenant, so there
// is no slug host to canonicalize away from. Redirecting there would send a
// developer on localhost to production.
//
// Asserting on the lookup count rather than only on the status is what makes
// this test discriminating: an empty slug happens to resolve to nothing in
// every registry, so a version that asked anyway would still not redirect —
// and would pay a query on every document request.
func TestCanonicalDomainRedirect_SingleTenantModeDoesNotEvenLookUp(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "acme",
		Slug:   "acme",
		Domain: new("inventory.acme.test"),
		Status: models.TenantStatusActive,
	}))
	counting := &countingTenantRegistry{TenantRegistry: params.FactorySet.TenantRegistry}

	// BaseDomain empty: single-tenant mode, every host resolves to an empty
	// slug.
	mw := apiserver.CanonicalDomainRedirect(&apiserver.HostTenantResolver{}, counting)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, host := range []string{"localhost", "acme.example.test", "inventory.acme.test"} {
		c.Run(host, func(c *qt.C) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Host = host
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			c.Assert(rr.Code, qt.Equals, http.StatusOK)
			c.Assert(rr.Header().Get("Location"), qt.Equals, "")
		})
	}

	c.Assert(counting.slugLookups, qt.Equals, 0)
}
