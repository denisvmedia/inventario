package restore_test

import (
	"context"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ensureGroupForUser creates a default location group (stamped USD as its
// main valuation currency) for the given user, attaches the user as an
// admin member, and returns a context carrying both. The restore processor's
// validation path pulls the main currency off the group in context, so every
// test that drives the processor needs a group wired up this way.
func ensureGroupForUser(ctx context.Context, fs *registry.FactorySet, user *models.User) context.Context {
	slug := must.Must(models.GenerateGroupSlug())
	group := must.Must(fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Slug:                slug,
		Name:                "Test Group",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
		MainCurrency:        models.Currency("USD"),
	}))
	must.Must(fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		GroupID:             group.ID,
		MemberUserID:        user.ID,
		Role:                models.GroupRoleAdmin,
	}))
	return appctx.WithGroup(appctx.WithUser(ctx, user), group)
}
