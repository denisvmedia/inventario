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
