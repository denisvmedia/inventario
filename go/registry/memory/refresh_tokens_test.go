package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

// TestRefreshTokenRegistryMemory_GetByTokenHash_ReturnsRevokedRow pins, for the
// in-memory backend, the same property the postgres suite pins: GetByTokenHash
// returns a row even after it has been revoked (it does NOT filter on
// RevokedAt). The #967 reuse-detection handler relies on this so a
// replayed-after-rotation cookie surfaces the revoked row rather than
// ErrNotFound — keeping the two backends in lockstep on the one property the
// theft cascade depends on.
func TestRefreshTokenRegistryMemory_GetByTokenHash_ReturnsRevokedRow(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	r := memory.NewRefreshTokenRegistry()

	_, hash, err := models.GenerateRefreshToken()
	c.Assert(err, qt.IsNil)

	created, err := r.Create(ctx, models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: "tenant-1",
			UserID:   "user-1",
		},
		TokenHash: hash,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created.RevokedAt, qt.IsNil)

	err = r.RevokeByID(ctx, "user-1", created.ID)
	c.Assert(err, qt.IsNil)

	got, err := r.GetByTokenHash(ctx, hash)
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, created.ID)
	c.Assert(got.RevokedAt, qt.IsNotNil)
}
