package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/checkers"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services"
)

// Block + unblock handler tests (#1747). Every AC in the issue spec
// maps to one (or more) of the subtests below. Tests live alongside
// admin_routes_test.go and reuse its `promoteToSystemAdmin` helper to
// flip the IsSystemAdmin flag on a fixture user without standing up a
// second tenant.

// adminTestEnv bundles the moving parts every block/unblock test needs
// so the per-test setup stays one line. Standing this up via a helper
// keeps the table-driven cases focused on inputs + expectations rather
// than ceremony.
type adminTestEnv struct {
	params  apiserver.Params
	handler http.Handler
	admin   *models.User
}

// newAdminEnv returns a Params with a single system-admin fixture user
// (the test's "actor") and the assembled HTTP handler. The admin user
// returned is the one whose auth header should be attached to admin
// requests — promote a separate fixture (via createSecondUser) when
// the test needs a distinct subject.
func newAdminEnv(c *qt.C) adminTestEnv {
	c.Helper()
	params, user, _ := newParams()
	promoteToSystemAdmin(c, params, user)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	return adminTestEnv{
		params:  params,
		handler: handler,
		admin:   user,
	}
}

// createTestUserDirect creates an additional user fixture in the given
// tenant. The CLI-side admin service's CreateUser would be overkill for
// tests — use the registry directly so a single helper covers both
// "second admin" and "ordinary user". Callers pass the tenant ID
// explicitly so the helper does not silently pick `tenants[0]` when the
// fixture set contains more than one tenant.
func createTestUserDirect(c *qt.C, params apiserver.Params, tenantID, email string, isActive, isSystemAdmin bool) *models.User {
	c.Helper()
	c.Assert(tenantID, qt.Not(qt.Equals), "", qt.Commentf("createTestUserDirect: tenantID is required"))
	u := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               email,
		Name:                email,
		IsActive:            isActive,
		IsSystemAdmin:       isSystemAdmin,
	}
	must.Assert(u.SetPassword("Password123"))
	created := must.Must(params.FactorySet.UserRegistry.Create(context.Background(), u))
	return created
}

// blockBody builds a POST /block JSON body. Tests that want to omit
// the reason or send `force=true` pass the values through directly so
// the table-driven flavor stays readable.
func blockBody(reason string, force bool) map[string]any {
	return map[string]any{"reason": reason, "force": force}
}

// unblockBody mirrors blockBody for the unblock endpoint. Kept as a
// separate constructor so the test cases don't accidentally smuggle a
// `force` field into unblock — the unblock DTO has no such field and
// DisallowUnknownFields would reject it.
func unblockBody(reason string) map[string]any {
	return map[string]any{"reason": reason}
}

// doAdminJSONRequest is the admin-side twin of doJSONAPIRequest. We
// don't use doJSONAPIRequest itself because /admin/* sets a plain
// "application/json" content type (mirrors users_me); the JSON:API
// variant in doJSONAPIRequest would still work but the explicit JSON
// content type matches what the FE sends and keeps the test honest.
//
// `body` is encoded by switching on its concrete type:
//   - nil → no body
//   - []byte / json.RawMessage / string → passed through verbatim
//     (lets tests send malformed JSON or trailing-token payloads that
//     `json.Marshal` would otherwise sanitise into valid JSON)
//   - anything else → `json.Marshal`
func doAdminJSONRequest(t *testing.T, handler http.Handler, method, path, userID string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	switch v := body.(type) {
	case nil:
		// no body
	case []byte:
		raw = v
	case json.RawMessage:
		raw = []byte(v)
	case string:
		raw = []byte(v)
	default:
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
	}
	req, err := http.NewRequest(method, path, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		addTestUserAuthHeader(req, userID)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestAdminBlockUser_HappyPath(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.admin.TenantID, "ordinary@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/block",
		env.admin.ID, blockBody("policy violation", false))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.id"), target.ID)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.type"), "users")
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.is_active"), false)

	// IsActive should be persisted false on the registry row.
	reloaded := must.Must(env.params.FactorySet.UserRegistry.Get(context.Background(), target.ID))
	c.Assert(reloaded.IsActive, qt.IsFalse)
}

