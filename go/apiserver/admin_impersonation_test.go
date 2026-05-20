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
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
)

// Impersonation primitive handler tests (#1750). Every acceptance
// criterion in the issue spec maps to one (or more) of the subtests
// below. Tests reuse the admin fixtures from admin_routes_test.go
// (promoteToSystemAdmin) and admin_users_test.go (createTestUserDirect).

// impersonateClaims is the minimal claim set for a forged impersonation
// access token used by tests that need to exercise the end / current /
// refresh paths without first calling the start endpoint.
type impersonateClaims struct {
	jti            string
	targetUserID   string
	targetTenantID string
	adminUserID    string
	expiresAt      time.Time
}

// makeImpersonationToken signs an impersonation access token with the
// shared test JWT secret. Mirrors adminImpersonationAPI.signImpersonationToken
// so tests can forge a token directly (e.g. an already-expired one).
func makeImpersonationToken(c *qt.C, cl impersonateClaims) string {
	c.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":             cl.jti,
		"user_id":         cl.targetUserID,
		"tenant_id":       cl.targetTenantID,
		"impersonated_by": cl.adminUserID,
		"imp":             true,
		"is_system_admin": false,
		"iat":             time.Now().Add(-time.Minute).Unix(),
		"exp":             cl.expiresAt.Unix(),
	})
	signed, err := token.SignedString(testJWTSecret)
	c.Assert(err, qt.IsNil)
	return signed
}

// doImpersonateStart issues POST /admin/users/{id}/impersonate as the
// given admin and returns the recorder.
func doImpersonateStart(c *qt.C, handler http.Handler, adminID, targetID string, body map[string]any) *httptest.ResponseRecorder {
	c.Helper()
	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		c.Assert(err, qt.IsNil)
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+targetID+"/impersonate", reader)
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, adminID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestImpersonate_StartSucceeds(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, admin.ID, target.ID, map[string]any{"reason": "support ticket #42"})

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var resp apiserver.LoginResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.AccessToken, qt.Not(qt.Equals), "")
	c.Assert(resp.TokenType, qt.Equals, "Bearer")
	c.Assert(resp.User, qt.IsNotNil)
	c.Assert(resp.User.ID, qt.Equals, target.ID)

	// The issued token must carry the impersonation claims.
	claims := parseTestTokenClaims(c, resp.AccessToken)
	c.Assert(claims["user_id"], qt.Equals, target.ID)
	c.Assert(claims["impersonated_by"], qt.Equals, admin.ID)
	c.Assert(claims["imp"], qt.Equals, true)
	c.Assert(claims["is_system_admin"], qt.Equals, false)
}

func TestImpersonate_StartWithoutBodySucceeds(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// The reason body is optional — an empty body must still succeed.
	rr := doImpersonateStart(c, handler, admin.ID, target.ID, nil)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
}

func TestImpersonate_RejectsOverLongReason(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// An over-long reason is a 422 with admin.impersonate.reason_too_long
	// — matching the admin block handler's reason-length contract.
	rr := doImpersonateStart(c, handler, admin.ID, target.ID,
		map[string]any{"reason": strings.Repeat("x", 501)})

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateReasonTooLongCode)
}

func TestImpersonate_RejectsNonAdmin(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	target := createTestUserDirect(c, params, user.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, user.ID, target.ID, nil)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	assertErrorCode(t, c, rr.Body.Bytes(), "admin.forbidden")
}

func TestImpersonate_RejectsTargetAdmin(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	targetAdmin := createTestUserDirect(c, params, admin.TenantID, "peer-admin@example.com", true, true)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, admin.ID, targetAdmin.ID, nil)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateTargetIsAdminCode)
}

func TestImpersonate_RejectsBlockedTarget(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	blocked := createTestUserDirect(c, params, admin.TenantID, "blocked@example.com", false, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, admin.ID, blocked.ID, nil)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateTargetBlockedCode)
}

