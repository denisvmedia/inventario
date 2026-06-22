package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// TestRefreshTokenRegistryPostgres_GetByTokenHash_ReturnsRevokedRow pins the
// property the #967 reuse-detection handler depends on: GetByTokenHash returns
// a row even after it has been revoked (it does NOT filter on
// revoked_at IS NULL). Without this, a replayed-after-rotation cookie would
// surface as ErrNotFound and the theft cascade could never fire. Exercised in
// service mode (RLS-bypass) because /auth/refresh resolves the row in service
// mode.
func TestRefreshTokenRegistryPostgres_GetByTokenHash_ReturnsRevokedRow(t *testing.T) {
	c := qt.New(t)

	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	dbx := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	fs := postgres.NewFactorySet(dbx)
	serviceSet := fs.CreateServiceRegistrySet()
	r := serviceSet.RefreshTokenRegistry

	ctx := context.Background()
	_, hash, err := models.GenerateRefreshToken()
	c.Assert(err, qt.IsNil)

	created, err := r.Create(ctx, models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		TokenHash: hash,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created.RevokedAt, qt.IsNil)

	// Revoke the row.
	err = r.RevokeByID(ctx, user.ID, created.ID)
	c.Assert(err, qt.IsNil)

	// GetByTokenHash must still return the row, now carrying RevokedAt — the
	// in-handler theft discriminator.
	got, err := r.GetByTokenHash(ctx, hash)
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, created.ID)
	c.Assert(got.RevokedAt, qt.IsNotNil)
}