func TestAdminBlockUser_Idempotent(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.admin.TenantID, "ordinary@example.com", false, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/block",
		env.admin.ID, blockBody("policy violation", false))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.is_active"), false)
}

func TestAdminBlockUser_SelfBlockRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+env.admin.ID+"/block",
		env.admin.ID, blockBody("trying to lock myself out", false))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminBlockSelfBlockedCode)
}

func TestAdminBlockUser_AdminWithoutForceRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	peer := createTestUserDirect(c, env.params, env.admin.TenantID, "peer-admin@example.com", true, true)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+peer.ID+"/block",
		env.admin.ID, blockBody("peer review", false))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminBlockAdminRequiresForceCode)

	// And IsActive on the peer is preserved — the guard rejected before
	// the cascade could fire.
	reloaded := must.Must(env.params.FactorySet.UserRegistry.Get(context.Background(), peer.ID))
	c.Assert(reloaded.IsActive, qt.IsTrue)
}

func TestAdminBlockUser_AdminWithForceAllowedAndAudited(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	peer := createTestUserDirect(c, env.params, env.admin.TenantID, "peer-admin@example.com", true, true)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+peer.ID+"/block",
		env.admin.ID, blockBody("compromise drill", true))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.is_active"), false)

	// Audit row uses the distinct AuditActionAdminUserBlockForce action
	// AND the breadcrumb carries forced=true.
	rows := must.Must(env.params.FactorySet.AuditLogRegistry.List(context.Background()))
	var force *models.AuditLog
	for i := range rows {
		if rows[i].Action == apiserver.AuditActionAdminUserBlockForce {
			force = rows[i]
			break
		}
	}
	c.Assert(force, qt.IsNotNil, qt.Commentf("expected an admin.user_block_force audit row"))
	var bc map[string]any
	c.Assert(json.Unmarshal([]byte(force.UserAgent), &bc), qt.IsNil)
	c.Assert(bc["forced"], qt.Equals, true)
	c.Assert(bc["reason"], qt.Equals, "compromise drill")
}

func TestAdminBlockUser_AuditRowCarriesReason(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.admin.TenantID, "ordinary@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/block",
		env.admin.ID, blockBody("policy violation: rule 7", false))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	rows := must.Must(env.params.FactorySet.AuditLogRegistry.List(context.Background()))
	var row *models.AuditLog
	for i := range rows {
		if rows[i].Action == apiserver.AuditActionAdminUserBlock {
			row = rows[i]
			break
		}
	}
	c.Assert(row, qt.IsNotNil)
	c.Assert(row.UserID, qt.IsNotNil)
	c.Assert(*row.UserID, qt.Equals, env.admin.ID)
	c.Assert(row.EntityID, qt.IsNotNil)
	c.Assert(*row.EntityID, qt.Equals, target.ID)
	var bc map[string]any
	c.Assert(json.Unmarshal([]byte(row.UserAgent), &bc), qt.IsNil)
	c.Assert(bc["reason"], qt.Equals, "policy violation: rule 7")
	// Non-forced rows should not carry the forced=true breadcrumb key.
	_, hasForced := bc["forced"]
	c.Assert(hasForced, qt.IsFalse)
}

