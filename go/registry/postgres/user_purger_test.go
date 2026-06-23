package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
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
