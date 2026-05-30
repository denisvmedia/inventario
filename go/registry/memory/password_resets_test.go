package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// TestPasswordResetRegistry_Update_NotFound pins parity with the postgres
// backend: Update against an unknown ID returns registry.ErrNotFound rather
// than silently succeeding. See #1814.
func TestPasswordResetRegistry_Update_NotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewPasswordResetRegistry()

	pr := models.PasswordReset{
		UserID:    "user-1",
		TenantID:  "tenant-1",
		Token:     "token-update-missing",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	pr.ID = "no-such-id"
	_, err := r.Update(ctx, pr)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