func TestAdminBlockUser_UnknownUserReturns404(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/does-not-exist/block",
		env.admin.ID, blockBody("policy violation", false))
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAdminBlockUser_BadBodyVariants(t *testing.T) {
	cases := []struct {
		name     string
		body     any
		wantCode int
	}{
		{"missing_reason", map[string]any{"force": false}, http.StatusUnprocessableEntity},
		{"empty_reason", map[string]any{"reason": "   "}, http.StatusUnprocessableEntity},
		{"reason_too_long", map[string]any{"reason": strings.Repeat("x", 501)}, http.StatusUnprocessableEntity},
		// Raw bytes so the body actually IS malformed JSON; passing a
		// Go string through json.Marshal would re-encode it as a valid
		// JSON string literal and never exercise the decoder error path.
		{"malformed_json", []byte(`{"reason":`), http.StatusBadRequest},
		{"unknown_field", map[string]any{"reason": "ok", "extra": 1}, http.StatusBadRequest},
		// Trailing tokens after a valid object must be rejected — the
		// json.Decoder accepts the first value happily, so the handler
		// has to do a second Decode and require io.EOF (via the
		// decoderAtEOF helper) to catch the concatenation attack.
		{"multi_object_trailing", []byte(`{"reason":"x"}{"extra":1}`), http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			env := newAdminEnv(c)
			target := createTestUserDirect(c, env.params, env.admin.TenantID, "ordinary@example.com", true, false)

			rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
				"/api/v1/admin/users/"+target.ID+"/block",
				env.admin.ID, tc.body)
			c.Assert(rr.Code, qt.Equals, tc.wantCode)
		})
	}
}

func TestAdminBlockUser_NonAdminForbidden(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams() // not promoted
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	target := createTestUserDirect(c, params, user.TenantID, "ordinary@example.com", true, false)

	rr := doAdminJSONRequest(t, handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/block",
		user.ID, blockBody("policy violation", false))
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	c.Assert(rr.Body.String(), qt.Contains, "admin.forbidden")
}

func TestAdminBlockUser_UnauthenticatedRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.admin.TenantID, "ordinary@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/block",
		"", blockBody("policy violation", false))
	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

func TestAdminUnblockUser_HappyPath(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.admin.TenantID, "ordinary@example.com", false, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/unblock",
		env.admin.ID, unblockBody("appeal accepted"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.is_active"), true)

	reloaded := must.Must(env.params.FactorySet.UserRegistry.Get(context.Background(), target.ID))
	c.Assert(reloaded.IsActive, qt.IsTrue)
}

func TestAdminUnblockUser_Idempotent(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.admin.TenantID, "ordinary@example.com", true, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/unblock",
		env.admin.ID, unblockBody("nothing to do"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.is_active"), true)
}

func TestAdminUnblockUser_UnknownUserReturns404(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/does-not-exist/unblock",
		env.admin.ID, unblockBody("test"))
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAdminUnblockUser_AuditRowCarriesReason(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.admin.TenantID, "ordinary@example.com", false, false)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/unblock",
		env.admin.ID, unblockBody("appeal accepted: ticket 1234"))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	rows := must.Must(env.params.FactorySet.AuditLogRegistry.List(context.Background()))
	var row *models.AuditLog
	for i := range rows {
		if rows[i].Action == apiserver.AuditActionAdminUserUnblock {
			row = rows[i]
			break
		}
	}
	c.Assert(row, qt.IsNotNil)
	var bc map[string]any
	c.Assert(json.Unmarshal([]byte(row.UserAgent), &bc), qt.IsNil)
	c.Assert(bc["reason"], qt.Equals, "appeal accepted: ticket 1234")
}

// TestAdminUnblockUser_BadBodyVariants mirrors the block-side table but
// stays narrow — just enough to pin the trailing-tokens regression that
// the decoderAtEOF helper (second Decode + io.EOF check) guards
// against. The other reason-validation paths are already covered by
// the block-side table and by the decodeUnblockRequest happy-path
// tests.
func TestAdminUnblockUser_BadBodyVariants(t *testing.T) {
	cases := []struct {
		name     string
		body     any
		wantCode int
	}{
		{"malformed_json", []byte(`{"reason":`), http.StatusBadRequest},
		{"multi_object_trailing", []byte(`{"reason":"x"}{"extra":1}`), http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			env := newAdminEnv(c)
			target := createTestUserDirect(c, env.params, env.admin.TenantID, "ordinary@example.com", false, false)

			rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
				"/api/v1/admin/users/"+target.ID+"/unblock",
				env.admin.ID, tc.body)
			c.Assert(rr.Code, qt.Equals, tc.wantCode)
		})
	}
}

// ---------------------------------------------------------------------
// Integration tests: prove the block cascade actually invalidates live
// access tokens (iat-staleness) and revokes refresh tokens. These
// exercise the full middleware chain — JWT validation, blacklist
// check, refresh-token revocation — not just the handler logic.
// ---------------------------------------------------------------------

