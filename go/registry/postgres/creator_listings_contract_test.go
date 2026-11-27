package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/registrytest"
)

func TestCreatorListingsContract_Postgres(t *testing.T) {
	registrytest.RunCreatorListingsContract(t, func(t *testing.T) registrytest.CreatorListingsEnv {
		return newPostgresCreatorListingsEnv(t, nil)
	})
}

// TestCreatorListingsContract_Postgres_RLS runs the contract on a connection
// that logs in as a role without BYPASSRLS, the production role model: the
// default harness connects as the owner of the tables, which RLS does not
// bind.
func TestCreatorListingsContract_Postgres_RLS(t *testing.T) {
	registrytest.RunCreatorListingsContract(t, func(t *testing.T) registrytest.CreatorListingsEnv {
		c := qt.New(t)
		return newPostgresCreatorListingsEnv(t, newNonBypassAppFactory(c, skipIfNoPostgreSQL(t)))
	})
}

// newPostgresCreatorListingsEnv returns two members of the test group with
// the group's one location and area. The rows are written and read through
// factories, or through the harness's own factory set when it is nil.
func newPostgresCreatorListingsEnv(t *testing.T, factories *registry.FactorySet) registrytest.CreatorListingsEnv {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	if factories == nil {
		factories = fx.factorySet
	}
	second := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: fx.user.TenantID},
		Email:               "second-member@test-org.com",
		Name:                "Second Member",
		IsActive:            true,
	}
	c.Assert(second.SetPassword("TestPassword123"), qt.IsNil)
	userB, err := fx.set.UserRegistry.Create(c.Context(), second)
	c.Assert(err, qt.IsNil)
	location := createTestLocation(c, fx.set)
	area := createTestArea(c, fx.set, location.ID)
	return registrytest.CreatorListingsEnv{
		Factories:    factories,
		CtxA:         tagCtxFor(fx.user, fx.groupID),
		CtxB:         tagCtxFor(userB, fx.groupID),
		UserA:        fx.user.ID,
		UserB:        userB.ID,
		LocationID:   location.ID,
		LocationName: location.Name,
		AreaID:       area.ID,
		AreaName:     area.Name,
	}
}
