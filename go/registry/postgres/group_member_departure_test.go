package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// Removing a member from a group removes, in the transaction that deletes
// the membership, what was personal to them there (#2892), and keeps what
// the rest of the group relies on. See the memory test of the same name in
// services for the list.
func TestGroupMembership_Postgres_RemoveMemberRemovesWhatWasPersonalToTheMember(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	ctx := context.Background()
	fs := fx.factorySet
	owner := fx.user
	tenantID := owner.TenantID

	memberships := fs.GroupMembershipRegistry
	if _, err := memberships.GetByGroupAndUser(ctx, fx.groupID, owner.ID); err != nil {
		_, err = memberships.Create(ctx, models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
			GroupID:             fx.groupID,
			MemberUserID:        owner.ID,
			Role:                models.GroupRoleOwner,
		})
		c.Assert(err, qt.IsNil)
	}
	memberModel := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               "departing-member@test-org.com",
		Name:                "Departing Member",
		IsActive:            true,
	}
	c.Assert(memberModel.SetPassword("Password123"), qt.IsNil)
	member, err := fs.UserRegistry.Create(ctx, memberModel)
	c.Assert(err, qt.IsNil)
	_, err = memberships.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		GroupID:             fx.groupID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)
	ownerCtx := fx.ctx
	memberCtx := tagCtxFor(member, fx.groupID)

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
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		GroupID:             fx.groupID,
		UserID:              member.ID,
		Category:            "warranty",
		Enabled:             false,
	})
	c.Assert(err, qt.IsNil)

	areaID := seedTagArea(c, fx.set, ownerCtx)
	commodityID := seedTagCommodity(c, fx.set, ownerCtx, areaID, "Shared drill")
	reminders, err := fs.CommodityReminderRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	reminder, err := reminders.Create(memberCtx, newPGReminder(commodityID, "Return window", "2026-12-15"))
	c.Assert(err, qt.IsNil)

	drafts, err := fs.CommodityDraftRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	draft, _, err := drafts.CreateOrGet(memberCtx, models.CommodityDraft{ClientKey: "half-done", Values: models.CommodityDraftValues{"name": "Lamp"}})
	c.Assert(err, qt.IsNil)
	delivered, _, err := drafts.CreateOrGet(memberCtx, models.CommodityDraft{
		ClientKey: models.IntakeDeliveredDraftKeyPrefix + "paperless-7", Values: models.CommodityDraftValues{"name": "Invoice 7"},
	})
	c.Assert(err, qt.IsNil)
	photo := fx.file(c, "photo.pdf", 10, models.FileLink{EntityType: models.FileLinkDraft, EntityID: draft.ID, Meta: models.FileLinkMetaDocuments})

	intake, err := fs.IntakeRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	receipt := fx.file(c, "receipt.pdf", 10)
	session, _, err := intake.CreateSession(memberCtx,
		models.IntakeSession{ClientKey: "scan-1", Origin: models.IntakeOriginUpload},
		[]registry.IntakeSourceInput{{FileIDs: []string{receipt.ID}}}, 0)
	c.Assert(err, qt.IsNil)

	usedAt := time.Now()
	unused := mustCreateInvite(c, ctx, fs, tenantID, fx.groupID, member.ID, time.Now().Add(time.Hour), nil, nil)
	accepted := mustCreateInvite(c, ctx, fs, tenantID, fx.groupID, member.ID, time.Now().Add(time.Hour), &owner.ID, &usedAt)

	createToken := func(name string, groupID *string) *models.APIToken {
		gen, err := models.GenerateAPIToken()
		c.Assert(err, qt.IsNil)
		token, err := fs.APITokenRegistry.Create(ctx, models.APIToken{
			TenantUserAwareEntityID: models.WithTenantUserAwareEntityID("", tenantID, member.ID),
			Name:                    name,
			TokenHash:               gen.Hash,
			TokenPrefix:             gen.DisplayPrefix,
			Scope:                   models.APITokenScopeReadWrite,
			GroupID:                 groupID,
		})
		c.Assert(err, qt.IsNil)
		return token
	}
	groupToken := createToken("group", &fx.groupID)
	anyGroupToken := createToken("any group", nil)

	svc := services.NewGroupService(fs.LocationGroupRegistry, memberships, fs.GroupInviteRegistry)
	c.Assert(svc.RemoveMember(ctx, fx.groupID, member.ID), qt.IsNil)

	views := fs.SavedViewRegistryFactory.CreateServiceRegistry()
	_, err = views.Get(ctx, personal.ID)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = views.Get(ctx, memberShared.ID)
	c.Check(err, qt.IsNil)
	ownerPlacements, err := fs.SavedViewRegistryFactory.MustCreateUserRegistry(ownerCtx).Placements(ownerCtx)
	c.Assert(err, qt.IsNil)
	c.Check(ownerPlacements[memberShared.ID].ShowInSidebar, qt.IsTrue)
	var memberPlacements int
	c.Assert(fx.dbx.GetContext(ctx, &memberPlacements, `SELECT COUNT(*) FROM saved_view_placements WHERE user_id = $1`, member.ID), qt.IsNil)
	c.Check(memberPlacements, qt.Equals, 0)

	prefs, err := fs.GroupNotificationPrefRegistry.ListByUserGroup(ctx, tenantID, fx.groupID, member.ID)
	c.Assert(err, qt.IsNil)
	c.Check(prefs, qt.HasLen, 0)

	_, err = fs.CommodityReminderRegistryFactory.CreateServiceRegistry().Get(ctx, reminder.ID)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)

	serviceDrafts := fs.CommodityDraftRegistryFactory.CreateServiceRegistry()
	_, err = serviceDrafts.Get(ctx, draft.ID)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = serviceDrafts.Get(ctx, delivered.ID)
	c.Check(err, qt.IsNil)
	for _, file := range []*models.FileEntity{photo, receipt} {
		kept, err := fx.set.FileRegistry.Get(ownerCtx, file.ID)
		c.Assert(err, qt.IsNil)
		c.Check(kept.Links, qt.HasLen, 0, qt.Commentf("file %s", file.Title))
	}

	_, err = fs.IntakeRegistryFactory.CreateServiceRegistry().Get(ctx, session.ID)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)

	var invites []string
	c.Assert(fx.dbx.SelectContext(ctx, &invites, `SELECT id FROM group_invites WHERE id IN ($1, $2)`, unused, accepted), qt.IsNil)
	c.Check(invites, qt.DeepEquals, []string{accepted})

	for _, token := range []*models.APIToken{groupToken, anyGroupToken} {
		got, err := fs.APITokenRegistry.GetByTokenHash(ctx, token.TokenHash)
		c.Assert(err, qt.IsNil)
		c.Check(got.RevokedAt != nil, qt.Equals, token == groupToken, qt.Commentf("token %s", token.Name))
	}
}