// mintAccessTokenForUser creates a signed access token whose iat is
// `iatOffset` seconds before now. Tests can pass a negative offset to
// simulate a token that was minted before the block was issued. The
// token carries the canonical claims the JWT middleware expects:
// user_id, jti, iat, exp.
func mintAccessTokenForUser(t *testing.T, userID string, iatOffset time.Duration) string {
	t.Helper()
	iat := time.Now().Add(iatOffset)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id":    userID,
		"jti":        "test-jti-" + userID,
		"token_type": "access",
		"iat":        iat.Unix(),
		"exp":        iat.Add(24 * time.Hour).Unix(),
	})
	signed, err := token.SignedString(testJWTSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

func TestAdminBlockUser_AccessTokenIssuedBeforeBlockFails(t *testing.T) {
	c := qt.New(t)

	// Build params with a real in-memory blacklister so the BlacklistUserTokens
	// call inside the block cascade is observable by subsequent /system requests.
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	bl := services.NewInMemoryTokenBlacklister()
	params.TokenBlacklister = bl
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	target := createTestUserDirect(c, params, admin.TenantID, "ordinary@example.com", true, false)
	// Mint an access token for the TARGET with iat=now-1s. After block
	// the blacklister stamps "since" at the block's wall-clock time, so
	// this token is "before" the blacklist threshold and must be rejected.
	staleToken := mintAccessTokenForUser(t, target.ID, -time.Second)

	// Pre-block sanity: GET /users/me/sessions with the target's access
	// token should be reachable (the JWT path doesn't care about
	// IsActive=true users, and the sessions route is auth-only with no
	// group dependency).
	preReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/sessions", nil)
	preReq.Header.Set("Authorization", "Bearer "+staleToken)
	preRR := httptest.NewRecorder()
	handler.ServeHTTP(preRR, preReq)
	c.Assert(preRR.Code, qt.Equals, http.StatusOK,
		qt.Commentf("pre-block sessions probe must succeed; body=%s", preRR.Body.String()))

	// Sleep ~10ms so that the blacklister's stamped "since" timestamp is
	// guaranteed to fall *after* the access token's iat (which is
	// `now-1s` at the moment the token was minted, but the iat field is
	// truncated to whole seconds via Unix() — two events firing within
	// the same second would otherwise share the same epoch second and
	// the iat.Before(since) check would compare equal).
	time.Sleep(10 * time.Millisecond)

	// Execute the block as the admin actor.
	blockRR := doAdminJSONRequest(t, handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/block",
		admin.ID, blockBody("integration test", false))
	c.Assert(blockRR.Code, qt.Equals, http.StatusOK,
		qt.Commentf("block must succeed; body=%s", blockRR.Body.String()))

	// Post-block: the same access token must be rejected — the iat-stale
	// check inside checkTokenBlacklist trips, returning 401.
	postReq := httptest.NewRequest(http.MethodGet, "/api/v1/users/me/sessions", nil)
	postReq.Header.Set("Authorization", "Bearer "+staleToken)
	postRR := httptest.NewRecorder()
	handler.ServeHTTP(postRR, postReq)
	c.Assert(postRR.Code, qt.Equals, http.StatusUnauthorized,
		qt.Commentf("post-block sessions probe must be 401; body=%s", postRR.Body.String()))
}

func TestAdminBlockUser_RefreshTokenRevokedByCascade(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	target := createTestUserDirect(c, params, admin.TenantID, "ordinary@example.com", true, false)

	// Seed a refresh token for the target. After block, ListActiveByUserID
	// must return zero rows — the cascade calls RevokeByUserID and the
	// memory impl flips Revoked=true.
	rt := models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: target.TenantID,
			UserID:   target.ID,
		},
		TokenHash: "test-hash-block-cascade",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	must.Must(params.FactorySet.RefreshTokenRegistry.Create(context.Background(), rt))

	preActive := must.Must(params.FactorySet.RefreshTokenRegistry.ListActiveByUserID(context.Background(), target.ID))
	c.Assert(preActive, qt.HasLen, 1)

	blockRR := doAdminJSONRequest(t, handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/block",
		admin.ID, blockBody("integration test", false))
	c.Assert(blockRR.Code, qt.Equals, http.StatusOK)

	postActive := must.Must(params.FactorySet.RefreshTokenRegistry.ListActiveByUserID(context.Background(), target.ID))
	c.Assert(postActive, qt.HasLen, 0)
}

