package memory_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

// A group the user created is handed to another member at deletion, so it
// counts as retained only when it has nobody to go to and is not purged.
func TestUserContentOwnership_GroupWithoutAnotherMemberIsRetained(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()
	handedOver := seedCreatedGroup(c, fs, "handed-over", "creator")
	seedMember(c, fs, handedOver, "owner", models.GroupRoleOwner)
	seedMember(c, fs, handedOver, "creator", models.GroupRoleUser)
	private := seedCreatedGroup(c, fs, "private", "creator")
	seedMember(c, fs, private, "creator", models.GroupRoleOwner)

	owns, err := fs.UserContentOwnershipChecker.HasRetainedOwnedContent(t.Context(), "tenant-1", "creator", nil)
	c.Assert(err, qt.IsNil)
	c.Check(owns, qt.IsTrue)

	owns, err = fs.UserContentOwnershipChecker.HasRetainedOwnedContent(t.Context(), "tenant-1", "creator", []string{private.ID})
	c.Assert(err, qt.IsNil)
	c.Check(owns, qt.IsFalse, qt.Commentf("the handed-over group has an heir and the private one is purged"))
}

func seedCreatedGroup(c *qt.C, fs *registry.FactorySet, slug, createdBy string) *models.LocationGroup {
	c.Helper()
	g, err := fs.LocationGroupRegistry.Create(c.Context(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Name:                slug,
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           createdBy,
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	return g
}

func seedMember(c *qt.C, fs *registry.FactorySet, g *models.LocationGroup, userID string, role models.GroupRole) {
	c.Helper()
	_, err := fs.GroupMembershipRegistry.Create(c.Context(), models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: g.TenantID},
		GroupID:             g.ID,
		MemberUserID:        userID,
		Role:                role,
	})
	c.Assert(err, qt.IsNil)
}