// A token limited to the group that is stored while a member removal holds
// the group's membership lock waits for the removal, and is refused once the
// removal commits. Without the wait it could land after the removal revoked
// the group's tokens and work again after a re-invite.
func TestAPITokens_Postgres_GroupTokenWaitsForAMemberRemoval(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)
	ctx := context.Background()
	fs := fx.factorySet
	tenantID := fx.user.TenantID
	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)
	db := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")

	member, err := fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               "late-token-member@test-org.com",
		Name:                "Late Token Member",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	membership, err := fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		GroupID:             fx.groupID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)
	gen, err := models.GenerateAPIToken()
	c.Assert(err, qt.IsNil)

	// The removal takes the lock DeleteWithMemberInvariants takes and
	// deletes the membership, and has not committed yet.
	removal, err := db.BeginTxx(ctx, nil)
	c.Assert(err, qt.IsNil)
	defer func() { _ = removal.Rollback() }()
	_, err = removal.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('group_membership_leave'), hashtext($1))`, fx.groupID)
	c.Assert(err, qt.IsNil)
	_, err = removal.ExecContext(ctx, `DELETE FROM group_memberships WHERE id = $1`, membership.ID)
	c.Assert(err, qt.IsNil)

	created := make(chan error, 1)
	go func() {
		_, err := fs.APITokenRegistry.Create(ctx, models.APIToken{
			TenantUserAwareEntityID: models.WithTenantUserAwareEntityID("", tenantID, member.ID),
			Name:                    "late",
			TokenHash:               gen.Hash,
			TokenPrefix:             gen.DisplayPrefix,
			Scope:                   models.APITokenScopeReadOnly,
			GroupID:                 new(fx.groupID),
		})
		created <- err
	}()
	select {
	case err := <-created:
		c.Fatalf("the token was stored while the removal held the group lock (err=%v)", err)
	case <-time.After(500 * time.Millisecond):
	}

	c.Assert(removal.Commit(), qt.IsNil)
	select {
	case err := <-created:
		c.Assert(err, qt.ErrorIs, registry.ErrNotAGroupMember)
	case <-time.After(30 * time.Second):
		c.Fatal("the token create did not return after the removal committed")
	}
}