// TestAdminBlockUser_IdempotentBlockReRunsCascade pins the B1 fix: a
// re-block of an already-inactive user must run the cascade again so
// any tokens issued between blocks (e.g. via the blacklist ring
// expiring under a 30-min+ gap, or a future impersonation grant) are
// invalidated. Without the fix the cascade is skipped on the
// idempotent path and a stale refresh token survives the second block.
func TestAdminBlockUser_IdempotentBlockReRunsCascade(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Seed an already-inactive user — simulates the "previously
	// blocked" state where the cascade ran once long ago.
	target := createTestUserDirect(c, params, admin.TenantID, "ordinary@example.com", false, false)

	// Plant a fresh refresh token AFTER the user was already inactive.
	// Represents a token that should never have existed (post-block
	// new grant), or that survived the previous cascade's TTL window.
	rt := models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: target.TenantID,
			UserID:   target.ID,
		},
		TokenHash: "test-hash-idempotent-cascade",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	must.Must(params.FactorySet.RefreshTokenRegistry.Create(context.Background(), rt))

	preActive := must.Must(params.FactorySet.RefreshTokenRegistry.ListActiveByUserID(context.Background(), target.ID))
	c.Assert(preActive, qt.HasLen, 1)

	// Re-block: even though IsActive is already false, the cascade
	// must fire.
	rr := doAdminJSONRequest(t, handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/block",
		admin.ID, blockBody("re-block to scrub stale tokens", false))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	postActive := must.Must(params.FactorySet.RefreshTokenRegistry.ListActiveByUserID(context.Background(), target.ID))
	c.Assert(postActive, qt.HasLen, 0,
		qt.Commentf("idempotent re-block must run the cascade and revoke the planted token"))
}

// createTestJWTTokenWithClaims signs an arbitrary MapClaims for the
// given user. Used by the impersonation self-block test so it can
// stamp `imp` / `impersonated_by` onto the access token the same way
// the (forthcoming) impersonation primitive will. Mirrors the shape of
// createTestJWTToken but stays test-local so the production helper
// doesn't grow a knob it never uses.
func createTestJWTTokenWithClaims(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	if _, ok := claims["exp"]; !ok {
		claims["exp"] = time.Now().Add(24 * time.Hour).Unix()
	}
	// Default to an access token so the forged token clears the
	// token-type enforcement in validateJWTToken (#1778). Callers can
	// override token_type explicitly to test the rejection path.
	if _, ok := claims["token_type"]; !ok {
		claims["token_type"] = "access"
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(testJWTSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return signed
}

// TestAdminBlockUser_SelfBlockViaImpersonationRejected pins the B2
// fix: the self-block guard must compare against the operator-of-
// record (the `impersonated_by` claim), not the impersonated user's
// JWT user_id. Without this, an operator could route a block-self
// request through an impersonated peer-admin session and bypass the
// guard.
//
// Scenario: env.admin is the operator and impersonates `peer` (also
// a system admin so the impersonated session clears the admin gate).
// The bearer token's user_id is `peer.ID` with `imp=true` and
// `impersonated_by=env.admin.ID`. The operator then issues a block
// against env.admin.ID — without the operator-aware guard the
// handler would compare target.ID (env.admin.ID) to actor.ID
// (peer.ID) and let it through.
func TestAdminBlockUser_SelfBlockViaImpersonationRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	peer := createTestUserDirect(c, env.params, env.admin.TenantID, "peer-admin@example.com", true, true)

	token := createTestJWTTokenWithClaims(t, jwt.MapClaims{
		"user_id":         peer.ID,
		"imp":             true,
		"impersonated_by": env.admin.ID,
	})

	body, err := json.Marshal(blockBody("self-block via impersonation", true))
	c.Assert(err, qt.IsNil)
	req, err := http.NewRequest(http.MethodPost,
		"/api/v1/admin/users/"+env.admin.ID+"/block", bytes.NewReader(body))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	env.handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity,
		qt.Commentf("self-block via impersonated session must be 422; body=%s", rr.Body.String()))
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminBlockSelfBlockedCode)

	// And the operator's own account stays active — the guard fired
	// before any registry write.
	reloaded := must.Must(env.params.FactorySet.UserRegistry.Get(context.Background(), env.admin.ID))
	c.Assert(reloaded.IsActive, qt.IsTrue)
}

