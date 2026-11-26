package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
)

// TestUserPurger_Postgres_PurgesAuthRowsKeepsContent is the #2116 regression:
// PurgeUserDependents must hard-delete a user's auth / identity rows (refresh
// tokens, MFA secret) so the eventual DELETE FROM users isn't blocked by the
// NO ACTION child FKs, while LEAVING the user's authored content rows in place
// (commodities/areas/locations carry NOT NULL user_id + NOT NULL created_by,
// so orphaning is impossible — the purger must not touch them, and the
// orchestration layer reassigns ownership before the final user delete).
func TestUserPurger_Postgres_PurgesAuthRowsKeepsContent(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	fs := postgres.NewFactorySet(dbx)

	user := getTestUser(c, set)
	tenantID := user.TenantID
	userID := user.ID

	ctx := appctx.WithUser(context.Background(), user)

	// -- seed auth/identity rows the purge must remove --------------------

	_, err = fs.RefreshTokenRegistry.Create(ctx, models.RefreshToken{
		TenantUserAwareEntityID: models.WithTenantUserAwareEntityID("", tenantID, userID),
		TokenHash:               "hash-user-a",
		ExpiresAt:               time.Now().Add(24 * time.Hour),
	})
	c.Assert(err, qt.IsNil)

	_, err = fs.UserMFASecretRegistry.Create(ctx, models.UserMFASecret{
		TenantUserAwareEntityID: models.WithTenantUserAwareEntityID("", tenantID, userID),
		SecretEncrypted:         "encrypted-secret",
		BackupCodesHashed:       models.ValuerSlice[string]{},
	})
	c.Assert(err, qt.IsNil)

	// -- seed authored content that must SURVIVE the purge ----------------

	areaID := seedTagArea(c, set, ctx)
	commodityID := seedTagCommodity(c, set, ctx, areaID, "Owned Drill")

	// -- a different user whose auth rows must stay untouched --------------

	otherUserModel := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               "other@test-org.com",
		Name:                "Other User",
		IsActive:            true,
	}
	c.Assert(otherUserModel.SetPassword("Password123"), qt.IsNil)
	otherUser, err := fs.UserRegistry.Create(ctx, otherUserModel)
	c.Assert(err, qt.IsNil)

	_, err = fs.RefreshTokenRegistry.Create(ctx, models.RefreshToken{
		TenantUserAwareEntityID: models.WithTenantUserAwareEntityID("", tenantID, otherUser.ID),
		TokenHash:               "hash-other",
		ExpiresAt:               time.Now().Add(24 * time.Hour),
	})
	c.Assert(err, qt.IsNil)

	// -- purge user A -----------------------------------------------------

	err = fs.UserPurger.PurgeUserDependents(context.Background(), tenantID, userID)
	c.Assert(err, qt.IsNil)

	// User A's auth rows are gone.
	tokensA, err := fs.RefreshTokenRegistry.GetByUserID(context.Background(), userID)
	c.Assert(err, qt.IsNil)
	c.Assert(tokensA, qt.HasLen, 0)

	_, err = fs.UserMFASecretRegistry.GetByUser(context.Background(), tenantID, userID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// User A's authored content STILL EXISTS — orphaning is schema-impossible
	// (NOT NULL created_by), so the purger leaves it for ownership transfer.
	serviceSet := fs.CreateServiceRegistrySet()
	commodity, err := serviceSet.CommodityRegistry.Get(context.Background(), commodityID)
	c.Assert(err, qt.IsNil)
	c.Assert(commodity.GetID(), qt.Equals, commodityID)

	// User B's auth rows are untouched.
	tokensB, err := fs.RefreshTokenRegistry.GetByUserID(context.Background(), otherUser.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(tokensB, qt.HasLen, 1)

	// Idempotent: a second purge after the rows are gone is a clean no-op.
	err = fs.UserPurger.PurgeUserDependents(context.Background(), tenantID, userID)
	c.Assert(err, qt.IsNil)
}

// TestUserPurger_Postgres_PurgesTheUsersDrafts covers #2812: the drafts a
// user created go with the account, together with every file link to them,
// so the files stay as standalone files. created_by_user_id is a NOT NULL FK
// with NO ACTION, so a draft left behind would block the final users delete.
func TestUserPurger_Postgres_PurgesTheUsersDrafts(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)

	drafts, err := fx.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(fx.ctx)
	c.Assert(err, qt.IsNil)
	draft, _, err := drafts.CreateOrGet(fx.ctx, models.CommodityDraft{ClientKey: "leaving", Values: models.CommodityDraftValues{"name": "Half done"}})
	c.Assert(err, qt.IsNil)
	photo := fx.file(c, "photo.pdf", 10, models.FileLink{EntityType: models.FileLinkDraft, EntityID: draft.ID, Meta: models.FileLinkMetaDocuments})

	c.Assert(fx.factorySet.UserPurger.PurgeUserDependents(context.Background(), fx.user.TenantID, fx.user.ID), qt.IsNil)

	_, err = fx.factorySet.CommodityDraftRegistryFactory.CreateServiceRegistry().Get(context.Background(), draft.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	kept, err := fx.set.FileRegistry.Get(fx.ctx, photo.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(kept.Links, qt.HasLen, 0)
}

// TestUserPurger_Postgres_PurgesTheUsersIntakeSessions covers #2810: the
// intake sessions a user created go with the account, with their sources,
// candidates and the links of their files. The files stay as standalone
// files. created_by_user_id is a NOT NULL FK with NO ACTION on all three
// tables, so a row left behind would block the final users delete.
func TestUserPurger_Postgres_PurgesTheUsersIntakeSessions(t *testing.T) {
	c := qt.New(t)
	fx := newFileLinkPGFixture(t)

	intake, err := fx.factorySet.IntakeRegistryFactory.CreateUserRegistry(fx.ctx)
	c.Assert(err, qt.IsNil)
	receipt := fx.file(c, "receipt.pdf", 10)
	session, _, err := intake.CreateSession(fx.ctx,
		models.IntakeSession{ClientKey: "leaving", Origin: models.IntakeOriginUpload},
		[]registry.IntakeSourceInput{{FileIDs: []string{receipt.ID}}}, 0)
	c.Assert(err, qt.IsNil)
	linked, err := fx.set.FileRegistry.Get(fx.ctx, receipt.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(linked.Links, qt.HasLen, 1)
	// The record of a document the user's token delivered goes with them
	// when the group has no other member to take it.
	doc, _, err := fx.set.IntakeDocumentRegistry.CreateOrGet(fx.ctx, models.IntakeDocument{
		ExternalSystem: "paperless-ngx", ExternalDocumentID: "7", PayloadHash: "h",
	})
	c.Assert(err, qt.IsNil)

	c.Assert(fx.factorySet.UserPurger.PurgeUserDependents(context.Background(), fx.user.TenantID, fx.user.ID), qt.IsNil)

	_, err = fx.factorySet.IntakeRegistryFactory.CreateServiceRegistry().Get(context.Background(), session.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = fx.factorySet.IntakeDocumentRegistryFactory.CreateServiceRegistry().Get(context.Background(), doc.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	kept, err := fx.set.FileRegistry.Get(fx.ctx, receipt.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(kept.Links, qt.HasLen, 0)
}

// TestUserPurger_Postgres_HandsOverDeliveredDocuments covers #2809: the
// record of a document another system delivered through the user's API token
// is group content. It goes to the group's owner with the draft it made, so a
// replay of the delivery still finds both, and the users row can go.
func TestUserPurger_Postgres_HandsOverDeliveredDocuments(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := appctx.WithUser(context.Background(), fx.user)
	tenantID := fx.user.TenantID

	memberModel := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               "intake-integrator@test-org.com",
		Name:                "Integrator",
		IsActive:            true,
	}
	c.Assert(memberModel.SetPassword("Password123"), qt.IsNil)
	member, err := fx.factorySet.UserRegistry.Create(ctx, memberModel)
	c.Assert(err, qt.IsNil)
	memberships := fx.factorySet.GroupMembershipRegistry
	if _, err := memberships.GetByGroupAndUser(ctx, fx.groupAID, fx.user.ID); err != nil {
		_, err = memberships.Create(ctx, models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
			GroupID:             fx.groupAID,
			MemberUserID:        fx.user.ID,
			Role:                models.GroupRoleOwner,
		})
		c.Assert(err, qt.IsNil)
	}
	_, err = memberships.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		GroupID:             fx.groupAID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	memberSet := postgres.NewRegistrySetWithUserAndGroupID(fx.dbx, member.ID, tenantID, fx.groupAID)
	doc, _, err := memberSet.IntakeDocumentRegistry.CreateOrGet(tagCtxFor(member, fx.groupAID), models.IntakeDocument{
		ExternalSystem: "paperless-ngx", ExternalDocumentID: "7", PayloadHash: "h",
	})
	c.Assert(err, qt.IsNil)
	c.Assert(doc.CreatedByUserID, qt.Equals, member.ID)
	memberCtx := tagCtxFor(member, fx.groupAID)
	delivered, _, err := memberSet.CommodityDraftRegistry.CreateOrGet(memberCtx, models.CommodityDraft{
		ClientKey: doc.DraftClientKey(), Values: models.CommodityDraftValues{"name": "Invoice 7"},
	})
	c.Assert(err, qt.IsNil)
	own, _, err := memberSet.CommodityDraftRegistry.CreateOrGet(memberCtx, models.CommodityDraft{
		ClientKey: "form-1", Values: models.CommodityDraftValues{"name": "Lamp"},
	})
	c.Assert(err, qt.IsNil)

	c.Assert(fx.factorySet.UserPurger.PurgeUserDependents(t.Context(), tenantID, member.ID), qt.IsNil)
	got, err := fx.factorySet.IntakeDocumentRegistryFactory.CreateServiceRegistry().Get(t.Context(), doc.ID)
	c.Assert(err, qt.IsNil)
	c.Check(got.CreatedByUserID, qt.Equals, fx.user.ID)
	// The draft the document made goes with it; the member's own draft goes
	// with the member.
	drafts := fx.factorySet.CommodityDraftRegistryFactory.CreateServiceRegistry()
	kept, err := drafts.Get(t.Context(), delivered.ID)
	c.Assert(err, qt.IsNil)
	c.Check(kept.CreatedByUserID, qt.Equals, fx.user.ID)
	_, err = drafts.Get(t.Context(), own.ID)
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	c.Check(fx.factorySet.UserRegistry.Delete(ctx, member.ID), qt.IsNil)
}

// TestUserPurger_Postgres_ClearsGroupInvitesAuditReferences is the #2147
// regression: group_invites_audit.created_by and .used_by are NOT NULL FK ->
// users(id) with NO ACTION, so a user whose group ever had a USED invite would
// block the final DELETE FROM users. PurgeUserDependents must clear those audit
// rows (matching either column). It seeds one audit row that names the user as
// BOTH creator and accepter, and a second referencing a different user that must
// survive.
func TestUserPurger_Postgres_ClearsGroupInvitesAuditReferences(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	fs := postgres.NewFactorySet(dbx)

	user := getTestUser(c, set)
	tenantID := user.TenantID
	userID := user.ID

	ctx := appctx.WithUser(context.Background(), user)

	// A second user whose audit row must survive the purge.
	otherModel := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               "audit-other@test-org.com",
		Name:                "Audit Other",
		IsActive:            true,
	}
	c.Assert(otherModel.SetPassword("Password123"), qt.IsNil)
	other, err := fs.UserRegistry.Create(ctx, otherModel)
	c.Assert(err, qt.IsNil)

	now := time.Now()

	// Audit row referencing the user as BOTH creator and accepter.
	userAudit, err := fs.GroupInviteAuditRegistry.Create(ctx, models.GroupInviteAudit{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		OriginalInviteID:    "orig-invite-user",
		OriginalInviteUUID:  "orig-uuid-user",
		OriginalGroupID:     "orig-group-user",
		OriginalGroupSlug:   "orig-group-user-slug",
		OriginalGroupName:   "Orig Group User",
		Token:               "token-user",
		CreatedBy:           userID,
		UsedBy:              userID,
		OriginalCreatedAt:   now.Add(-time.Hour),
		OriginalExpiresAt:   now.Add(time.Hour),
		UsedAt:              now,
	})
	c.Assert(err, qt.IsNil)

	// Audit row referencing the OTHER user — must be untouched.
	otherAudit, err := fs.GroupInviteAuditRegistry.Create(ctx, models.GroupInviteAudit{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		OriginalInviteID:    "orig-invite-other",
		OriginalInviteUUID:  "orig-uuid-other",
		OriginalGroupID:     "orig-group-other",
		OriginalGroupSlug:   "orig-group-other-slug",
		OriginalGroupName:   "Orig Group Other",
		Token:               "token-other",
		CreatedBy:           other.ID,
		UsedBy:              other.ID,
		OriginalCreatedAt:   now.Add(-time.Hour),
		OriginalExpiresAt:   now.Add(time.Hour),
		UsedAt:              now,
	})
	c.Assert(err, qt.IsNil)

	// Single-column matches: the purge must remove a row where the user is the
	// creator OR the accepter, not only when BOTH columns match. These guard
	// against a regression to `created_by = $2 AND used_by = $2`.
	createdByOnly, err := fs.GroupInviteAuditRegistry.Create(ctx, models.GroupInviteAudit{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		OriginalInviteID:    "orig-invite-created-only",
		OriginalInviteUUID:  "orig-uuid-created-only",
		OriginalGroupID:     "orig-group-created-only",
		OriginalGroupSlug:   "orig-group-created-only-slug",
		OriginalGroupName:   "Orig Group Created Only",
		Token:               "token-created-only",
		CreatedBy:           userID,
		UsedBy:              other.ID,
		OriginalCreatedAt:   now.Add(-time.Hour),
		OriginalExpiresAt:   now.Add(time.Hour),
		UsedAt:              now,
	})
	c.Assert(err, qt.IsNil)

	usedByOnly, err := fs.GroupInviteAuditRegistry.Create(ctx, models.GroupInviteAudit{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		OriginalInviteID:    "orig-invite-used-only",
		OriginalInviteUUID:  "orig-uuid-used-only",
		OriginalGroupID:     "orig-group-used-only",
		OriginalGroupSlug:   "orig-group-used-only-slug",
		OriginalGroupName:   "Orig Group Used Only",
		Token:               "token-used-only",
		CreatedBy:           other.ID,
		UsedBy:              userID,
		OriginalCreatedAt:   now.Add(-time.Hour),
		OriginalExpiresAt:   now.Add(time.Hour),
		UsedAt:              now,
	})
	c.Assert(err, qt.IsNil)

	// Purge the user.
	err = fs.UserPurger.PurgeUserDependents(context.Background(), tenantID, userID)
	c.Assert(err, qt.IsNil)

	// The user's audit rows are gone — both the both-columns match and each
	// single-column (created_by-only / used_by-only) match; the other user's
	// row survives.
	_, err = fs.GroupInviteAuditRegistry.Get(context.Background(), userAudit.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = fs.GroupInviteAuditRegistry.Get(context.Background(), createdByOnly.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = fs.GroupInviteAuditRegistry.Get(context.Background(), usedByOnly.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	surviving, err := fs.GroupInviteAuditRegistry.Get(context.Background(), otherAudit.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(surviving.ID, qt.Equals, otherAudit.ID)

	// Idempotent: a second purge is a clean no-op.
	err = fs.UserPurger.PurgeUserDependents(context.Background(), tenantID, userID)
	c.Assert(err, qt.IsNil)
}
