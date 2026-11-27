package services_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/golang-jwt/jwt/v5"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/blobkeys"
	_ "go.5x5.cz/inventario/internal/fileblob" // register file:// driver
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
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
	svc := services.NewAccountDeletionService(fs, purger, services.NewAvatarService(fs, uploadLocation))
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

// The profile photo is not a file row, so the group purge does not reach it.
// Erasing the account has to remove it too, or a personal image outlives the
// account nobody can now remove it from.
func TestAccountDeletionService_DeleteAccount_RemovesTheProfilePhoto(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, uploadLocation := newAccountDeletionService(c)

	user := seedAccountUser(c, ctx, fs, "photo@example.com")
	seedAccountGroupWithMember(c, ctx, fs, "photo-group-slug-000000000000", user.ID, models.GroupRoleOwner)

	key, err := services.NewAvatarService(fs, uploadLocation).Store(ctx, user.ID, bytes.NewReader(pngBytes(c, 64, 64)))
	c.Assert(err, qt.IsNil)

	c.Assert(svc.DeleteAccount(ctx, user.TenantID, user.ID), qt.IsNil)

	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	exists, err := b.Exists(ctx, key)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
}

func TestAccountDeletionService_DeleteAccount_RemovesThePersonalDataExports(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, uploadLocation := newAccountDeletionService(c)

	user := seedAccountUser(c, ctx, fs, "export@example.com")
	seedAccountGroupWithMember(c, ctx, fs, "export-group-slug-00000000000", user.ID, models.GroupRoleOwner)
	now := time.Now().UTC()
	export, blocking, err := fs.PersonalDataExportRegistry.CreateUnlessRequestedSince(ctx, models.PersonalDataExport{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: user.TenantID, UserID: user.ID},
		CreatedAt:               now,
		ExpiresAt:               now.Add(models.PersonalDataExportRetention),
	}, now.Add(-models.PersonalDataExportRequestInterval))
	c.Assert(err, qt.IsNil)
	c.Assert(blocking, qt.IsNil)
	key := blobkeys.BuildPersonalDataExportBlobKey(user.TenantID, export.ID)
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	c.Assert(b.WriteAll(ctx, key, []byte("archive"), nil), qt.IsNil)

	c.Assert(svc.DeleteAccount(ctx, user.TenantID, user.ID), qt.IsNil)

	exists, err := b.Exists(ctx, key)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
	_, err = fs.PersonalDataExportRegistry.Get(ctx, export.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// An export requested before the user moved to another tenant keeps its
// archive under the old tenant; deleting the account removes that one too.
func TestAccountDeletionService_DeleteAccount_RemovesAnArchiveKeptUnderAnotherTenant(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, uploadLocation := newAccountDeletionService(c)

	user := seedAccountUser(c, ctx, fs, "moved-export@example.com")
	const before = "tenant-before-the-move"
	c.Assert(before, qt.Not(qt.Equals), user.TenantID)
	now := time.Now().UTC()
	export, blocking, err := fs.PersonalDataExportRegistry.CreateUnlessRequestedSince(ctx, models.PersonalDataExport{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: before, UserID: user.ID},
		CreatedAt:               now,
		ExpiresAt:               now.Add(models.PersonalDataExportRetention),
	}, now.Add(-models.PersonalDataExportRequestInterval))
	c.Assert(err, qt.IsNil)
	c.Assert(blocking, qt.IsNil)
	key := blobkeys.BuildPersonalDataExportBlobKey(before, export.ID)
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	c.Assert(b.WriteAll(ctx, key, []byte("archive"), nil), qt.IsNil)

	c.Assert(svc.DeleteAccount(ctx, user.TenantID, user.ID), qt.IsNil)

	exists, err := b.Exists(ctx, key)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
}

// TestAccountDeletionService_DeleteAccount_RemovesTheMembersItemReminders
// covers #2805 on the memory backend: a plain member's item reminders in a
// shared group email only that member, so they go with the account while the
// group and the other members' data stay.
func TestAccountDeletionService_DeleteAccount_RemovesTheMembersItemReminders(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, _ := newAccountDeletionService(c)

	leaving := seedAccountUser(c, ctx, fs, "leaving@example.com")
	staying := seedAccountUser(c, ctx, fs, "staying@example.com")
	group := seedAccountGroupCreatedBy(c, ctx, fs, "reminder-group-slug-0000000000", staying.ID, staying.ID, models.GroupRoleOwner)
	_, err := fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		MemberUserID:        leaving.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	createReminder := func(user *models.User, title string) *models.CommodityReminder {
		userCtx := appctx.WithGroup(appctx.WithUser(ctx, user), group)
		reg, err := fs.CommodityReminderRegistryFactory.CreateUserRegistry(userCtx)
		c.Assert(err, qt.IsNil)
		reminder, err := reg.Create(userCtx, models.CommodityReminder{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: group.ID, CreatedByUserID: user.ID},
			CommodityID:              "commodity-1",
			Title:                    title,
			DueOn:                    "2026-12-15",
			RemindDaysBefore:         models.ValuerSlice[int]{7},
			RepeatUnit:               models.ReminderRepeatNone,
			RepeatInterval:           1,
		})
		c.Assert(err, qt.IsNil)
		return reminder
	}
	mine := createReminder(leaving, "Return window")
	theirs := createReminder(staying, "Insurance renewal")
	won, err := fs.CommodityReminderSendRegistry.CreateOnce(ctx, models.CommodityReminderSend{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: group.ID, CreatedByUserID: leaving.ID},
		ReminderID:               mine.ID,
		OccurrenceOn:             "2026-12-15",
		LeadDays:                 7,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsTrue)

	c.Assert(svc.DeleteAccount(ctx, leaving.TenantID, leaving.ID), qt.IsNil)

	reminders := fs.CommodityReminderRegistryFactory.CreateServiceRegistry()
	_, err = reminders.Get(ctx, mine.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	n, err := fs.CommodityReminderSendRegistry.DeleteByReminder(ctx, mine.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)
	_, err = reminders.Get(ctx, theirs.ID)
	c.Assert(err, qt.IsNil)
}