// TestAdminBlockUser_IdempotentOnPeerAdminWithoutForce pins the S4
// fix: re-blocking an already-inactive peer system admin must be a
// 200 no-op even when `force=true` is absent. Previously the
// admin-without-force guard ran before the idempotency check and
// returned a surprising 422 on a state-equivalent request.
func TestAdminBlockUser_IdempotentOnPeerAdminWithoutForce(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	// Peer is a system admin AND already inactive — the
	// admin-without-force guard would normally reject without force,
	// but the idempotent path must short-circuit it.
	peer := createTestUserDirect(c, env.params, env.admin.TenantID, "peer-admin@example.com", false, true)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+peer.ID+"/block",
		env.admin.ID, blockBody("idempotent peer-admin no-op", false))
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.is_active"), false)

	// The audit row must use the un-forced action and carry
	// forced=false (or omit the key) — nothing was actually forced.
	rows := must.Must(env.params.FactorySet.AuditLogRegistry.List(context.Background()))
	var row *models.AuditLog
	for i := range rows {
		if rows[i].EntityID != nil && *rows[i].EntityID == peer.ID {
			row = rows[i]
			break
		}
	}
	c.Assert(row, qt.IsNotNil)
	c.Assert(row.Action, qt.Equals, apiserver.AuditActionAdminUserBlock,
		qt.Commentf("idempotent path must NOT emit the _force action"))
	var bc map[string]any
	c.Assert(json.Unmarshal([]byte(row.UserAgent), &bc), qt.IsNil)
	_, hasForced := bc["forced"]
	c.Assert(hasForced, qt.IsFalse,
		qt.Commentf("idempotent path must NOT set forced=true in the breadcrumb"))
}

// TestAdminUnblockUser_LeavesStalenessRingIntact pins the S3 promise:
// unblocking a user does NOT clear the iat-staleness ring entry, so
// access tokens minted before the block stay rejected after unblock.
func TestAdminUnblockUser_LeavesStalenessRingIntact(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	bl := services.NewInMemoryTokenBlacklister()
	params.TokenBlacklister = bl
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	target := createTestUserDirect(c, params, admin.TenantID, "ordinary@example.com", true, false)

	// Block — installs an entry in the staleness ring.
	blockRR := doAdminJSONRequest(t, handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/block",
		admin.ID, blockBody("policy violation", false))
	c.Assert(blockRR.Code, qt.Equals, http.StatusOK)

	since, blacklisted, err := bl.UserBlacklistedSince(context.Background(), target.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(blacklisted, qt.IsTrue, qt.Commentf("block must install a staleness ring entry"))
	c.Assert(since.IsZero(), qt.IsFalse)

	// Unblock — must leave the ring untouched.
	unblockRR := doAdminJSONRequest(t, handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/unblock",
		admin.ID, unblockBody("appeal accepted"))
	c.Assert(unblockRR.Code, qt.Equals, http.StatusOK)

	sinceAfter, blacklistedAfter, err := bl.UserBlacklistedSince(context.Background(), target.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(blacklistedAfter, qt.IsTrue,
		qt.Commentf("unblock must NOT clear the staleness ring entry (#1747 spec)"))
	c.Assert(sinceAfter.Equal(since), qt.IsTrue,
		qt.Commentf("the ring entry's `since` must be unchanged by unblock"))
}
