package postgres_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// groupCreatorFixture is a group that creator created and owner owns, in
// the tenant of the loan fixture.
type groupCreatorFixture struct {
	fs      *registry.FactorySet
	db      *sqlx.DB
	owner   *models.User
	creator *models.User
	group   *models.LocationGroup
}

func newGroupCreatorFixture(t *testing.T) groupCreatorFixture {
	t.Helper()
	c := qt.New(t)
	f := newLoanPGFixture(t)
	ctx := context.Background()
	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)
	fs := f.factorySet

	creator, err := fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.user.TenantID},
		Email:               "group-creator@test-org.com",
		Name:                "Group Creator",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	slug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	group, err := fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.user.TenantID},
		Name:                "Handed over",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           creator.ID,
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	addPGMember(c, fs, group, f.user.ID, models.GroupRoleOwner)

	return groupCreatorFixture{
		fs:      fs,
		db:      sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx"),
		owner:   f.user,
		creator: creator,
		group:   group,
	}
}

func addPGMember(c *qt.C, fs *registry.FactorySet, group *models.LocationGroup, userID string, role models.GroupRole) {
	c.Helper()
	_, err := fs.GroupMembershipRegistry.Create(context.Background(), models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: group.TenantID},
		GroupID:             group.ID,
		MemberUserID:        userID,
		Role:                role,
	})
	c.Assert(err, qt.IsNil)
}

func (f groupCreatorFixture) deleteCreator(c *qt.C) error {
	c.Helper()
	fileSvc := services.NewFileService(f.fs, newPostgresFileUploadLocation(c))
	deletion := services.NewAccountDeletionService(f.fs, services.NewGroupPurgeService(f.fs, fileSvc), nil)
	return deletion.DeleteAccount(context.Background(), f.creator.TenantID, f.creator.ID)
}

func (f groupCreatorFixture) groupCreator(c *qt.C) string {
	c.Helper()
	var createdBy string
	c.Assert(f.db.QueryRowContext(context.Background(),
		"SELECT created_by FROM location_groups WHERE id = $1", f.group.ID).Scan(&createdBy), qt.IsNil)
	return createdBy
}

// A group's creator who handed ownership to someone else and stayed as a
// member can delete their account: the group passes to its owner, and the
// hand-over is in the audit log.
func TestAccountDeletion_Postgres_CreatorWhoHandedOwnershipOver(t *testing.T) {
	c := qt.New(t)
	f := newGroupCreatorFixture(t)
	addPGMember(c, f.fs, f.group, f.creator.ID, models.GroupRoleUser)

	c.Assert(f.deleteCreator(c), qt.IsNil)

	_, err := f.fs.UserRegistry.Get(context.Background(), f.creator.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Check(f.groupCreator(c), qt.Equals, f.owner.ID)

	var breadcrumb string
	c.Assert(f.db.QueryRowContext(context.Background(),
		"SELECT user_agent FROM audit_logs WHERE action = $1 AND entity_id = $2",
		registry.AuditActionAccountDeletionHandOver, f.group.ID).Scan(&breadcrumb), qt.IsNil)
	var handOver struct {
		ToUserID string         `json:"to_user_id"`
		Rows     map[string]int `json:"rows"`
	}
	c.Assert(json.Unmarshal([]byte(breadcrumb), &handOver), qt.IsNil)
	c.Check(handOver.ToUserID, qt.Equals, f.owner.ID)
	c.Check(handOver.Rows, qt.DeepEquals, map[string]int{"location_groups": 1})
}

// A group's creator who handed ownership over and then left the group can
// delete their account too.
func TestAccountDeletion_Postgres_CreatorWhoLeftTheGroup(t *testing.T) {
	c := qt.New(t)
	f := newGroupCreatorFixture(t)

	c.Assert(f.deleteCreator(c), qt.IsNil)

	c.Check(f.groupCreator(c), qt.Equals, f.owner.ID)
}

// A group the user created that has no other member has nobody to take it,
// so the pre-check counts it unless the deletion purges the group.
func TestUserContentOwnership_Postgres_GroupWithoutAnotherMemberIsRetained(t *testing.T) {
	c := qt.New(t)
	f := newGroupCreatorFixture(t)
	ctx := context.Background()
	slug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	private, err := f.fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.creator.TenantID},
		Name:                "Private",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           f.creator.ID,
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	addPGMember(c, f.fs, private, f.creator.ID, models.GroupRoleOwner)

	owns, err := f.fs.UserContentOwnershipChecker.HasRetainedOwnedContent(ctx, f.creator.TenantID, f.creator.ID, nil)
	c.Assert(err, qt.IsNil)
	c.Check(owns, qt.IsTrue)

	owns, err = f.fs.UserContentOwnershipChecker.HasRetainedOwnedContent(ctx, f.creator.TenantID, f.creator.ID, []string{private.ID})
	c.Assert(err, qt.IsNil)
	c.Check(owns, qt.IsFalse, qt.Commentf("the handed-over group has an heir and the private one is purged"))
}
