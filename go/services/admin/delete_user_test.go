package admin_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/registry"
)

// Deleting a user from the CLI deletes their profile photo too (#2894): a
// photo left in storage after the users row is gone is a personal image
// nobody can see or remove.
func TestService_DeleteUser_DeletesTheProfilePhoto(t *testing.T) {
	c := qt.New(t)
	env := newMoveEnv(c)
	photo := env.storePhoto(c)
	c.Assert(env.blobExists(c, photo), qt.IsTrue)

	c.Assert(env.svc.DeleteUser(env.ctx, env.user.Email), qt.IsNil)

	_, err := env.fs.UserRegistry.Get(env.ctx, env.user.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(env.blobExists(c, photo), qt.IsFalse)
}