func TestImpersonate_RejectsUnknownTarget(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, admin.ID, "00000000-0000-0000-0000-000000000000", nil)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestImpersonate_RejectsNestedImpersonation(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	secondTarget := createTestUserDirect(c, params, admin.TenantID, "second@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Start a real impersonation session to get a genuine imp token.
	startRR := doImpersonateStart(c, handler, admin.ID, target.ID, nil)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)

	// Use the impersonation token to attempt a second impersonation.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+secondTarget.ID+"/impersonate", nil)
	req.Header.Set("Authorization", "Bearer "+startResp.AccessToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// The impersonation token belongs to a non-admin target, so
	// RequireSystemAdmin rejects it first with 403 — nested impersonation
	// is structurally impossible because the imp token never carries
	// system-admin authority. This is a stronger guarantee than the
	// 422 nested-guard, which only fires if the target somehow had the
	// admin flag.
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
}

func TestImpersonate_NestedGuardRejectsWhenTokenReachesHandler(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	// A second admin is the impersonation target's stand-in: we forge an
	// imp token whose user_id is an ADMIN so it clears RequireSystemAdmin
	// and actually reaches the nested-impersonation guard in the handler.
	targetAdmin := createTestUserDirect(c, params, admin.TenantID, "target-admin@example.com", true, true)
	victim := createTestUserDirect(c, params, admin.TenantID, "victim@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	impToken := makeImpersonationToken(c, impersonateClaims{
		jti:            "nested-test-jti",
		targetUserID:   targetAdmin.ID,
		targetTenantID: targetAdmin.TenantID,
		adminUserID:    admin.ID,
		expiresAt:      time.Now().Add(20 * time.Minute),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+victim.ID+"/impersonate", nil)
	req.Header.Set("Authorization", "Bearer "+impToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateNestedCode)
}

// TestImpersonate_ExpiredTokenWithNoSlotEndsAsNotActive: an expired imp
// token whose return slot was never recorded (here, a forged token) still
// reaches endImpersonation now that `end` is mounted without JWTMiddleware
// (#1750 / PR #1771 FIX 2) — but with no slot to restore, the only correct
// answer is 422 not_active. The expiry relaxation does NOT turn a
// slot-less token into a successful end; the server-side slot is still the
// real proof. See TestImpersonate_EndAcceptsExpiredTokenWithLiveSlot for
// the expired-but-restorable case.
func TestImpersonate_ExpiredTokenWithNoSlotEndsAsNotActive(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	expiredToken := makeImpersonationToken(c, impersonateClaims{
		jti:            "expired-jti",
		targetUserID:   target.ID,
		targetTenantID: target.TenantID,
		adminUserID:    admin.ID,
		expiresAt:      time.Now().Add(-time.Minute),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/impersonation/end", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateNotActiveCode)
}

func TestImpersonate_RefreshRejectsImpersonationToken(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	impToken := makeImpersonationToken(c, impersonateClaims{
		jti:            "refresh-test-jti",
		targetUserID:   target.ID,
		targetTenantID: target.TenantID,
		adminUserID:    admin.ID,
		expiresAt:      time.Now().Add(20 * time.Minute),
	})

	// The refresh endpoint must reject a request carrying an imp token,
	// even when a (would-be valid) refresh cookie is also present.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+impToken)
	// #nosec G124 -- test request cookie; Secure/SameSite are irrelevant to the assertion.
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "some-refresh-token"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

func TestImpersonate_EndRestoresAdminContext(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Start a real session so the return slot exists.
	startRR := doImpersonateStart(c, handler, admin.ID, target.ID, nil)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)

	// End the session using the impersonation token.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/impersonation/end", nil)
	req.Header.Set("Authorization", "Bearer "+startResp.AccessToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var endResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &endResp), qt.IsNil)
	// The restored session is the admin's own — user + token claims.
	c.Assert(endResp.User, qt.IsNotNil)
	c.Assert(endResp.User.ID, qt.Equals, admin.ID)
	claims := parseTestTokenClaims(c, endResp.AccessToken)
	c.Assert(claims["user_id"], qt.Equals, admin.ID)
	c.Assert(claims["is_system_admin"], qt.Equals, true)
	// The restored admin token must NOT carry impersonation claims.
	_, hasImp := claims["imp"]
	c.Assert(hasImp, qt.IsFalse)

	// The LogAdmin path that stamps the audit row: the
	// admin.impersonate_end row must record actor = target (UserID) AND
	// impersonated_by = admin — the end request runs under the imp token
	// so LogAdmin auto-fills ImpersonatedBy from the claims.
	rows := must2(params.FactorySet.AuditLogRegistry.List(context.Background()))
	var endRow *models.AuditLog
	for _, row := range rows {
		if row.Action == apiserver.AuditActionAdminImpersonateEnd {
			endRow = row
			break
		}
	}
	c.Assert(endRow, qt.IsNotNil, qt.Commentf("expected an admin.impersonate_end audit row"))
	c.Assert(endRow.UserID, qt.IsNotNil)
	c.Assert(*endRow.UserID, qt.Equals, target.ID)
	c.Assert(endRow.ImpersonatedBy, qt.IsNotNil)
	c.Assert(*endRow.ImpersonatedBy, qt.Equals, admin.ID)
}

func TestImpersonate_EndRejectsNonImpersonationToken(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// A plain admin token (no imp claim) at the end endpoint → 422.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/impersonation/end", nil)
	addTestUserAuthHeader(req, admin.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateNotActiveCode)
}

func TestImpersonate_CurrentReportsInactiveForPlainAdmin(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/impersonation/current", nil)
	addTestUserAuthHeader(req, admin.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var resp apiserver.ImpersonationStateResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Active, qt.IsFalse)
}

func TestImpersonate_CurrentReportsActiveSession(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	startRR := doImpersonateStart(c, handler, admin.ID, target.ID, nil)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/impersonation/current", nil)
	req.Header.Set("Authorization", "Bearer "+startResp.AccessToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var resp apiserver.ImpersonationStateResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Active, qt.IsTrue)
	c.Assert(resp.TargetUser, qt.IsNotNil)
	c.Assert(resp.TargetUser.ID, qt.Equals, target.ID)
	c.Assert(resp.AdminUser, qt.IsNotNil)
	c.Assert(resp.AdminUser.ID, qt.Equals, admin.ID)
	c.Assert(resp.StartedAt, qt.IsNotNil)
	c.Assert(resp.ExpiresAt, qt.IsNotNil)
}

func TestImpersonate_RateLimitEnforced(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	// The in-memory limiter is shared across requests on this handler;
	// 10/hour is the configured impersonation limit.
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	var lastCode int
	for range 12 {
		rr := doImpersonateStart(c, handler, admin.ID, target.ID, nil)
		lastCode = rr.Code
		if rr.Code == http.StatusTooManyRequests {
			assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateRateLimitedCode)
			break
		}
	}
	c.Assert(lastCode, qt.Equals, http.StatusTooManyRequests,
		qt.Commentf("expected the per-admin impersonation rate limit to trip within 12 attempts"))
}

// createAdminRefreshToken persists a genuine refresh-token row for the
// given user and returns the raw cookie value. Used by the cookie-flow
// tests to give the admin a real, refreshable session before
// impersonation starts — so the test can prove the cookie is replaced on
// start and restored on end.
func createAdminRefreshToken(c *qt.C, params apiserver.Params, user *models.User) string {
	c.Helper()
	raw, hash, err := models.GenerateRefreshToken()
	c.Assert(err, qt.IsNil)
	_, err = params.FactorySet.RefreshTokenRegistry.Create(context.Background(), models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		TokenHash: hash,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
		CreatedAt: time.Now(),
	})
	c.Assert(err, qt.IsNil)
	return raw
}

