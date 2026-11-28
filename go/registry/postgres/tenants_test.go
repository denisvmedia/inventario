package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

func newRegistryTestTenant(name, slug string) models.Tenant {
	return models.Tenant{
		Name:   name,
		Slug:   slug,
		Status: models.TenantStatusActive,
	}
}

func TestTenantRegistry_Create_RefusesATakenSlug(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	_, err := registrySet.TenantRegistry.Create(ctx, newRegistryTestTenant("Acme", "acme-registry"))
	c.Assert(err, qt.IsNil)

	_, err = registrySet.TenantRegistry.Create(ctx, newRegistryTestTenant("Acme again", "acme-registry"))
	c.Assert(err, qt.ErrorIs, registry.ErrSlugAlreadyExists)
}

func TestTenantRegistry_Update_RefusesAnotherTenantsSlug(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	_, err := registrySet.TenantRegistry.Create(ctx, newRegistryTestTenant("Acme", "acme-registry"))
	c.Assert(err, qt.IsNil)
	globex, err := registrySet.TenantRegistry.Create(ctx, newRegistryTestTenant("Globex", "globex-registry"))
	c.Assert(err, qt.IsNil)

	taken := *globex
	taken.Slug = "acme-registry"
	_, err = registrySet.TenantRegistry.Update(ctx, taken)
	c.Assert(err, qt.ErrorIs, registry.ErrSlugAlreadyExists)

	stored, err := registrySet.TenantRegistry.Get(ctx, globex.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Slug, qt.Equals, "globex-registry")
}

func TestTenantRegistry_Update_KeepsItsOwnSlug(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	globex, err := registrySet.TenantRegistry.Create(ctx, newRegistryTestTenant("Globex", "globex-registry"))
	c.Assert(err, qt.IsNil)

	renamed := *globex
	renamed.Name = "Globex Corporation"
	updated, err := registrySet.TenantRegistry.Update(ctx, renamed)
	c.Assert(err, qt.IsNil)
	c.Assert(updated.Name, qt.Equals, "Globex Corporation")
	c.Assert(updated.Slug, qt.Equals, "globex-registry")
}

func newRegistryTestTenantWithDomain(name, slug, domain string) models.Tenant {
	tenant := newRegistryTestTenant(name, slug)
	tenant.Domain = new(domain)
	return tenant
}

// The service checks a domain before it writes, so these writes reach the
// registry only when two of them race past that check. The unique index then
// refuses the second, and the registry names the conflict.
func TestTenantRegistry_Create_RefusesATakenDomain(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	_, err := registrySet.TenantRegistry.Create(ctx, newRegistryTestTenantWithDomain("Acme", "acme-registry", "acme-registry.example"))
	c.Assert(err, qt.IsNil)

	_, err = registrySet.TenantRegistry.Create(ctx, newRegistryTestTenantWithDomain("Acme again", "acme-again-registry", "acme-registry.example"))
	c.Assert(err, qt.ErrorIs, registry.ErrDomainAlreadyExists)
}

func TestTenantRegistry_Update_RefusesAnotherTenantsDomain(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	_, err := registrySet.TenantRegistry.Create(ctx, newRegistryTestTenantWithDomain("Acme", "acme-registry", "acme-registry.example"))
	c.Assert(err, qt.IsNil)
	globex, err := registrySet.TenantRegistry.Create(ctx, newRegistryTestTenantWithDomain("Globex", "globex-registry", "globex-registry.example"))
	c.Assert(err, qt.IsNil)

	taken := *globex
	taken.Domain = new("acme-registry.example")
	_, err = registrySet.TenantRegistry.Update(ctx, taken)
	c.Assert(err, qt.ErrorIs, registry.ErrDomainAlreadyExists)

	stored, err := registrySet.TenantRegistry.Get(ctx, globex.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(*stored.Domain, qt.Equals, "globex-registry.example")
}

func TestTenantRegistry_LetsTenantsWithoutADomainThrough(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	_, err := registrySet.TenantRegistry.Create(ctx, newRegistryTestTenant("Acme", "acme-registry"))
	c.Assert(err, qt.IsNil)
	globex, err := registrySet.TenantRegistry.Create(ctx, newRegistryTestTenant("Globex", "globex-registry"))
	c.Assert(err, qt.IsNil)

	renamed := *globex
	renamed.Name = "Globex Corporation"
	_, err = registrySet.TenantRegistry.Update(ctx, renamed)
	c.Assert(err, qt.IsNil)
}
