package apiserver_test

import (
	"context"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
)

func TestJWTTenantResolver_ResolveTenant(t *testing.T) {
	// Happy path tests
	t.Run("resolve tenant from authenticated user", func(t *testing.T) {
		c := qt.New(t)
		resolver := &apiserver.JWTTenantResolver{}

		// Create user with tenant ID
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-123"},
				TenantID: "tenant-123",
			},
		}

		// Add user to context
		ctx := appctx.WithUser(context.Background(), user)
		req := httptest.NewRequest("GET", "/", nil)
		req = req.WithContext(ctx)

		tenantID, err := resolver.ResolveTenant(req)
		c.Assert(err, qt.IsNil)
		c.Assert(tenantID, qt.Equals, "tenant-123")
	})

	t.Run("fallback to default tenant", func(t *testing.T) {
		c := qt.New(t)
		resolver := &apiserver.JWTTenantResolver{}

		// Create user without tenant ID
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "user-123"},
			},
		}

		// Add user to context
		ctx := appctx.WithUser(context.Background(), user)
		req := httptest.NewRequest("GET", "/", nil)
		req = req.WithContext(ctx)

		tenantID, err := resolver.ResolveTenant(req)
		c.Assert(err, qt.IsNil)
		c.Assert(tenantID, qt.Equals, "default-tenant")
	})

	// Unhappy path tests
	t.Run("no authenticated user", func(t *testing.T) {
		c := qt.New(t)
		resolver := &apiserver.JWTTenantResolver{}

		req := httptest.NewRequest("GET", "/", nil)

		tenantID, err := resolver.ResolveTenant(req)
		c.Assert(err, qt.IsNotNil)
		c.Assert(tenantID, qt.Equals, "")
		c.Assert(err, qt.Equals, apiserver.ErrTenantNotFound)
	})
}