// refreshCookieFromResponse extracts the value the response set the
// refresh_token cookie to (the part after "refresh_token=" up to the
// first ';'). Returns ("", false) when the response sets no refresh
// cookie. Used to follow the cookie a handler emitted into a subsequent
// request, the way a browser would.
func refreshCookieFromResponse(rr *httptest.ResponseRecorder) (string, bool) {
	for _, sc := range rr.Header().Values("Set-Cookie") {
		if !strings.HasPrefix(sc, "refresh_token=") {
			continue
		}
		value := strings.TrimPrefix(sc, "refresh_token=")
		if i := strings.IndexByte(value, ';'); i >= 0 {
			value = value[:i]
		}
		return value, true
	}
	return "", false
}

// TestImpersonate_StartReplacesRefreshCookie proves the start handler
// overwrites the admin's own refresh cookie with the impersonation
// marker. Leaving the admin's refresh token in the browser would let the
// FE's cookie-based refresh interceptor silently mint a fresh admin
// access token mid-impersonation (#1750 / PR #1771 review).
func TestImpersonate_StartReplacesRefreshCookie(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, admin.ID, target.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	cookieValue, ok := refreshCookieFromResponse(rr)
	c.Assert(ok, qt.IsTrue, qt.Commentf("start must Set-Cookie the refresh_token"))
	// The cookie must carry the non-refreshable impersonation marker, not
	// a genuine refresh token.
	c.Assert(strings.HasPrefix(cookieValue, "imp:"), qt.IsTrue,
		qt.Commentf("refresh cookie should hold the impersonation marker, got %q", cookieValue))
}

