package services_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register file:// driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// newAccountDeletionService builds an AccountDeletionService over a fresh
// in-memory factory set, reusing the file:// upload-location helper from
// group_purge_service_test.go.
func newAccountDeletionService(c *qt.C) (context.Context, *registry.FactorySet, *services.AccountDeletionService, string) {
	ctx := context.Background()
	uploadLocation := newFileUploadLocation(c)
	fs := memory.NewFactorySet()
	fileSvc := services.NewFileService(fs, uploadLocation)
	purger := services.NewGroupPurgeService(fs, fileSvc)
	svc := services.NewAccountDeletionService(fs, purger)
	return ctx, fs, svc, uploadLocation
}

// seedAccountUser inserts an active password user in tenant-a.
func seedAccountUser(c *qt.C, ctx context.Context, fs *registry.FactorySet, email string) *models.User {
	c.Helper()
	u := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		Email:               email,
		Name:                email,
		IsActive:            true,
	}
	c.Assert(u.SetPassword("Sup3r-Secret-Pw!"), qt.IsNil)
	created, err := fs.UserRegistry.Create(ctx, u)
	c.Assert(err, qt.IsNil)
	return created
}

// seedAccountGroupWithMember creates an active group CREATED BY the given user
// and adds that user's membership in it with the supplied role. Returns the
// created group.
func seedAccountGroupWithMember(c *qt.C, ctx context.Context, fs *registry.FactorySet, slug string, userID string, role models.GroupRole) *models.LocationGroup {
	c.Helper()
	return seedAccountGroupCreatedBy(c, ctx, fs, slug, userID, userID, role)
}

// seedAccountGroupCreatedBy creates an active group whose CreatedBy is
// createdByUserID, and adds memberUserID as a member with the supplied role.
// It lets a test seed a shared group that the member did NOT create (so the
// #2147 content-ownership pre-check does not flag the member as a content
// owner via the group's created_by).
func seedAccountGroupCreatedBy(c *qt.C, ctx context.Context, fs *registry.FactorySet, slug, createdByUserID, memberUserID string, role models.GroupRole) *models.LocationGroup {
	c.Helper()
	group, err := fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		Slug:                slug,
		Name:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           createdByUserID,
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	_, err = fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		MemberUserID:        memberUserID,
		Role:                role,
	})
	c.Assert(err, qt.IsNil)
	return group
}

