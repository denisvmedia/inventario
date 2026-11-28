package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

func newMemoryTestTenant(name, slug string) models.Tenant {
	return models.Tenant{Name: name, Slug: slug, Status: models.TenantStatusActive}
}

func TestTenantRegistry_SlugIsUnique(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	reg := memory.NewTenantRegistry()

	acme, err := reg.Create(ctx, newMemoryTestTenant("Acme", "acme"))
	c.Assert(err, qt.IsNil)
	globex, err := reg.Create(ctx, newMemoryTestTenant("Globex", "globex"))
	c.Assert(err, qt.IsNil)

	_, err = reg.Create(ctx, newMemoryTestTenant("Acme again", "acme"))
	c.Assert(err, qt.ErrorIs, registry.ErrSlugAlreadyExists)

	taken := *globex
	taken.Slug = acme.Slug
	_, err = reg.Update(ctx, taken)
	c.Assert(err, qt.ErrorIs, registry.ErrSlugAlreadyExists)

	renamed := *globex
	renamed.Name = "Globex Corporation"
	updated, err := reg.Update(ctx, renamed)
	c.Assert(err, qt.IsNil)
	c.Assert(updated.Slug, qt.Equals, "globex")
}

func TestTenantRegistry_LetsSlugLessTenantsThrough(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	reg := memory.NewTenantRegistry()

	// Fixtures that skip model validation seed tenants without a slug; two
	// of them must not collide on the empty string.
	_, err := reg.Create(ctx, newMemoryTestTenant("First", ""))
	c.Assert(err, qt.IsNil)
	_, err = reg.Create(ctx, newMemoryTestTenant("Second", ""))
	c.Assert(err, qt.IsNil)
}

func newMemoryTestTenantWithDomain(name, slug, domain string) models.Tenant {
	tenant := newMemoryTestTenant(name, slug)
	tenant.Domain = new(domain)
	return tenant
}

func TestTenantRegistry_DomainIsUnique(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	reg := memory.NewTenantRegistry()

	_, err := reg.Create(ctx, newMemoryTestTenantWithDomain("Acme", "acme", "acme.example"))
	c.Assert(err, qt.IsNil)
	globex, err := reg.Create(ctx, newMemoryTestTenantWithDomain("Globex", "globex", "globex.example"))
	c.Assert(err, qt.IsNil)

	_, err = reg.Create(ctx, newMemoryTestTenantWithDomain("Acme again", "acme-again", "acme.example"))
	c.Assert(err, qt.ErrorIs, registry.ErrDomainAlreadyExists)

	taken := *globex
	taken.Domain = new("acme.example")
	_, err = reg.Update(ctx, taken)
	c.Assert(err, qt.ErrorIs, registry.ErrDomainAlreadyExists)

	renamed := *globex
	renamed.Name = "Globex Corporation"
	updated, err := reg.Update(ctx, renamed)
	c.Assert(err, qt.IsNil)
	c.Assert(*updated.Domain, qt.Equals, "globex.example")
}

func TestTenantRegistry_LetsDomainLessTenantsThrough(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	reg := memory.NewTenantRegistry()

	_, err := reg.Create(ctx, newMemoryTestTenant("First", "first"))
	c.Assert(err, qt.IsNil)
	second, err := reg.Create(ctx, newMemoryTestTenant("Second", "second"))
	c.Assert(err, qt.IsNil)

	renamed := *second
	renamed.Name = "Second renamed"
	_, err = reg.Update(ctx, renamed)
	c.Assert(err, qt.IsNil)
}