// TestImpersonate_RefreshRejectedDuringSession_CookieOnly is the core
// regression test for the PR #1771 review bug: a refresh attempt made
// from inside an impersonation session — cookie-based, with NO
// Authorization header (the realistic FE refresh path) — must be
// rejected and must not mint an access token.
func TestImpersonate_RefreshRejectedDuringSession_CookieOnly(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Start a genuine impersonation session and follow the marker cookie
	// the handler planted, exactly as a browser would.
	startRR := doImpersonateStart(c, handler, admin.ID, target.ID, nil)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	markerCookie, ok := refreshCookieFromResponse(startRR)
	c.Assert(ok, qt.IsTrue)

	// Refresh with ONLY the cookie — no Authorization header, the way the
	// FE refresh interceptor posts to /auth/refresh.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	// #nosec G124 -- test request cookie; transport security is irrelevant to the assertion.
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: markerCookie})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
	// No access token may be minted — the response body must not be a
	// LoginResponse carrying one.
	var resp apiserver.LoginResponse
	_ = json.Unmarshal(rr.Body.Bytes(), &resp)
	c.Assert(resp.AccessToken, qt.Equals, "")
}

// TestImpersonate_EndRestoresRefreshCookie proves the end handler puts
// the admin's original refresh cookie back, and that the restored
// session is genuinely refreshable afterwards — i.e. the operator is
// whole again, including the cookie-based refresh flow.
func TestImpersonate_EndRestoresRefreshCookie(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Give the admin a real, refreshable session before impersonating.
	adminRefreshRaw := createAdminRefreshToken(c, params, admin)

	// Start: send the admin's refresh cookie so the return slot captures it.
	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+target.ID+"/impersonate", bytes.NewReader(nil))
	startReq.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(startReq, admin.ID)
	// #nosec G124 -- test request cookie.
	startReq.AddCookie(&http.Cookie{Name: "refresh_token", Value: adminRefreshRaw})
	startRR := httptest.NewRecorder()
	handler.ServeHTTP(startRR, startReq)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)

	// End the session with the impersonation token.
	endReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/impersonation/end", nil)
	endReq.Header.Set("Authorization", "Bearer "+startResp.AccessToken)
	endRR := httptest.NewRecorder()
	handler.ServeHTTP(endRR, endReq)
	c.Assert(endRR.Code, qt.Equals, http.StatusOK)

	// The end response must restore the admin's ORIGINAL refresh token,
	// not the marker.
	restored, ok := refreshCookieFromResponse(endRR)
	c.Assert(ok, qt.IsTrue, qt.Commentf("end must Set-Cookie the refresh_token"))
	c.Assert(restored, qt.Equals, adminRefreshRaw)

	// The restored session must be genuinely refreshable: a cookie-based
	// /auth/refresh with the restored token succeeds and mints an ADMIN
	// access token.
	refreshReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	// #nosec G124 -- test request cookie.
	refreshReq.AddCookie(&http.Cookie{Name: "refresh_token", Value: restored})
	refreshRR := httptest.NewRecorder()
	handler.ServeHTTP(refreshRR, refreshReq)
	c.Assert(refreshRR.Code, qt.Equals, http.StatusOK)
	var refreshResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(refreshRR.Body.Bytes(), &refreshResp), qt.IsNil)
	c.Assert(refreshResp.AccessToken, qt.Not(qt.Equals), "")
	refreshClaims := parseTestTokenClaims(c, refreshResp.AccessToken)
	c.Assert(refreshClaims["user_id"], qt.Equals, admin.ID)
}