func TestSubdomainTenantResolver_ResolveTenant(t *testing.T) {
	// Happy path tests
	t.Run("resolve tenant from subdomain with base domain", func(t *testing.T) {
		c := qt.New(t)
		resolver := &apiserver.SubdomainTenantResolver{
			BaseDomain: "inventario.com",
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "tenant1.inventario.com"

		tenantID, err := resolver.ResolveTenant(req)
		c.Assert(err, qt.IsNil)
		c.Assert(tenantID, qt.Equals, "tenant1")
	})

	t.Run("resolve tenant from subdomain with port", func(t *testing.T) {
		c := qt.New(t)
		resolver := &apiserver.SubdomainTenantResolver{
			BaseDomain: "inventario.com",
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "tenant1.inventario.com:8080"

		tenantID, err := resolver.ResolveTenant(req)
		c.Assert(err, qt.IsNil)
		c.Assert(tenantID, qt.Equals, "tenant1")
	})

	t.Run("resolve tenant from custom domain without base domain", func(t *testing.T) {
		c := qt.New(t)
		resolver := &apiserver.SubdomainTenantResolver{
			BaseDomain: "",
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "custom-tenant.com"

		tenantID, err := resolver.ResolveTenant(req)
		c.Assert(err, qt.IsNil)
		c.Assert(tenantID, qt.Equals, "custom-tenant.com")
	})

	// Unhappy path tests
	t.Run("missing host header", func(t *testing.T) {
		c := qt.New(t)
		resolver := &apiserver.SubdomainTenantResolver{
			BaseDomain: "inventario.com",
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.Host = ""

		tenantID, err := resolver.ResolveTenant(req)
		c.Assert(err, qt.IsNotNil)
		c.Assert(tenantID, qt.Equals, "")
		c.Assert(err, qt.Equals, apiserver.ErrTenantNotFound)
	})

	t.Run("www subdomain", func(t *testing.T) {
		c := qt.New(t)
		resolver := &apiserver.SubdomainTenantResolver{
			BaseDomain: "inventario.com",
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "www.inventario.com"

		tenantID, err := resolver.ResolveTenant(req)
		c.Assert(err, qt.IsNotNil)
		c.Assert(tenantID, qt.Equals, "")
		c.Assert(err, qt.Equals, apiserver.ErrTenantNotFound)
	})

	t.Run("base domain without subdomain", func(t *testing.T) {
		c := qt.New(t)
		resolver := &apiserver.SubdomainTenantResolver{
			BaseDomain: "inventario.com",
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "inventario.com"

		tenantID, err := resolver.ResolveTenant(req)
		c.Assert(err, qt.IsNotNil)
		c.Assert(tenantID, qt.Equals, "")
		c.Assert(err, qt.Equals, apiserver.ErrTenantNotFound)
	})

	t.Run("different domain", func(t *testing.T) {
		c := qt.New(t)
		resolver := &apiserver.SubdomainTenantResolver{
			BaseDomain: "inventario.com",
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.Host = "example.com"

		tenantID, err := resolver.ResolveTenant(req)
		c.Assert(err, qt.IsNotNil)
		c.Assert(tenantID, qt.Equals, "")
		c.Assert(err, qt.Equals, apiserver.ErrTenantNotFound)
	})
}

func TestTenantFromContext(t *testing.T) {
	// Happy path tests
	t.Run("retrieve tenant from context", func(t *testing.T) {
		c := qt.New(t)
		tenant := &models.Tenant{
			EntityID: models.EntityID{ID: "tenant-123"},
			Name:     "Test Tenant",
			Slug:     "test-tenant",
			Status:   models.TenantStatusActive,
		}

		ctx := apiserver.WithTenant(context.Background(), tenant)
		retrievedTenant := apiserver.TenantFromContext(ctx)

		c.Assert(retrievedTenant, qt.IsNotNil)
		c.Assert(retrievedTenant.ID, qt.Equals, "tenant-123")
		c.Assert(retrievedTenant.Name, qt.Equals, "Test Tenant")
	})

	// Unhappy path tests
	t.Run("no tenant in context", func(t *testing.T) {
		c := qt.New(t)
		retrievedTenant := apiserver.TenantFromContext(context.Background())
		c.Assert(retrievedTenant, qt.IsNil)
	})
}

func TestTenantIDFromContext(t *testing.T) {
	// Happy path tests
	t.Run("retrieve tenant ID from context", func(t *testing.T) {
		c := qt.New(t)
		ctx := apiserver.WithTenantID(context.Background(), "tenant-123")
		tenantID := apiserver.TenantIDFromContext(ctx)

		c.Assert(tenantID, qt.Equals, "tenant-123")
	})

	t.Run("retrieve tenant ID from tenant context", func(t *testing.T) {
		c := qt.New(t)
		tenant := &models.Tenant{
			EntityID: models.EntityID{ID: "tenant-456"},
			Name:     "Test Tenant",
			Slug:     "test-tenant",
			Status:   models.TenantStatusActive,
		}

		ctx := apiserver.WithTenant(context.Background(), tenant)
		tenantID := apiserver.TenantIDFromContext(ctx)

		c.Assert(tenantID, qt.Equals, "tenant-456")
	})

	// Unhappy path tests
	t.Run("no tenant ID in context", func(t *testing.T) {
		c := qt.New(t)
		tenantID := apiserver.TenantIDFromContext(context.Background())
		c.Assert(tenantID, qt.Equals, "")
	})
}

func TestWithTenant(t *testing.T) {
	t.Run("add tenant to context", func(t *testing.T) {
		c := qt.New(t)
		tenant := &models.Tenant{
			EntityID: models.EntityID{ID: "tenant-123"},
			Name:     "Test Tenant",
			Slug:     "test-tenant",
			Status:   models.TenantStatusActive,
		}

		ctx := apiserver.WithTenant(context.Background(), tenant)

		retrievedTenant := apiserver.TenantFromContext(ctx)
		c.Assert(retrievedTenant, qt.IsNotNil)
		c.Assert(retrievedTenant.ID, qt.Equals, "tenant-123")

		tenantID := apiserver.TenantIDFromContext(ctx)
		c.Assert(tenantID, qt.Equals, "tenant-123")
	})
}

func TestWithTenantID(t *testing.T) {
	t.Run("add tenant ID to context", func(t *testing.T) {
		c := qt.New(t)
		ctx := apiserver.WithTenantID(context.Background(), "tenant-123")

		tenantID := apiserver.TenantIDFromContext(ctx)
		c.Assert(tenantID, qt.Equals, "tenant-123")

		// Tenant object should not be available
		tenant := apiserver.TenantFromContext(ctx)
		c.Assert(tenant, qt.IsNil)
	})
}
