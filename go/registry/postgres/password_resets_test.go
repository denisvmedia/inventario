package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// TestPasswordResetRegistry_Update_NotFound pins parity with the memory
// backend: Update against an unknown ID returns registry.ErrNotFound rather
// than silently succeeding with a zero-row UPDATE. See #1814.
func TestPasswordResetRegistry_Update_NotFound(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	pr := models.PasswordReset{
		UserID:    user.ID,
		TenantID:  user.TenantID,
		Token:     "token-update-missing",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	pr.ID = "no-such-id"
	_, err := registrySet.PasswordResetRegistry.Update(ctx, pr)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}