// parseTestTokenClaims parses a JWT signed with the shared test secret
// and returns its claims. Used to assert the impersonation / restored
// token shapes without going through the middleware.
func parseTestTokenClaims(c *qt.C, tokenString string) jwt.MapClaims {
	c.Helper()
	token, err := jwt.Parse(tokenString, func(_ *jwt.Token) (any, error) {
		return testJWTSecret, nil
	})
	c.Assert(err, qt.IsNil)
	claims, ok := token.Claims.(jwt.MapClaims)
	c.Assert(ok, qt.IsTrue)
	return claims
}

// TestImpersonate_AuditTrailRecordsImpersonator is the integration
// check from the #1750 acceptance criteria: an admin impersonates a
// regular user, the impersonated session hits a (non-admin) GROUP
// endpoint, and the resulting audit-log row carries both the actor
// (the impersonated target, via the JWT user_id) and the impersonator
// (the admin, via the `impersonated_by` claim). The audit helper reads
// `imp`/`impersonated_by` from the JWT claims automatically — this test
// verifies that propagation end-to-end through the real router.
func TestImpersonate_AuditTrailRecordsImpersonator(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)

	// A regular (non-admin) target who owns a fresh group — group
	// deletion is the LogAuth-based group endpoint that audit-logs.
	target := createTestUserDirect(c, params, admin.TenantID, "group-owner@example.com", true, false)
	group := createOwnedTestGroup(c, params, target)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Start a genuine impersonation session via the start endpoint.
	startRR := doImpersonateStart(c, handler, admin.ID, target.ID, nil)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)

	// The impersonated session deletes the group it (the target) owns.
	// deleteGroup requires the group name as confirm_word plus the
	// target's password ("Password123", set by createTestUserDirect).
	delBody, err := json.Marshal(map[string]any{
		"confirm_word": group.Name,
		"password":     "Password123",
	})
	c.Assert(err, qt.IsNil)
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/groups/"+group.ID, bytes.NewReader(delBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+startResp.AccessToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent,
		qt.Commentf("group delete under impersonation; body=%s", rr.Body.String()))

	// The group_delete audit row must record actor = target (JWT
	// user_id) AND impersonated_by = admin.
	rows := must2(params.FactorySet.AuditLogRegistry.List(context.Background()))
	var found *models.AuditLog
	for _, row := range rows {
		if row.Action == "group_delete" {
			found = row
			break
		}
	}
	c.Assert(found, qt.IsNotNil, qt.Commentf("expected a group_delete audit row"))
	c.Assert(found.UserID, qt.IsNotNil)
	c.Assert(*found.UserID, qt.Equals, target.ID)
	c.Assert(found.ImpersonatedBy, qt.IsNotNil)
	c.Assert(*found.ImpersonatedBy, qt.Equals, admin.ID)
}

// createOwnedTestGroup creates a fresh group whose sole member is the
// given user, with the Owner role — so the user can delete it. Used by
// the audit-trail integration test, which needs a group endpoint the
// impersonated session is authorized to hit.
func createOwnedTestGroup(c *qt.C, params apiserver.Params, owner *models.User) *models.LocationGroup {
	c.Helper()
	ctx := context.Background()
	slug := must2(models.GenerateGroupSlug())
	group := must2(params.FactorySet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		Name:                "Impersonation Test Group",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           owner.ID,
		GroupCurrency:       models.Currency("USD"),
	}))
	must2(params.FactorySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		GroupID:             group.ID,
		MemberUserID:        owner.ID,
		Role:                models.GroupRoleOwner,
	}))
	return group
}

// must2 unwraps a (T, error) pair, panicking on error. Used only in the
// audit-trail test's setup-style registry reads where an error means the
// fixture wiring is broken, not the behaviour under test.
func must2[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// refreshCookieIsDeletion reports whether the response instructs the
// browser to DELETE the refresh_token cookie — i.e. it Set-Cookie's
// refresh_token with a non-positive Max-Age (the form clearRefreshCookie
// emits, Max-Age=-1). Returns false when no refresh cookie is set at all.
func refreshCookieIsDeletion(rr *httptest.ResponseRecorder) bool {
	for _, sc := range rr.Result().Cookies() {
		if sc.Name != "refresh_token" {
			continue
		}
		// MaxAge < 0 means "delete now"; MaxAge == 0 with an empty value
		// is also a deletion as emitted via http.Cookie.
		return sc.MaxAge < 0 || (sc.MaxAge == 0 && sc.Value == "")
	}
	return false
}

// startImpersonationGetJTI runs a real impersonation start as admin→target
// and returns the started session's access token and its jti claim. Used
// by the FIX 2 test to forge an EXPIRED token bound to the SAME jti as a
// live return slot.
func startImpersonationGetJTI(c *qt.C, handler http.Handler, adminID, targetID string) (accessToken, jti string) {
	c.Helper()
	startRR := doImpersonateStart(c, handler, adminID, targetID, nil)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)
	claims := parseTestTokenClaims(c, startResp.AccessToken)
	jtiClaim, ok := claims["jti"].(string)
	c.Assert(ok, qt.IsTrue)
	return startResp.AccessToken, jtiClaim
}

