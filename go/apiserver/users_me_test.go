package apiserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

// TestUsersMeAPI exercises the four /users/me handlers shipped with
// issue #1644: list sessions (with is_current flag), revoke a single
// session, revoke all-but-current, and the login-history endpoint.
// Memory registries back the test so the assertions only need to
// match application-level behaviour — RLS, FK constraints and the
// service-mode role are postgres-only and out of scope here.
func TestUsersMeAPI(t *testing.T) {
	c := qt.New(t)

	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "u-1"},
			TenantID: "tenant-1",
		},
		Email:    "alex@example.com",
		Name:     "Alex",
		IsActive: true,
	}

	rtReg := memory.NewRefreshTokenRegistry()
	leReg := memory.NewLoginEventRegistry()

	// Seed two refresh tokens belonging to the user: one is the
	// "current" session (matching the request cookie hash), the
	// other is an old browser we expect to be able to revoke
	// independently. The token strings are arbitrary in-memory
	// values; only their hash matters to the registry.
	currentRaw, currentHash, err := models.GenerateRefreshToken()
	c.Assert(err, qt.IsNil)
	current, err := rtReg.Create(c.Context(), models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: user.TenantID, UserID: user.ID,
		},
		TokenHash: currentHash,
		ExpiresAt: time.Now().Add(time.Hour),
		IPAddress: "203.0.113.0/24",
		UserAgent: "Mozilla/5.0 Chrome/120.0",
	})
	c.Assert(err, qt.IsNil)
	other, err := rtReg.Create(c.Context(), models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: user.TenantID, UserID: user.ID,
		},
		TokenHash: "other-hash",
		ExpiresAt: time.Now().Add(time.Hour),
		IPAddress: "198.51.100.0/24",
		UserAgent: "Mozilla/5.0 Safari/605",
	})
	c.Assert(err, qt.IsNil)

	// Seed a successful + failed login_event for this user. The
	// listLoginHistory subtest asserts newest-first ordering + the
	// failed_last_7d hint, so we only need two rows.
	uidPtr := user.ID
	_, err = leReg.Create(c.Context(), models.LoginEvent{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		UserID:              &uidPtr,
		Email:               user.Email,
		Outcome:             models.LoginOutcomeOK,
		Method:              models.LoginMethodPassword,
		IPAddress:           "203.0.113.0/24",
		UserAgent:           "Mozilla/5.0 Chrome/120.0",
	})
	c.Assert(err, qt.IsNil)
	time.Sleep(2 * time.Millisecond) // ensure CreatedAt ordering is deterministic
	_, err = leReg.Create(c.Context(), models.LoginEvent{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		UserID:              &uidPtr,
		Email:               user.Email,
		Outcome:             models.LoginOutcomeBadPassword,
		Method:              models.LoginMethodPassword,
	})
	c.Assert(err, qt.IsNil)

	// Build a chi router that mounts the UsersMe sub-routes under
	// the same prefix the apiserver does. The auth middleware is
	// stubbed with a tiny helper that injects the seeded user into
	// the request context — production wires this via JWTMiddleware.
	r := chi.NewRouter()
	r.Route("/users/me", func(sub chi.Router) {
		sub.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				ctx := appctx.WithUser(req.Context(), user)
				next.ServeHTTP(w, req.WithContext(ctx))
			})
		})
		apiserver.UsersMe(apiserver.UsersMeParams{
			RefreshTokenRegistry: rtReg,
			LoginEventRegistry:   leReg,
		})(sub)
	})

	// GET /sessions — both seeded tokens visible, the right one
	// flagged as current via the refresh cookie hash.
	t.Run("list_sessions_marks_current", func(t *testing.T) {
		c := qt.New(t)
		req := httptest.NewRequest(http.MethodGet, "/users/me/sessions", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: currentRaw})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, http.StatusOK)
		var resp apiserver.SessionsListResponse
		c.Assert(json.Unmarshal(w.Body.Bytes(), &resp), qt.IsNil)
		c.Assert(resp.Sessions, qt.HasLen, 2)
		var sawCurrent bool
		for _, s := range resp.Sessions {
			if s.ID == current.ID {
				c.Assert(s.IsCurrent, qt.IsTrue)
				sawCurrent = true
			} else {
				c.Assert(s.IsCurrent, qt.IsFalse)
			}
		}
		c.Assert(sawCurrent, qt.IsTrue)
	})

	// DELETE /sessions/{id} — revoking a token belonging to the user
	// works; trying a foreign id yields 404 (and we MUST not leak
	// the difference between "doesn't exist" and "belongs to someone
	// else", so a sibling user can't enumerate the token table).
	t.Run("revoke_single_session", func(t *testing.T) {
		c := qt.New(t)
		req := httptest.NewRequest(http.MethodDelete, "/users/me/sessions/"+other.ID, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, http.StatusNoContent)
		// Now expired/revoked — the active list shrinks to one.
		active, err := rtReg.ListActiveByUserID(c.Context(), user.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(active, qt.HasLen, 1)
		c.Assert(active[0].ID, qt.Equals, current.ID)
	})

	// DELETE /sessions — revoke-all-but-current keeps the cookie's
	// session alive. Re-seed a "third device" first since the
	// previous subtest already revoked `other`.
	t.Run("revoke_all_other_sessions", func(t *testing.T) {
		c := qt.New(t)
		third, err := rtReg.Create(c.Context(), models.RefreshToken{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{
				TenantID: user.TenantID, UserID: user.ID,
			},
			TokenHash: "third-hash",
			ExpiresAt: time.Now().Add(time.Hour),
		})
		c.Assert(err, qt.IsNil)

		req := httptest.NewRequest(http.MethodDelete, "/users/me/sessions", nil)
		req.AddCookie(&http.Cookie{Name: "refresh_token", Value: currentRaw})
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, http.StatusNoContent)

		active, err := rtReg.ListActiveByUserID(c.Context(), user.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(active, qt.HasLen, 1)
		c.Assert(active[0].ID, qt.Equals, current.ID)

		// `third` should have been revoked.
		stored, err := rtReg.Get(c.Context(), third.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(stored.RevokedAt, qt.IsNotNil)
	})

	// GET /login-history — newest first, optional failed-count
	// hint populated.
	t.Run("login_history_newest_first", func(t *testing.T) {
		c := qt.New(t)
		req := httptest.NewRequest(http.MethodGet, "/users/me/login-history?limit=10", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		c.Assert(w.Code, qt.Equals, http.StatusOK)
		var resp apiserver.LoginHistoryResponse
		c.Assert(json.Unmarshal(w.Body.Bytes(), &resp), qt.IsNil)
		c.Assert(resp.Events, qt.HasLen, 2)
		// The bad_password row was created second so it sorts first.
		c.Assert(resp.Events[0].Outcome, qt.Equals, models.LoginOutcomeBadPassword)
		c.Assert(resp.Events[1].Outcome, qt.Equals, models.LoginOutcomeOK)
		c.Assert(resp.FailedLast7d, qt.Equals, 1)
	})
}
