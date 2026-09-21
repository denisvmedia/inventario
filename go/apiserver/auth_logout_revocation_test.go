package apiserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/models"
)

const (
	logoutRefreshCookie = "refresh_token"
	// The pre-#1785 impersonation marker. Cross-plane impersonation no
	// longer plants it; logout still has to recognize the shape so an
	// old cookie is dropped rather than hashed against the registry.
	legacyImpersonationMarker = "imp:"
)

// logoutFixture wires an auth router with a refresh-token registry
// holding one live row for `rawToken`.
func logoutFixture(c *qt.C, rawToken, userID string) (http.Handler, *mockRefreshTokenRegistryForAuth) {
	c.Helper()

	refreshMock := &mockRefreshTokenRegistryForAuth{tokensByHash: map[string]*models.RefreshToken{}}
	hash := models.HashRefreshToken(rawToken)
	refreshMock.tokensByHash[hash] = &models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			EntityID: models.EntityID{ID: "rt-1"},
			TenantID: "tenant-1",
			UserID:   userID,
		},
		TokenHash: hash,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	router := chi.NewRouter()
	apiserver.Auth(apiserver.AuthParams{
		UserRegistry:         &mockUserRegistryForAuth{users: map[string]*models.User{}},
		RefreshTokenRegistry: refreshMock,
		JWTSecret:            []byte("test-secret-32-bytes-minimum-length"),
	})(router)
	return router, refreshMock
}

// postLogout sends the raw cookie header a browser would send. Only the
// name and value travel on a request; Secure, HttpOnly and SameSite are
// response-side attributes and have no place here.
func postLogout(c *qt.C, handler http.Handler, cookieValue *string) *httptest.ResponseRecorder {
	c.Helper()

	req := httptest.NewRequest("POST", "/logout", nil)
	if cookieValue != nil {
		req.Header.Set("Cookie", logoutRefreshCookie+"="+*cookieValue)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// A session that is logged out has to stop working. Clearing the cookie
// is not enough on its own — anyone holding the raw token could still
// exchange it — so the row behind it is what must be revoked.
func TestLogout_RevokesTheRefreshTokenRow(t *testing.T) {
	c := qt.New(t)

	const rawToken = "raw-refresh-token-value"
	handler, refreshMock := logoutFixture(c, rawToken, "user-1")

	stored := refreshMock.tokensByHash[models.HashRefreshToken(rawToken)]
	c.Assert(stored.RevokedAt, qt.IsNil)

	rr := postLogout(c, handler, new(rawToken))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	c.Assert(stored.RevokedAt, qt.IsNotNil)
	c.Assert(refreshMock.updates, qt.HasLen, 1)
	c.Assert(refreshMock.updates[0].ID, qt.Equals, "rt-1")
}

// Logout is deliberately unauthenticated so an expired access token can
// still end a session. That makes the cookie the only input, and every
// shape it can arrive in has to leave the handler returning 200 rather
// than 500.
func TestLogout_TolerantOfTheCookieItGets(t *testing.T) {
	tests := []struct {
		name        string
		cookieValue *string
	}{
		{name: "no cookie at all", cookieValue: nil},
		{name: "a token that matches no row", cookieValue: new("never-issued")},
		{name: "an empty value", cookieValue: new("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			const rawToken = "raw-refresh-token-value"
			handler, refreshMock := logoutFixture(c, rawToken, "user-1")

			rr := postLogout(c, handler, tt.cookieValue)
			c.Assert(rr.Code, qt.Equals, http.StatusOK)

			var body apiserver.LogoutResponse
			c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
			c.Assert(body.Message, qt.Equals, "Logged out successfully")

			// Someone else's live session is not collateral.
			c.Assert(refreshMock.tokensByHash[models.HashRefreshToken(rawToken)].RevokedAt, qt.IsNil)
			c.Assert(refreshMock.updates, qt.HasLen, 0)
		})
	}
}

// The legacy marker is not a token. Hashing it would look up a row that
// cannot exist, and the point of recognizing the shape is to skip that
// lookup rather than to rely on it missing.
func TestLogout_DropsTheLegacyImpersonationMarker(t *testing.T) {
	c := qt.New(t)

	const rawToken = "raw-refresh-token-value"
	handler, refreshMock := logoutFixture(c, rawToken, "user-1")

	rr := postLogout(c, handler, new(legacyImpersonationMarker+"some-jti"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(refreshMock.updates, qt.HasLen, 0)
	c.Assert(refreshMock.tokensByHash[models.HashRefreshToken(rawToken)].RevokedAt, qt.IsNil)
}

// A value that merely contains the marker is a token, not a marker. The
// prefix check has to be anchored or a token would silently skip
// revocation.
func TestLogout_MarkerCheckIsAPrefix(t *testing.T) {
	c := qt.New(t)

	const rawToken = "x-imp:-not-a-marker"
	handler, refreshMock := logoutFixture(c, rawToken, "user-1")

	rr := postLogout(c, handler, new(rawToken))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(refreshMock.tokensByHash[models.HashRefreshToken(rawToken)].RevokedAt, qt.IsNotNil)
}

// The browser has to stop sending the cookie, and the replacement has to
// carry the same flags as the one it replaces — a cleared cookie without
// them is a cleared cookie a script can read on the way out.
func TestLogout_ClearsTheCookieWithItsFlags(t *testing.T) {
	c := qt.New(t)

	const rawToken = "raw-refresh-token-value"
	handler, _ := logoutFixture(c, rawToken, "user-1")

	rr := postLogout(c, handler, new(rawToken))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var cleared *http.Cookie
	for _, cookie := range rr.Result().Cookies() {
		if cookie.Name == logoutRefreshCookie {
			cleared = cookie
		}
	}
	c.Assert(cleared, qt.IsNotNil)
	c.Check(cleared.Value, qt.Equals, "")
	c.Check(cleared.MaxAge, qt.Equals, -1)
	c.Check(cleared.HttpOnly, qt.IsTrue)
	c.Check(cleared.SameSite, qt.Equals, http.SameSiteStrictMode)
	c.Check(cleared.Path, qt.Equals, "/api/v1")
	// Plain HTTP in the test; Secure follows the request's scheme so local
	// dev over http still receives the clear.
	c.Check(cleared.Secure, qt.IsFalse)
}

// Logging out twice is something a user with two tabs does. The second
// one must not resurrect or re-stamp the row.
func TestLogout_IsIdempotent(t *testing.T) {
	c := qt.New(t)

	const rawToken = "raw-refresh-token-value"
	handler, refreshMock := logoutFixture(c, rawToken, "user-1")
	cookie := new(rawToken)

	c.Assert(postLogout(c, handler, cookie).Code, qt.Equals, http.StatusOK)
	firstRevokedAt := *refreshMock.tokensByHash[models.HashRefreshToken(rawToken)].RevokedAt

	c.Assert(postLogout(c, handler, cookie).Code, qt.Equals, http.StatusOK)
	secondRevokedAt := *refreshMock.tokensByHash[models.HashRefreshToken(rawToken)].RevokedAt

	c.Assert(secondRevokedAt.After(firstRevokedAt), qt.IsFalse,
		qt.Commentf("the second logout moved revoked_at from %s to %s", firstRevokedAt, secondRevokedAt))
}