// TestAccountDeletionService_DeleteAccount_PrivateGroup verifies the happy
// path: a user who is the sole member of one private group has that group, its
// content (a file + physical blob) and their own user row all hard-deleted,
// and DeleteAccount returns nil.
func TestAccountDeletionService_DeleteAccount_PrivateGroup(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, uploadLocation := newAccountDeletionService(c)

	user := seedAccountUser(c, ctx, fs, "solo@example.com")
	group := seedAccountGroupWithMember(c, ctx, fs, "solo-group-slug-0000000000000", user.ID, models.GroupRoleOwner)

	// Seed a physical blob + a FileEntity in the private group so the purge
	// has real content to remove.
	blobPath := "solo/file.txt"
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	c.Assert(b.WriteAll(ctx, blobPath, []byte("payload"), nil), qt.IsNil)
	c.Assert(b.Close(), qt.IsNil)

	fileReg := fs.FileRegistryFactory.CreateServiceRegistry()
	file, err := fileReg.Create(ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        "tenant-a",
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		Title: "solo-file",
		Type:  models.FileTypeDocument,
		File: &models.File{
			Path:         "solo/file",
			OriginalPath: blobPath,
			Ext:          ".txt",
			MIMEType:     "text/plain",
		},
	})
	c.Assert(err, qt.IsNil)

	err = svc.DeleteAccount(ctx, user.TenantID, user.ID)
	c.Assert(err, qt.IsNil)

	// User row is gone.
	_, err = fs.UserRegistry.Get(ctx, user.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Private group row is gone.
	_, err = fs.LocationGroupRegistry.Get(ctx, group.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The group's membership is gone.
	memberships, err := fs.GroupMembershipRegistry.ListByUser(ctx, "tenant-a", user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(memberships, qt.HasLen, 0)

	// The group's file is gone.
	_, err = fileReg.Get(ctx, file.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Physical blob removed.
	b, err = blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	exists, err := b.Exists(ctx, blobPath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
}

// TestAccountDeletionService_DeleteAccount_SoleOwnerOfSharedGroup verifies the
// abort-before-mutating contract: a user who is the sole owner of a group that
// still has another member is refused with ErrAccountSoleOwnerOfSharedGroup,
// and NOTHING is deleted (the user, the group, and both memberships survive).
func TestAccountDeletionService_DeleteAccount_SoleOwnerOfSharedGroup(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, _ := newAccountDeletionService(c)

	owner := seedAccountUser(c, ctx, fs, "owner@example.com")
	other := seedAccountUser(c, ctx, fs, "member@example.com")

	group := seedAccountGroupWithMember(c, ctx, fs, "shared-group-slug-000000000000", owner.ID, models.GroupRoleOwner)
	// A second, non-owner member makes the group shared, so the sole owner
	// cannot erase their account without transferring ownership first.
	_, err := fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		MemberUserID:        other.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	err = svc.DeleteAccount(ctx, owner.TenantID, owner.ID)
	c.Assert(err, qt.ErrorIs, services.ErrAccountSoleOwnerOfSharedGroup)

	// Nothing was deleted: owner, group and both memberships survive.
	_, err = fs.UserRegistry.Get(ctx, owner.ID)
	c.Assert(err, qt.IsNil)
	_, err = fs.LocationGroupRegistry.Get(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	members, err := fs.GroupMembershipRegistry.ListByGroup(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(members, qt.HasLen, 2)
}

// TestAccountDeletionService_DeleteAccount_CoOwnerWhoCreatedSharedGroupOwnsContent
// verifies the #2147 abort-before-mutate pre-check: a co-owner who CREATED the
// shared group (its created_by -> users(id) is a NOT NULL FK that survives the
// retained group) is refused with ErrAccountStillOwnsContent, and NOTHING is
// mutated. Before #2147 this user was deleted, leaving the group's created_by
// dangling — exactly the half-erased state the pre-check now prevents.
func TestAccountDeletionService_DeleteAccount_CoOwnerWhoCreatedSharedGroupOwnsContent(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, _ := newAccountDeletionService(c)

	leaving := seedAccountUser(c, ctx, fs, "leaving@example.com")
	staying := seedAccountUser(c, ctx, fs, "staying@example.com")

	// The leaving user CREATED this shared group (CreatedBy = leaving.ID).
	group := seedAccountGroupWithMember(c, ctx, fs, "coowned-group-slug-00000000000", leaving.ID, models.GroupRoleOwner)
	_, err := fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		MemberUserID:        staying.ID,
		Role:                models.GroupRoleOwner,
	})
	c.Assert(err, qt.IsNil)

	err = svc.DeleteAccount(ctx, leaving.TenantID, leaving.ID)
	c.Assert(err, qt.ErrorIs, services.ErrAccountStillOwnsContent)

	// Nothing mutated: the leaving user, the group, and both memberships survive.
	_, err = fs.UserRegistry.Get(ctx, leaving.ID)
	c.Assert(err, qt.IsNil)
	_, err = fs.LocationGroupRegistry.Get(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	members, err := fs.GroupMembershipRegistry.ListByGroup(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(members, qt.HasLen, 2)
}

// TestAccountDeletionService_DeleteAccount_CoOwnerCreatedNothingLeavesSharedGroupIntact
// verifies the success path for a co-owner who created NEITHER the shared group
// NOR any content in it: DeleteAccount succeeds, leaving the group + the other
// owner intact and removing only the leaving user's membership and user row.
func TestAccountDeletionService_DeleteAccount_CoOwnerCreatedNothingLeavesSharedGroupIntact(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, _ := newAccountDeletionService(c)

	leaving := seedAccountUser(c, ctx, fs, "leaving@example.com")
	staying := seedAccountUser(c, ctx, fs, "staying@example.com")

	// The group is CREATED BY staying; leaving is only a co-owner member and
	// authored no content, so the #2147 pre-check finds nothing the user owns.
	group := seedAccountGroupCreatedBy(c, ctx, fs, "coowned-group-slug-00000000000", staying.ID, staying.ID, models.GroupRoleOwner)
	_, err := fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		MemberUserID:        leaving.ID,
		Role:                models.GroupRoleOwner,
	})
	c.Assert(err, qt.IsNil)

	err = svc.DeleteAccount(ctx, leaving.TenantID, leaving.ID)
	c.Assert(err, qt.IsNil)

	// Leaving user is gone; group and the staying owner survive.
	_, err = fs.UserRegistry.Get(ctx, leaving.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = fs.LocationGroupRegistry.Get(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	members, err := fs.GroupMembershipRegistry.ListByGroup(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(members, qt.HasLen, 1)
	c.Assert(members[0].MemberUserID, qt.Equals, staying.ID)
}

// TestAccountDeletionService_DeleteAccount_AuthoredContentInRetainedGroup
// verifies the #2147 pre-check fires on authored CONTENT (not just the group's
// created_by): a co-owner member who created a commodity in a RETAINED shared
// group is refused with ErrAccountStillOwnsContent and nothing is mutated.
func TestAccountDeletionService_DeleteAccount_AuthoredContentInRetainedGroup(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, _ := newAccountDeletionService(c)

	leaving := seedAccountUser(c, ctx, fs, "leaving@example.com")
	staying := seedAccountUser(c, ctx, fs, "staying@example.com")

	// Group is created by staying; leaving is a co-owner who authored content.
	group := seedAccountGroupCreatedBy(c, ctx, fs, "content-group-slug-0000000000", staying.ID, staying.ID, models.GroupRoleOwner)
	_, err := fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		MemberUserID:        leaving.ID,
		Role:                models.GroupRoleOwner,
	})
	c.Assert(err, qt.IsNil)

	// The leaving user authors a commodity in the retained group. CreateWithUser
	// stamps created_by_user_id + tenant_id from the user in context; the
	// service registry leaves the explicit GroupID intact.
	commodityReg := fs.CommodityRegistryFactory.CreateServiceRegistry()
	commodity, err := commodityReg.Create(appctx.WithUser(ctx, leaving), models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			GroupID: group.ID,
		},
		Name:      "Leaving User Drill",
		ShortName: "drill",
		Type:      models.CommodityTypeEquipment,
		Status:    models.CommodityStatusInUse,
		Count:     1,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(commodity.GetCreatedByUserID(), qt.Equals, leaving.ID)

	err = svc.DeleteAccount(ctx, leaving.TenantID, leaving.ID)
	c.Assert(err, qt.ErrorIs, services.ErrAccountStillOwnsContent)

	// Nothing mutated: the user, the group, both memberships, and the commodity
	// all survive.
	_, err = fs.UserRegistry.Get(ctx, leaving.ID)
	c.Assert(err, qt.IsNil)
	_, err = fs.LocationGroupRegistry.Get(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	members, err := fs.GroupMembershipRegistry.ListByGroup(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(members, qt.HasLen, 2)
	_, err = commodityReg.Get(ctx, commodity.GetID())
	c.Assert(err, qt.IsNil)
}