// TestAccountDeletionService_DeleteAccount_RemovesTheMembersDrafts covers
// #2812 on the memory backend: the drafts a member created go with the
// account, and a file another member attached to one of them stays as a
// standalone file instead of pointing at a missing draft.
func TestAccountDeletionService_DeleteAccount_RemovesTheMembersDrafts(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, _ := newAccountDeletionService(c)

	leaving := seedAccountUser(c, ctx, fs, "leaving@example.com")
	staying := seedAccountUser(c, ctx, fs, "staying@example.com")
	group := seedAccountGroupCreatedBy(c, ctx, fs, "draft-group-slug-000000000000", staying.ID, staying.ID, models.GroupRoleOwner)
	_, err := fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		MemberUserID:        leaving.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	leavingCtx := appctx.WithGroup(appctx.WithUser(ctx, leaving), group)
	stayingCtx := appctx.WithGroup(appctx.WithUser(ctx, staying), group)
	createDraft := func(userCtx context.Context, key string) *models.CommodityDraft {
		reg, err := fs.CommodityDraftRegistryFactory.CreateUserRegistry(userCtx)
		c.Assert(err, qt.IsNil)
		draft, created, err := reg.CreateOrGet(userCtx, models.CommodityDraft{ClientKey: key, Values: models.CommodityDraftValues{"name": key}})
		c.Assert(err, qt.IsNil)
		c.Assert(created, qt.IsTrue)
		return draft
	}
	mine := createDraft(leavingCtx, "leaving-draft")
	theirs := createDraft(stayingCtx, "staying-draft")

	files, err := fs.FileRegistryFactory.CreateUserRegistry(stayingCtx)
	c.Assert(err, qt.IsNil)
	photo, err := files.Create(stayingCtx, models.FileEntity{
		Title:    "photo",
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		Links:    []models.FileLink{{EntityType: models.FileLinkDraft, EntityID: mine.ID, Meta: models.FileLinkMetaImages}},
		File:     &models.File{Path: "photo", OriginalPath: "photo.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	})
	c.Assert(err, qt.IsNil)

	c.Assert(svc.DeleteAccount(ctx, leaving.TenantID, leaving.ID), qt.IsNil)

	drafts := fs.CommodityDraftRegistryFactory.CreateServiceRegistry()
	_, err = drafts.Get(ctx, mine.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = drafts.Get(ctx, theirs.ID)
	c.Assert(err, qt.IsNil)
	kept, err := files.Get(stayingCtx, photo.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(kept.Links, qt.HasLen, 0)
}

// A member's records on another member's item in a shared group belong to
// the group (#2882): the deletion hands them to the owner instead of
// refusing, and the invites the member created or accepted go.
func TestAccountDeletionService_DeleteAccount_HandsTheMembersItemRecordsToTheOwner(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, _ := newAccountDeletionService(c)

	leaving := seedAccountUser(c, ctx, fs, "leaving@example.com")
	staying := seedAccountUser(c, ctx, fs, "staying@example.com")
	group := seedAccountGroupCreatedBy(c, ctx, fs, "records-group-slug-00000000000", staying.ID, staying.ID, models.GroupRoleOwner)
	_, err := fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		MemberUserID:        leaving.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	authored := models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: group.ID, CreatedByUserID: leaving.ID}
	leavingCtx := appctx.WithGroup(appctx.WithUser(ctx, leaving), group)
	event, err := fs.CommodityEventRegistryFactory.CreateServiceRegistry().Create(leavingCtx, models.CommodityEvent{
		TenantGroupAwareEntityID: authored,
		CommodityID:              "commodity-1",
		Kind:                     models.CommodityEventKindUpdated,
		OccurredAt:               time.Now(),
	})
	c.Assert(err, qt.IsNil)
	loans := fs.CommodityLoanRegistryFactory.CreateServiceRegistry()
	loan, err := loans.Create(leavingCtx, models.CommodityLoan{
		TenantGroupAwareEntityID: authored,
		CommodityID:              "commodity-1",
		BorrowerName:             "Neighbor",
		LentAt:                   models.Date("2026-05-01"),
	})
	c.Assert(err, qt.IsNil)
	won, err := fs.WarrantyReminderRegistry.CreateOnce(ctx, models.WarrantyReminder{TenantGroupAwareEntityID: authored, CommodityID: "commodity-1", ThresholdDays: 30})
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsTrue)

	sent, err := fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		Token:               "sent-by-leaving",
		CreatedBy:           leaving.ID,
		ExpiresAt:           time.Now().Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)
	usedAt := time.Now()
	accepted, err := fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		Token:               "accepted-by-leaving",
		CreatedBy:           staying.ID,
		ExpiresAt:           time.Now().Add(time.Hour),
		UsedBy:              &leaving.ID,
		UsedAt:              &usedAt,
	})
	c.Assert(err, qt.IsNil)

	c.Assert(svc.DeleteAccount(ctx, leaving.TenantID, leaving.ID), qt.IsNil)

	keptEvent, err := fs.CommodityEventRegistryFactory.CreateServiceRegistry().Get(ctx, event.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(keptEvent.CreatedByUserID, qt.Equals, staying.ID)
	keptLoan, err := loans.Get(ctx, loan.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(keptLoan.CreatedByUserID, qt.Equals, staying.ID)
	warranty, ok := fs.WarrantyReminderRegistry.(*memory.WarrantyReminderRegistry)
	c.Assert(ok, qt.IsTrue)
	reminders, err := warranty.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(reminders, qt.HasLen, 1)
	c.Assert(reminders[0].CreatedByUserID, qt.Equals, staying.ID)

	_, err = fs.GroupInviteRegistry.GetByToken(ctx, sent.Token)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = fs.GroupInviteRegistry.GetByToken(ctx, accepted.Token)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// A tag or seller a member created in a shared group is the group's (#2883):
// the deletion hands it to the owner instead of refusing, and records the
// hand-over in the audit log, naming the operator when the deletion runs in
// an impersonation session.
func TestAccountDeletionService_DeleteAccount_HandsTheMembersTagsAndSellersToTheOwner(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, _ := newAccountDeletionService(c)

	leaving := seedAccountUser(c, ctx, fs, "leaving@example.com")
	staying := seedAccountUser(c, ctx, fs, "staying@example.com")
	group := seedAccountGroupCreatedBy(c, ctx, fs, "catalog-group-slug-00000000000", staying.ID, staying.ID, models.GroupRoleOwner)
	_, err := fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		MemberUserID:        leaving.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	leavingCtx := appctx.WithGroup(appctx.WithUser(ctx, leaving), group)
	tags := must.Must(fs.TagRegistryFactory.CreateUserRegistry(leavingCtx))
	tag, err := tags.Create(leavingCtx, models.Tag{Kind: models.TagKindCommodity, Slug: "garage", Label: "Garage", Color: models.TagColorBlue})
	c.Assert(err, qt.IsNil)
	sellers := must.Must(fs.SellerRegistryFactory.CreateUserRegistry(leavingCtx))
	seller, err := sellers.Create(leavingCtx, models.Seller{Name: "Hardware store"})
	c.Assert(err, qt.IsNil)

	deleteCtx := appctx.WithJWTClaims(ctx, jwt.MapClaims{"imp": true, "impersonator_id": "operator-1"})
	c.Assert(svc.DeleteAccount(deleteCtx, leaving.TenantID, leaving.ID), qt.IsNil)

	keptTag, err := fs.TagRegistryFactory.CreateServiceRegistry().Get(ctx, tag.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(keptTag.CreatedByUserID, qt.Equals, staying.ID)
	keptSeller, err := fs.SellerRegistryFactory.CreateServiceRegistry().Get(ctx, seller.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(keptSeller.CreatedByUserID, qt.Equals, staying.ID)

	entries, err := fs.AuditLogRegistry.ListByAction(ctx, registry.AuditActionAccountDeletionHandOver)
	c.Assert(err, qt.IsNil)
	c.Assert(entries, qt.HasLen, 1)
	entry := entries[0]
	c.Assert(entry.UserID, qt.DeepEquals, &leaving.ID)
	c.Assert(entry.EntityID, qt.DeepEquals, &group.ID)
	c.Assert(entry.ImpersonatedBy, qt.DeepEquals, new("operator-1"))
	var handOver struct {
		ToUserID string         `json:"to_user_id"`
		Rows     map[string]int `json:"rows"`
	}
	c.Assert(json.Unmarshal([]byte(entry.UserAgent), &handOver), qt.IsNil)
	c.Assert(handOver.ToUserID, qt.Equals, staying.ID)
	c.Assert(handOver.Rows, qt.DeepEquals, map[string]int{"tags": 1, "sellers": 1})
}