// TestImpersonate_LogoutDuringImpersonationRevokesAdminToken is the
// regression test for PR #1771 FIX 1: an operator who logs out WHILE
// impersonating must have their genuine admin refresh token revoked.
// During impersonation the refresh cookie holds the `imp:<jti>` marker, so
// the plain logout path would hash the marker, find no DB row, and leave
// the admin's real refresh token valid for its full 30-day lifetime — a
// live credential the operator believes they terminated. logout must
// instead resolve the return slot and revoke slot.AdminRefreshTokenRaw.
func TestImpersonate_LogoutDuringImpersonationRevokesAdminToken(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// The admin has a genuine, refreshable session before impersonating.
	adminRefreshRaw := createAdminRefreshToken(c, params, admin)

	// Start: send the admin's refresh cookie so the return slot captures
	// the genuine token; the start handler replies with the imp: marker.
	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+target.ID+"/impersonate", bytes.NewReader(nil))
	startReq.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(startReq, admin.ID)
	// #nosec G124 -- test request cookie.
	startReq.AddCookie(&http.Cookie{Name: "refresh_token", Value: adminRefreshRaw})
	startRR := httptest.NewRecorder()
	handler.ServeHTTP(startRR, startReq)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)
	markerCookie, ok := refreshCookieFromResponse(startRR)
	c.Assert(ok, qt.IsTrue)
	c.Assert(strings.HasPrefix(markerCookie, "imp:"), qt.IsTrue)

	// Sanity check: the admin's genuine token is refreshable RIGHT NOW.
	preReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	// #nosec G124 -- test request cookie.
	preReq.AddCookie(&http.Cookie{Name: "refresh_token", Value: adminRefreshRaw})
	preRR := httptest.NewRecorder()
	handler.ServeHTTP(preRR, preReq)
	c.Assert(preRR.Code, qt.Equals, http.StatusOK)

	// The operator logs out from inside the impersonation session — the
	// browser sends the imp: marker cookie, NOT the genuine token.
	logoutReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+startResp.AccessToken)
	// #nosec G124 -- test request cookie.
	logoutReq.AddCookie(&http.Cookie{Name: "refresh_token", Value: markerCookie})
	logoutRR := httptest.NewRecorder()
	handler.ServeHTTP(logoutRR, logoutReq)
	c.Assert(logoutRR.Code, qt.Equals, http.StatusOK)

	// The admin's GENUINE refresh token must now be revoked: a cookie-based
	// /auth/refresh with the original raw token fails.
	postReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	// #nosec G124 -- test request cookie.
	postReq.AddCookie(&http.Cookie{Name: "refresh_token", Value: adminRefreshRaw})
	postRR := httptest.NewRecorder()
	handler.ServeHTTP(postRR, postReq)
	c.Assert(postRR.Code, qt.Equals, http.StatusUnauthorized,
		qt.Commentf("admin's genuine refresh token must be revoked after logout-during-impersonation"))
}

