package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/apiserver"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register file:// driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	memreg "github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// accountDeletionTestRig bundles the wiring an account-deletion handler test
// needs: the shared memory FactorySet (so RequireAuth and DeleteAccount see the
// same user store), the refresh-token + blacklist mocks for session-teardown
// assertions, and the assembled auth route handler.
type accountDeletionTestRig struct {
	fs          *registry.FactorySet
	refreshMock *mockRefreshTokenRegistryForAuth
	blacklist   *mockTokenBlacklisterForAuth
	authHandler func(chi.Router)
	jwtSecret   []byte
}

const accountDeletionUploadLocation = "file://account-deletion-uploads?memfs=1&create_dir=1"

// newAccountDeletionRig builds the rig over a fresh memory FactorySet. The real
// memory UserRegistry backs both RequireAuth (user lookup) and DeleteAccount
// (the final delete), so the success path actually removes the row.
func newAccountDeletionRig(c *qt.C) *accountDeletionTestRig {
	c.Helper()
	fs := memreg.NewFactorySet()
	fileSvc := services.NewFileService(fs, accountDeletionUploadLocation)
	purger := services.NewGroupPurgeService(fs, fileSvc)
	deletionSvc := services.NewAccountDeletionService(fs, purger)

	refreshMock := &mockRefreshTokenRegistryForAuth{tokensByHash: map[string]*models.RefreshToken{}}
	blacklist := &mockTokenBlacklisterForAuth{}
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	authHandler := apiserver.Auth(apiserver.AuthParams{
		UserRegistry:           fs.UserRegistry,
		RefreshTokenRegistry:   refreshMock,
		BlacklistService:       blacklist,
		AccountDeletionService: deletionSvc,
		UserPurger:             fs.UserPurger,
		JWTSecret:              jwtSecret,
	})

	return &accountDeletionTestRig{
		fs:          fs,
		refreshMock: refreshMock,
		blacklist:   blacklist,
		authHandler: authHandler,
		jwtSecret:   jwtSecret,
	}
}

// seedDeletionUser inserts an active user. When password is non-empty it is set
// as the user's password (password user); when empty the user has no password
// hash (OAuth-only user, #1394).
func (rig *accountDeletionTestRig) seedDeletionUser(c *qt.C, email, password string) *models.User {
	c.Helper()
	u := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		Email:               email,
		Name:                email,
		IsActive:            true,
	}
	if password != "" {
		c.Assert(u.SetPassword(password), qt.IsNil)
	}
	created, err := rig.fs.UserRegistry.Create(context.Background(), u)
	c.Assert(err, qt.IsNil)
	return created
}

// token mints a valid access token for the user.
func (rig *accountDeletionTestRig) token(c *qt.C, userID string) string {
	c.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    userID,
		"role":       "user",
		"token_type": "access",
		"exp":        time.Now().Add(24 * time.Hour).Unix(),
	})
	signed, err := tok.SignedString(rig.jwtSecret)
	c.Assert(err, qt.IsNil)
	return signed
}

// do issues DELETE /me with the given bearer token and JSON body.
func (rig *accountDeletionTestRig) do(c *qt.C, token string, body any) *httptest.ResponseRecorder {
	c.Helper()
	b, err := json.Marshal(body)
	c.Assert(err, qt.IsNil)
	req := httptest.NewRequest(http.MethodDelete, "/me", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	router := chi.NewRouter()
	rig.authHandler(router)
	router.ServeHTTP(rr, req)
	return rr
}

// TestAuthAPI_DeleteCurrentUser_PasswordUserEmptyPassword: a password user who
// omits the password gets 400 (the re-auth is mandatory) and is NOT deleted.
func TestAuthAPI_DeleteCurrentUser_PasswordUserEmptyPassword(t *testing.T) {
	c := qt.New(t)
	rig := newAccountDeletionRig(c)
	user := rig.seedDeletionUser(c, "pw-empty@example.com", "Sup3r-Secret-Pw!")

	rr := rig.do(c, rig.token(c, user.ID), map[string]string{"password": ""})

	c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
	_, err := rig.fs.UserRegistry.Get(context.Background(), user.ID)
	c.Assert(err, qt.IsNil)
}

// TestAuthAPI_DeleteCurrentUser_WrongPassword: a password user who supplies the
// wrong password gets 422 with the dotted code auth.delete.invalid_password and
// is NOT deleted.
func TestAuthAPI_DeleteCurrentUser_WrongPassword(t *testing.T) {
	c := qt.New(t)
	rig := newAccountDeletionRig(c)
	user := rig.seedDeletionUser(c, "pw-wrong@example.com", "Sup3r-Secret-Pw!")

	rr := rig.do(c, rig.token(c, user.ID), map[string]string{"password": "not-the-password"})

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), "auth.delete.invalid_password")
	_, err := rig.fs.UserRegistry.Get(context.Background(), user.ID)
	c.Assert(err, qt.IsNil)
}

// TestAuthAPI_DeleteCurrentUser_PasswordUserSuccess: a password user who
// supplies the correct password is deleted (204), and the session is torn down
// — refresh tokens revoked for the user and the user's tokens blacklisted.
func TestAuthAPI_DeleteCurrentUser_PasswordUserSuccess(t *testing.T) {
	c := qt.New(t)
	rig := newAccountDeletionRig(c)
	user := rig.seedDeletionUser(c, "pw-ok@example.com", "Sup3r-Secret-Pw!")

	rr := rig.do(c, rig.token(c, user.ID), map[string]string{"password": "Sup3r-Secret-Pw!"})

	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	// User row is gone.
	_, err := rig.fs.UserRegistry.Get(context.Background(), user.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Session teardown: refresh tokens revoked for the user, tokens blacklisted.
	c.Assert(rig.refreshMock.revokeByUserIDCalled, qt.IsTrue)
	c.Assert(rig.refreshMock.revokeByUserIDArg, qt.Equals, user.ID)
	c.Assert(rig.blacklist.blacklistUserTokensCalled, qt.IsTrue)
	c.Assert(rig.blacklist.blacklistUserTokensUserID, qt.Equals, user.ID)
}

// TestAuthAPI_DeleteCurrentUser_OAuthOnlyUserSuccess: an OAuth-only user (empty
// PasswordHash) is deleted (204) with an empty password — the password re-check
// is skipped, mirroring handleChangePassword's initial-set branch.
func TestAuthAPI_DeleteCurrentUser_OAuthOnlyUserSuccess(t *testing.T) {
	c := qt.New(t)
	rig := newAccountDeletionRig(c)
	user := rig.seedDeletionUser(c, "oauth-only@example.com", "")
	c.Assert(user.PasswordHash, qt.Equals, "")

	rr := rig.do(c, rig.token(c, user.ID), map[string]string{"password": ""})

	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
	_, err := rig.fs.UserRegistry.Get(context.Background(), user.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}
