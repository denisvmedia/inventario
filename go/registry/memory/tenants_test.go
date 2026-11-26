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
