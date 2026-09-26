package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func TestTenantStatus_Validate(t *testing.T) {
	// Happy path tests
	t.Run("valid tenant statuses", func(t *testing.T) {
		testCases := []struct {
			name   string
			status models.TenantStatus
		}{
			{
				name:   "active status",
				status: models.TenantStatusActive,
			},
			{
				name:   "suspended status",
				status: models.TenantStatusSuspended,
			},
			{
				name:   "inactive status",
				status: models.TenantStatusInactive,
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				c := qt.New(t)
				err := tc.status.Validate()
				c.Assert(err, qt.IsNil)
			})
		}
	})

	// Unhappy path tests
	t.Run("invalid tenant status", func(t *testing.T) {
		c := qt.New(t)
		invalidStatus := models.TenantStatus("invalid")
		err := invalidStatus.Validate()
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Contains, "must be one of: active, suspended, inactive")
	})
}

func TestTenant_ValidateWithContext(t *testing.T) {
	// Happy path tests
	t.Run("valid tenant", func(t *testing.T) {
		c := qt.New(t)
		tenant := &models.Tenant{
			Name:   "Test Tenant",
			Slug:   "test-tenant",
			Status: models.TenantStatusActive,
			Domain: new("test.example.com"),
		}

		err := tenant.ValidateWithContext(context.Background())
		c.Assert(err, qt.IsNil)
	})

	t.Run("valid tenant with minimal fields", func(t *testing.T) {
		c := qt.New(t)
		tenant := &models.Tenant{
			Name:   "Test",
			Slug:   "test",
			Status: models.TenantStatusActive,
		}

		err := tenant.ValidateWithContext(context.Background())
		c.Assert(err, qt.IsNil)
	})

	t.Run("valid tenant with numbers and hyphens in slug", func(t *testing.T) {
		c := qt.New(t)
		tenant := &models.Tenant{
			Name:   "Test Tenant 123",
			Slug:   "test-tenant-123",
			Status: models.TenantStatusActive,
		}

		err := tenant.ValidateWithContext(context.Background())
		c.Assert(err, qt.IsNil)
	})

	// Unhappy path tests
	t.Run("invalid tenant cases", func(t *testing.T) {
		testCases := []struct {
			name        string
			tenant      *models.Tenant
			expectedErr string
		}{
			{
				name: "empty name",
				tenant: &models.Tenant{
					Name:   "",
					Slug:   "test",
					Status: models.TenantStatusActive,
				},
				expectedErr: "cannot be blank",
			},
			{
				name: "empty slug",
				tenant: &models.Tenant{
					Name:   "Test",
					Slug:   "",
					Status: models.TenantStatusActive,
				},
				expectedErr: "cannot be blank",
			},
			{
				name: "invalid slug with uppercase",
				tenant: &models.Tenant{
					Name:   "Test",
					Slug:   "Test-Tenant",
					Status: models.TenantStatusActive,
				},
				expectedErr: "must be in a valid format",
			},
			{
				name: "invalid slug with spaces",
				tenant: &models.Tenant{
					Name:   "Test",
					Slug:   "test tenant",
					Status: models.TenantStatusActive,
				},
				expectedErr: "must be in a valid format",
			},
			{
				name: "invalid slug with special characters",
				tenant: &models.Tenant{
					Name:   "Test",
					Slug:   "test@tenant",
					Status: models.TenantStatusActive,
				},
				expectedErr: "must be in a valid format",
			},
			{
				name: "name too long",
				tenant: &models.Tenant{
					Name:   "This is a very long tenant name that exceeds the maximum allowed length of 100 characters for testing purposes",
					Slug:   "test",
					Status: models.TenantStatusActive,
				},
				expectedErr: "the length must be between 1 and 100",
			},
			{
				name: "slug too long",
				tenant: &models.Tenant{
					Name:   "Test",
					Slug:   "this-is-a-very-long-slug-that-exceeds-the-maximum-allowed-length-of-50-characters",
					Status: models.TenantStatusActive,
				},
				expectedErr: "the length must be between 1 and 50",
			},
			{
				name: "domain too long",
				tenant: &models.Tenant{
					Name:   "Test",
					Slug:   "test",
					Status: models.TenantStatusActive,
					Domain: new("this-is-a-very-long-domain-name-that-exceeds-the-maximum-allowed-length-of-255-characters-for-testing-purposes-and-should-fail-validation-because-it-is-way-too-long-for-a-domain-name-in-any-practical-scenario-that-we-might-encounter-in-real-world-usage-and-this-should-definitely-be-over-255-characters-now.example.com"),
				},
				expectedErr: "the length must be between 1 and 255",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				c := qt.New(t)
				err := tc.tenant.ValidateWithContext(context.Background())
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Contains, tc.expectedErr)
			})
		}
	})
}

