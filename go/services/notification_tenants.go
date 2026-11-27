package services

import (
	"context"
	"errors"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// tenantGate tells a notification worker whether the tenant behind a row is
// active. While a tenant is suspended or inactive its users cannot sign in
// and every link in an email answers 403, so the workers send them nothing:
// reminders, storage quota warnings, the weekly digest and the personal data
// export email.
//
// A skipped row writes no idempotency row, flips no sent flag and claims no
// digest week, the same as a row of a hidden section (sectionGate). A
// reminder still due once the tenant is active again goes out then, at the
// closest lead time reached, and the next digest goes out as usual.
//
// Each tenant is read once per sweep.
type tenantGate struct {
	tenants  registry.TenantRegistry
	byTenant map[string]bool
}

func newTenantGate(tenants registry.TenantRegistry) *tenantGate {
	return &tenantGate{tenants: tenants, byTenant: map[string]bool{}}
}

// active reports whether the tenant is active. Like a group that is gone for
// sectionGate, a tenant the registry does not have holds nothing back: every
// row references its tenant through a foreign key, and a tenant is deleted
// after its rows, so in a database the lookup finds it. A gate without a
// tenant registry lets everything through as well. Any other lookup error is
// returned, so the caller fails the row for this sweep and retries it on the
// next one instead of emailing a tenant that may be suspended.
func (g *tenantGate) active(ctx context.Context, tenantID string) (bool, error) {
	if g == nil || g.tenants == nil {
		return true, nil
	}
	if active, ok := g.byTenant[tenantID]; ok {
		return active, nil
	}
	tenant, err := g.tenants.Get(ctx, tenantID)
	var active bool
	switch {
	case errors.Is(err, registry.ErrNotFound):
		active = true
	case err != nil:
		return false, err
	default:
		active = tenant.Status == models.TenantStatusActive
	}
	g.byTenant[tenantID] = active
	return active, nil
}

// reminderGate is what a reminder sweep checks before it looks at a row's
// dates: the row's tenant is active and its group does not show the section
// hidden.
type reminderGate struct {
	tenants  *tenantGate
	sections *sectionGate
}

func newReminderGate(fs *registry.FactorySet) *reminderGate {
	gate := &reminderGate{sections: newSectionGate(fs)}
	if fs != nil {
		gate.tenants = newTenantGate(fs.TenantRegistry)
	}
	return gate
}

// skips reports whether the sweep leaves the row alone this time: its tenant
// is not active, or its group hides section.
func (g *reminderGate) skips(ctx context.Context, tenantID, groupID string, section models.GroupSection) (bool, error) {
	active, err := g.tenants.active(ctx, tenantID)
	if err != nil {
		return false, err
	}
	if !active {
		return true, nil
	}
	return g.sections.hides(ctx, groupID, section)
}