// TestImpersonate_EndAcceptsExpiredTokenWithLiveSlot is the regression
// test for PR #1771 FIX 2: an operator who lets the impersonation access
// token expire (idle) must still be able to POST /admin/impersonation/end
// and be restored — JWTMiddleware no longer 401s the expired token before
// the handler runs, and endImpersonation tolerates an expired `exp` while
// still verifying the signature + imp=true and proving authorization via
// the live server-side slot.
func TestImpersonate_EndAcceptsExpiredTokenWithLiveSlot(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Start a genuine session so the return slot exists, and capture its jti.
	_, jti := startImpersonationGetJTI(c, handler, admin.ID, target.ID)

	// Forge an EXPIRED impersonation token bound to the SAME jti — signed
	// with the real test secret, so the signature is valid; only `exp` is
	// in the past. This is the "operator went idle" shape.
	expiredToken := makeImpersonationToken(c, impersonateClaims{
		jti:            jti,
		targetUserID:   target.ID,
		targetTenantID: target.TenantID,
		adminUserID:    admin.ID,
		expiresAt:      time.Now().Add(-time.Minute),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/impersonation/end", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// The expired token still ends the session and restores the admin.
	c.Assert(rr.Code, qt.Equals, http.StatusOK,
		qt.Commentf("expired imp token with a live slot must still end the session; body=%s", rr.Body.String()))
	var endResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &endResp), qt.IsNil)
	c.Assert(endResp.User, qt.IsNotNil)
	c.Assert(endResp.User.ID, qt.Equals, admin.ID)
	endClaims := parseTestTokenClaims(c, endResp.AccessToken)
	c.Assert(endClaims["user_id"], qt.Equals, admin.ID)
	_, hasImp := endClaims["imp"]
	c.Assert(hasImp, qt.IsFalse)
}

// TestImpersonate_EndStillRejectsExpiredNonImpersonationToken guards the
// FIX 2 security boundary: relaxing `exp` must NOT let an expired PLAIN
// (non-imp) admin token reach a successful end. parseImpersonationEndToken
// requires imp=true, so an expired admin token is rejected with 422
// not_active just as a fresh one is.
func TestImpersonate_EndStillRejectsExpiredNonImpersonationToken(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// A plain (non-imp) admin token that has already expired.
	expiredAdmin := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":             "expired-admin-jti",
		"user_id":         admin.ID,
		"is_system_admin": true,
		"iat":             time.Now().Add(-2 * time.Hour).Unix(),
		"exp":             time.Now().Add(-time.Hour).Unix(),
	})
	signed, err := expiredAdmin.SignedString(testJWTSecret)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/impersonation/end", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateNotActiveCode)
}

// TestImpersonate_EndRejectsForgedToken guards the other FIX 2 security
// boundary: a token with a BAD signature must be rejected even when it
// claims imp=true. The expiry relaxation never bypasses signature
// verification.
func TestImpersonate_EndRejectsForgedToken(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// An imp token signed with the WRONG secret — a forgery.
	forged := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":             "forged-jti",
		"user_id":         target.ID,
		"tenant_id":       target.TenantID,
		"impersonated_by": admin.ID,
		"imp":             true,
		"is_system_admin": false,
		"iat":             time.Now().Add(-time.Minute).Unix(),
		"exp":             time.Now().Add(20 * time.Minute).Unix(),
	})
	signed, err := forged.SignedString([]byte("a-totally-different-32byte-secret-xx"))
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/impersonation/end", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateNotActiveCode)
}

// TestImpersonate_EndClearsMarkerCookieForPureBearerStart proves the
// pure-bearer path: when the admin started impersonation WITHOUT a refresh
// cookie (an API/test client), there is no genuine token to restore on
// `end`, so the handler must instead DELETE the marker cookie it planted
// at start — leaving no stale imp: marker in the client.
func TestImpersonate_EndClearsMarkerCookieForPureBearerStart(t *testing.T) {
	c := qt.New(t)
	params, admin, _ := newParams()
	promoteToSystemAdmin(c, params, admin)
	target := createTestUserDirect(c, params, admin.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Start WITHOUT a refresh cookie — doImpersonateStart sends only the
	// bearer auth header, so slot.AdminRefreshTokenRaw is empty.
	accessToken, _ := startImpersonationGetJTI(c, handler, admin.ID, target.ID)

	endReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/impersonation/end", nil)
	endReq.Header.Set("Authorization", "Bearer "+accessToken)
	endRR := httptest.NewRecorder()
	handler.ServeHTTP(endRR, endReq)
	c.Assert(endRR.Code, qt.Equals, http.StatusOK)

	// With no genuine token to restore, `end` must clear the marker cookie.
	c.Assert(refreshCookieIsDeletion(endRR), qt.IsTrue,
		qt.Commentf("pure-bearer end must delete the marker refresh cookie"))
}