func TestTenant_MarshalJSON(t *testing.T) {
	t.Run("marshal tenant to JSON", func(t *testing.T) {
		c := qt.New(t)
		tenant := &models.Tenant{
			Name:   "Test Tenant",
			Slug:   "test-tenant",
			Status: models.TenantStatusActive,
			Domain: new("test.example.com"),
		}

		data, err := tenant.MarshalJSON()
		c.Assert(err, qt.IsNil)
		c.Assert(string(data), qt.Contains, "Test Tenant")
		c.Assert(string(data), qt.Contains, "test-tenant")
		c.Assert(string(data), qt.Contains, "active")
		c.Assert(string(data), qt.Contains, "test.example.com")
	})
}

func TestTenant_UnmarshalJSON(t *testing.T) {
	t.Run("unmarshal JSON to tenant", func(t *testing.T) {
		c := qt.New(t)
		jsonData := `{
			"name": "Test Tenant",
			"slug": "test-tenant",
			"status": "active",
			"domain": "test.example.com"
		}`

		var tenant models.Tenant
		err := tenant.UnmarshalJSON([]byte(jsonData))
		c.Assert(err, qt.IsNil)
		c.Assert(tenant.Name, qt.Equals, "Test Tenant")
		c.Assert(tenant.Slug, qt.Equals, "test-tenant")
		c.Assert(tenant.Status, qt.Equals, models.TenantStatusActive)
		c.Assert(tenant.Domain, qt.IsNotNil)
		c.Assert(*tenant.Domain, qt.Equals, "test.example.com")
	})
}

// The domain column is matched against the normalized request Host verbatim,
// so a value that cannot equal a normalized host is a tenant nobody can
// reach. Rejecting it on write turns that into an error the operator sees
// (#1036).
func TestTenant_ValidateWithContext_Domain(t *testing.T) {
	valid := func(domain string) *models.Tenant {
		return &models.Tenant{
			Name:   "Test Tenant",
			Slug:   "test-tenant",
			Status: models.TenantStatusActive,
			Domain: new(domain),
		}
	}

	t.Run("accepted", func(t *testing.T) {
		for _, domain := range []string{
			"acme.com",
			"inventory.acme.co.uk",
			"my-shop.example.test",
			"a.b",
			"localhost",
			"x1-2y.example.com",
		} {
			t.Run(domain, func(t *testing.T) {
				c := qt.New(t)
				c.Assert(valid(domain).ValidateWithContext(context.Background()), qt.IsNil)
			})
		}
	})

	t.Run("refused", func(t *testing.T) {
		for _, tc := range []struct{ name, domain string }{
			{"uppercase", "Acme.com"},
			{"with a port", "acme.com:8080"},
			{"with a scheme", "https://acme.com"},
			{"trailing dot", "acme.com."},
			{"leading dot", ".acme.com"},
			{"with a path", "acme.com/inventory"},
			{"underscore", "my_shop.example.com"},
			{"hyphen at a label edge", "-acme.com"},
			{"double dot", "acme..com"},
			{"a space", "acme .com"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				c := qt.New(t)
				err := valid(tc.domain).ValidateWithContext(context.Background())
				c.Assert(err, qt.IsNotNil)
				c.Assert(err.Error(), qt.Contains, "domain")
			})
		}
	})

	// An absent domain and an empty one are both "no custom domain", and
	// neither may be refused: most tenants have none.
	t.Run("no domain is fine", func(t *testing.T) {
		c := qt.New(t)
		tenant := valid("acme.com")
		tenant.Domain = nil
		c.Assert(tenant.ValidateWithContext(context.Background()), qt.IsNil)

		tenant.Domain = new("")
		c.Assert(tenant.ValidateWithContext(context.Background()), qt.IsNil)
	})
}
