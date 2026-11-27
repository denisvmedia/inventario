package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// Removing a member from a group removes what was personal to them there
// (#2892): their personal saved views and placements, notification
// overrides, item reminders, drafts, Add item sessions and unused invites,
// and revokes their API tokens limited to the group. What the rest of the
// group relies on stays: the member's shared views and other members'
// placements of them, a draft made for a delivered document, and an invite
// someone already accepted.
func TestGroupService_RemoveMember_RemovesWhatWasPersonalToTheMember(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	svc := services.NewGroupService(fs.LocationGroupRegistry, fs.GroupMembershipRegistry, fs.GroupInviteRegistry)

	owner := seedAccountUser(c, ctx, fs, "owner@example.com")
	member := seedAccountUser(c, ctx, fs, "member@example.com")
	group := seedAccountGroupCreatedBy(c, ctx, fs, "departure-group-slug-000000000", owner.ID, owner.ID, models.GroupRoleOwner)
	_, err := svc.AddMember(ctx, "tenant-a", group.ID, member.ID, models.GroupRoleUser)
	c.Assert(err, qt.IsNil)
	ownerCtx := appctx.WithGroup(appctx.WithUser(ctx, owner), group)
	memberCtx := appctx.WithGroup(appctx.WithUser(ctx, member), group)

	// Saved views and where each member shows them.
	pinned := registry.SavedViewPlacementPatch{ShowInSidebar: new(true)}
	createView := func(userCtx context.Context, name string, shared bool) *models.SavedView {
		v, err := fs.SavedViewRegistryFactory.MustCreateUserRegistry(userCtx).CreateChecked(userCtx, models.SavedView{
			Name: name, Icon: models.SavedViewIconBookmark, Entity: models.SavedViewEntityCommodities,
			QueryVersion: models.SavedViewQueryVersion, Shared: shared,
		}, &pinned, nil)
		c.Assert(err, qt.IsNil)
		return v
	}
	personal := createView(memberCtx, "Mine", false)
	memberShared := createView(memberCtx, "For everyone", true)
	ownerShared := createView(ownerCtx, "Owner's", true)
	_, err = fs.SavedViewRegistryFactory.MustCreateUserRegistry(memberCtx).SetPlacement(memberCtx, ownerShared.ID, pinned)
	c.Assert(err, qt.IsNil)
	_, err = fs.SavedViewRegistryFactory.MustCreateUserRegistry(ownerCtx).SetPlacement(ownerCtx, memberShared.ID, pinned)
	c.Assert(err, qt.IsNil)

	_, err = fs.GroupNotificationPrefRegistry.Upsert(ctx, models.GroupNotificationPref{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		UserID:              member.ID,
		Category:            "warranty",
		Enabled:             false,
	})
	c.Assert(err, qt.IsNil)

	reminders, err := fs.CommodityReminderRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	reminder, err := reminders.Create(memberCtx, models.CommodityReminder{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: group.ID, CreatedByUserID: member.ID},
		CommodityID:              "commodity-1",
		Title:                    "Return window",
		DueOn:                    "2026-12-15",
		RemindDaysBefore:         models.ValuerSlice[int]{7},
		RepeatUnit:               models.ReminderRepeatNone,
		RepeatInterval:           1,
	})
	c.Assert(err, qt.IsNil)

	drafts, err := fs.CommodityDraftRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	draft, _, err := drafts.CreateOrGet(memberCtx, models.CommodityDraft{ClientKey: "half-done", Values: models.CommodityDraftValues{"name": "Lamp"}})
	c.Assert(err, qt.IsNil)
	delivered, _, err := drafts.CreateOrGet(memberCtx, models.CommodityDraft{
		ClientKey: models.IntakeDeliveredDraftKeyPrefix + "paperless-7", Values: models.CommodityDraftValues{"name": "Invoice 7"},
	})
	c.Assert(err, qt.IsNil)
	files, err := fs.FileRegistryFactory.CreateUserRegistry(ownerCtx)
	c.Assert(err, qt.IsNil)
	photo, err := files.Create(ownerCtx, models.FileEntity{
		Title:    "photo",
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		Links:    []models.FileLink{{EntityType: models.FileLinkDraft, EntityID: draft.ID, Meta: models.FileLinkMetaImages}},
		File:     &models.File{Path: "photo", OriginalPath: "photo.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	})
	c.Assert(err, qt.IsNil)

	intake, err := fs.IntakeRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	session, _, err := intake.CreateSession(memberCtx, models.IntakeSession{ClientKey: "scan-1", Origin: models.IntakeOriginUpload}, nil, 0)
	c.Assert(err, qt.IsNil)

	unused, err := fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		Token:               "unused-from-member",
		CreatedBy:           member.ID,
		ExpiresAt:           time.Now().Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)
	usedAt := time.Now()
	accepted, err := fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		GroupID:             group.ID,
		Token:               "accepted-from-member",
		CreatedBy:           member.ID,
		ExpiresAt:           time.Now().Add(time.Hour),
		UsedBy:              &owner.ID,
		UsedAt:              &usedAt,
	})
	c.Assert(err, qt.IsNil)

	createToken := func(hash string, groupID *string) *models.APIToken {
		token, err := fs.APITokenRegistry.Create(ctx, models.APIToken{
			TenantUserAwareEntityID: models.WithTenantUserAwareEntityID("", "tenant-a", member.ID),
			Name:                    hash,
			TokenHash:               hash,
			TokenPrefix:             "inv_pat_" + hash[:4],
			Scope:                   models.APITokenScopeReadWrite,
			GroupID:                 groupID,
		})
		c.Assert(err, qt.IsNil)
		return token
	}
	groupToken := createToken("group-token-hash", &group.ID)
	anyGroupToken := createToken("any-group-token-hash", nil)

	c.Assert(svc.RemoveMember(ctx, group.ID, member.ID), qt.IsNil)

	views := fs.SavedViewRegistryFactory.CreateServiceRegistry()
	_, err = views.Get(ctx, personal.ID)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = views.Get(ctx, memberShared.ID)
	c.Check(err, qt.IsNil)
	ownerPlacements, err := fs.SavedViewRegistryFactory.MustCreateUserRegistry(ownerCtx).Placements(ownerCtx)
	c.Assert(err, qt.IsNil)
	c.Check(ownerPlacements[memberShared.ID].ShowInSidebar, qt.IsTrue)

	prefs, err := fs.GroupNotificationPrefRegistry.ListByUserGroup(ctx, "tenant-a", group.ID, member.ID)
	c.Assert(err, qt.IsNil)
	c.Check(prefs, qt.HasLen, 0)

	_, err = fs.CommodityReminderRegistryFactory.CreateServiceRegistry().Get(ctx, reminder.ID)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)

	serviceDrafts := fs.CommodityDraftRegistryFactory.CreateServiceRegistry()
	_, err = serviceDrafts.Get(ctx, draft.ID)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = serviceDrafts.Get(ctx, delivered.ID)
	c.Check(err, qt.IsNil)
	keptPhoto, err := files.Get(ownerCtx, photo.ID)
	c.Assert(err, qt.IsNil)
	c.Check(keptPhoto.Links, qt.HasLen, 0)

	_, err = fs.IntakeRegistryFactory.CreateServiceRegistry().Get(ctx, session.ID)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)

	_, err = fs.GroupInviteRegistry.GetByToken(ctx, unused.Token)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = fs.GroupInviteRegistry.GetByToken(ctx, accepted.Token)
	c.Check(err, qt.IsNil)

	for _, token := range []*models.APIToken{groupToken, anyGroupToken} {
		got, err := fs.APITokenRegistry.GetByTokenHash(ctx, token.TokenHash)
		c.Assert(err, qt.IsNil)
		c.Check(got.RevokedAt != nil, qt.Equals, token == groupToken, qt.Commentf("token %s", token.Name))
	}
}
