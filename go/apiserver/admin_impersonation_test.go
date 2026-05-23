package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/csrf"
	csrfinmemory "github.com/denisvmedia/inventario/csrf/inmemory"
	"github.com/denisvmedia/inventario/models"
)

// Cross-plane impersonation tests (#1785 Phase 5). The
// /api/v1/admin/users/{id}/impersonate surface was cut over from a
// tenant operator (RequireSystemAdmin) to a back-office operator
// (RequireBackofficeAuth + RequirePlatformAdmin). The end /current
// trio likewise moved off the tenant gate — `end` restores the
// operator's back-office session, `current` accepts EITHER a
// back-office JWT or an active impersonation token. The legacy
// `imp:<jti>` marker refresh cookie is gone: the JTI-keyed return slot
// (slot.OperatorUserID + OperatorKind match) is the only binding.
//
// Tests reuse the back-office helpers from admin_routes_test.go
// (WithBackofficeAdmin, addBackofficeAuthHeader) and the tenant user
// fixtures from admin_users_test.go (createTestUserDirect).

// withBackofficeOperator seeds a back-office operator with the given
// role and returns the persisted row alongside a signed access token.
// Mirrors WithBackofficeAdmin (which forces platform_admin) but lets
// the test pick the role — used by the role-gating tests.
func withBackofficeOperator(t *testing.T, params apiserver.Params, role models.BackofficeRole) (*models.BackofficeUser, string) {
	t.Helper()
	c := qt.New(t)

	hash, err := bcrypt.GenerateFromPassword([]byte("UnusedTestPassword1!"), bcrypt.MinCost)
	c.Assert(err, qt.IsNil)

	u := models.BackofficeUser{
		Email:        fmt.Sprintf("ops-%d-%s@example.com", time.Now().UnixNano(), role),
		Name:         "Test Operator",
		PasswordHash: string(hash),
		Role:         role,
		IsActive:     true,
		MFAEnforced:  false,
	}
	created, err := params.FactorySet.BackofficeUserRegistry.Create(context.Background(), u)
	c.Assert(err, qt.IsNil)
	return created, signBackofficeAccessToken(t, created.ID, string(created.Role))
}

// impersonateClaims is the minimal claim set for a forged impersonation
// access token used by tests that need to exercise end / current
// without first calling the start endpoint.
type impersonateClaims struct {
	jti            string
	targetUserID   string
	targetTenantID string
	operatorID     string
	expiresAt      time.Time
}

// makeImpersonationToken signs an impersonation access token with the
// shared test JWT secret. Mirrors signImpersonationToken so tests can
// forge a token directly (e.g. an already-expired one).
func makeImpersonationToken(c *qt.C, cl impersonateClaims) string {
	c.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":               cl.jti,
		"user_id":           cl.targetUserID,
		"tenant_id":         cl.targetTenantID,
		"impersonator_id":   cl.operatorID,
		"impersonator_type": "backoffice_user",
		"imp":               true,
		"is_system_admin":   false,
		"token_type":        "access",
		"iat":               time.Now().Add(-time.Minute).Unix(),
		"exp":               cl.expiresAt.Unix(),
	})
	signed, err := token.SignedString(testJWTSecret)
	c.Assert(err, qt.IsNil)
	return signed
}

// doImpersonateStart issues POST /admin/users/{id}/impersonate as the
// back-office operator behind the given token and returns the recorder.
func doImpersonateStart(c *qt.C, handler http.Handler, operatorToken, targetID string, body map[string]any) *httptest.ResponseRecorder {
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
	addBackofficeAuthHeader(req, operatorToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestImpersonate_StartSucceedsForBackofficePlatformAdmin(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	operator, operatorToken := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, operatorToken, target.ID, map[string]any{"reason": "support ticket #42"})

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var resp apiserver.LoginResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.AccessToken, qt.Not(qt.Equals), "")
	c.Assert(resp.TokenType, qt.Equals, "Bearer")
	c.Assert(resp.User, qt.IsNotNil)
	c.Assert(resp.User.ID, qt.Equals, target.ID)

	// The issued token must carry the cross-plane impersonation claims:
	// imp=true, impersonator_id = back-office operator id,
	// impersonator_type = backoffice_user, is_system_admin = false.
	claims := parseTestTokenClaims(c, resp.AccessToken)
	c.Assert(claims["user_id"], qt.Equals, target.ID)
	c.Assert(claims["impersonator_id"], qt.Equals, operator.ID)
	c.Assert(claims["impersonator_type"], qt.Equals, "backoffice_user")
	c.Assert(claims["imp"], qt.Equals, true)
	c.Assert(claims["is_system_admin"], qt.Equals, false)
}

// TestImpersonate_RejectsSupportAgent is the regression test for the
// Phase 5 role split: a back-office user with role=support_agent
// authenticates fine on the back-office plane but RequirePlatformAdmin
// returns 403 + admin.role_required at the impersonation-start route.
// Every other admin surface (read-only listings, group reads, …) stays
// reachable for support_agent — only impersonation-start is gated.
func TestImpersonate_RejectsSupportAgent(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	_, supportToken := withBackofficeOperator(t, params, models.BackofficeRoleSupportAgent)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, supportToken, target.ID, nil)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminRoleRequiredCode)
}

func TestImpersonate_StartWithoutBodySucceeds(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	_, operatorToken := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, operatorToken, target.ID, nil)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
}

func TestImpersonate_RejectsOverLongReason(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	_, operatorToken := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, operatorToken, target.ID,
		map[string]any{"reason": strings.Repeat("x", 501)})

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateReasonTooLongCode)
}

// TestImpersonate_RejectsTenantJWT proves the back-office plane gate:
// a tenant JWT — even an IsSystemAdmin=true one — must not reach the
// impersonation start route. The wire response is the back-office
// middleware's plain-text 401.
func TestImpersonate_RejectsTenantJWT(t *testing.T) {
	c := qt.New(t)
	params, tenantUser, _ := newParams()
	promoteToSystemAdmin(c, params, tenantUser)
	target := createTestUserDirect(c, params, tenantUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/impersonate", bytes.NewReader(nil))
	addTestUserAuthHeader(req, tenantUser.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

func TestImpersonate_RejectsTargetAdmin(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	_, operatorToken := WithBackofficeAdmin(t, params)
	targetAdmin := createTestUserDirect(c, params, baseUser.TenantID, "peer-admin@example.com", true, true)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, operatorToken, targetAdmin.ID, nil)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateTargetIsAdminCode)
}

func TestImpersonate_RejectsBlockedTarget(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	_, operatorToken := WithBackofficeAdmin(t, params)
	blocked := createTestUserDirect(c, params, baseUser.TenantID, "blocked@example.com", false, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, operatorToken, blocked.ID, nil)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateTargetBlockedCode)
}

func TestImpersonate_RejectsUnknownTarget(t *testing.T) {
	c := qt.New(t)
	params, _, _ := newParams()
	_, operatorToken := WithBackofficeAdmin(t, params)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, operatorToken, "00000000-0000-0000-0000-000000000000", nil)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

// TestImpersonate_StartFromInsideImpersonationRejected is the
// nested-impersonation guard: an impersonation token (tenant JWT with
// imp=true) presented at the start route is rejected by
// RequireBackofficeAuth first (audience mismatch — the tenant token
// has no aud="backoffice" claim), so the request never reaches the
// handler. The wire result is the back-office middleware's 401.
func TestImpersonate_StartFromInsideImpersonationRejected(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	_, operatorToken := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	secondTarget := createTestUserDirect(c, params, baseUser.TenantID, "second@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	startRR := doImpersonateStart(c, handler, operatorToken, target.ID, nil)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)

	// Replay the impersonation token at the start route — the back-
	// office gate rejects it because the impersonation token is a
	// tenant JWT (no aud=backoffice).
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/admin/users/"+secondTarget.ID+"/impersonate", nil)
	req.Header.Set("Authorization", "Bearer "+startResp.AccessToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

// TestImpersonate_RefreshRejectsImpersonationToken: the tenant
// /auth/refresh endpoint must still refuse an impersonation token.
func TestImpersonate_RefreshRejectsImpersonationToken(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	operator, _ := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	impToken := makeImpersonationToken(c, impersonateClaims{
		jti:            "refresh-test-jti",
		targetUserID:   target.ID,
		targetTenantID: target.TenantID,
		operatorID:     operator.ID,
		expiresAt:      time.Now().Add(20 * time.Minute),
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+impToken)
	// #nosec G124 -- test request cookie; Secure/SameSite are irrelevant.
	req.AddCookie(&http.Cookie{Name: "refresh_token", Value: "some-refresh-token"})
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

// TestImpersonateEnd_RestoresBackofficeSession is the core Phase 5
// regression: calling `end` with the impersonation token restores the
// back-office operator's session (back-office access token, back-office
// refresh cookie at /api/v1/backoffice) — NOT a tenant session.
func TestImpersonateEnd_RestoresBackofficeSession(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	operator, operatorToken := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Start a real impersonation session so the return slot exists.
	startRR := doImpersonateStart(c, handler, operatorToken, target.ID, nil)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)

	// The start handler must NOT plant a tenant refresh cookie any
	// more — Phase 5 dropped the imp: marker entirely. A pure
	// httptest.ResponseRecorder check on the cookie suffices.
	cookieValue, hasCookie := tenantRefreshCookieFromResponse(startRR)
	c.Assert(hasCookie, qt.IsFalse,
		qt.Commentf("Phase 5 must not plant a tenant refresh cookie at start (got %q)", cookieValue))

	// End the session using the impersonation token. No marker cookie
	// is required — the JTI-keyed slot is the only binding.
	endRR := doImpersonateEnd(handler, startResp.AccessToken)
	c.Assert(endRR.Code, qt.Equals, http.StatusOK,
		qt.Commentf("end body=%s", endRR.Body.String()))

	// The end response is a BACK-OFFICE login response carrying a
	// back-office access token (aud=backoffice + admin_id claim).
	var endResp apiserver.BackofficeLoginResponse
	c.Assert(json.Unmarshal(endRR.Body.Bytes(), &endResp), qt.IsNil)
	c.Assert(endResp.AccessToken, qt.Not(qt.Equals), "")
	c.Assert(endResp.TokenType, qt.Equals, "Bearer")
	c.Assert(endResp.User, qt.IsNotNil)
	c.Assert(endResp.User.ID, qt.Equals, operator.ID)
	c.Assert(endResp.User.Role, qt.Equals, string(models.BackofficeRolePlatformAdmin))

	endClaims := parseTestTokenClaims(c, endResp.AccessToken)
	c.Assert(endClaims["admin_id"], qt.Equals, operator.ID)
	c.Assert(endClaims["aud"], qt.Equals, "backoffice")
	c.Assert(endClaims["role"], qt.Equals, "platform_admin")
	_, hasImp := endClaims["imp"]
	c.Assert(hasImp, qt.IsFalse)

	// The admin.impersonate_end audit row must record actor = target
	// (UserID = targetID — the request ran under the imp token) AND
	// impersonated_by = operator.ID (auto-filled by LogAdmin via the
	// impersonator_id claim).
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
	c.Assert(*endRow.ImpersonatedBy, qt.Equals, operator.ID)
}

func TestImpersonate_EndRejectsNonImpersonationToken(t *testing.T) {
	c := qt.New(t)
	params, _, _ := newParams()
	_, operatorToken := WithBackofficeAdmin(t, params)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// A plain back-office token (no imp claim) at the end endpoint is
	// an authentication failure — not an impersonation token — and is
	// rejected with 401, distinct from the 422 reserved for a validly-
	// signed impersonation token with no active session.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/impersonation/end", nil)
	addBackofficeAuthHeader(req, operatorToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

// TestImpersonate_ExpiredTokenWithNoSlotEndsAsNotActive: an expired
// imp token whose return slot was never recorded (here, a forged
// token) still reaches endImpersonation now that `end` is mounted
// bare — but with no slot to restore, the only correct answer is 422
// not_active. The expiry relaxation does NOT turn a slot-less token
// into a successful end; the server-side slot is still the real
// proof. See TestImpersonate_EndAcceptsExpiredTokenWithLiveSlot for
// the expired-but-restorable case.
func TestImpersonate_ExpiredTokenWithNoSlotEndsAsNotActive(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	operator, _ := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	expiredToken := makeImpersonationToken(c, impersonateClaims{
		jti:            "expired-jti",
		targetUserID:   target.ID,
		targetTenantID: target.TenantID,
		operatorID:     operator.ID,
		expiresAt:      time.Now().Add(-time.Minute),
	})

	rr := doImpersonateEnd(handler, expiredToken)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateNotActiveCode)
}

func TestImpersonate_CurrentReportsInactiveForPlainBackofficeOperator(t *testing.T) {
	c := qt.New(t)
	params, _, _ := newParams()
	_, operatorToken := WithBackofficeAdmin(t, params)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/impersonation/current", nil)
	addBackofficeAuthHeader(req, operatorToken)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	var resp apiserver.ImpersonationStateResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Active, qt.IsFalse)
}

// TestImpersonateCurrent_AcceptsBothBackofficeAndImpersonatingTenant
// is the regression test for the widened gate: GET
// /admin/impersonation/current must answer both
//
//  1. a back-office operator who is NOT currently impersonating
//     (returns active=false), and
//  2. a tenant request running INSIDE an active impersonation session
//     (returns active=true with target + operator filled).
//
// Both shapes hit the same handler; only the gate differs.
func TestImpersonateCurrent_AcceptsBothBackofficeAndImpersonatingTenant(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	operator, operatorToken := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// (1) Back-office operator: active=false.
	boReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/impersonation/current", nil)
	addBackofficeAuthHeader(boReq, operatorToken)
	boRR := httptest.NewRecorder()
	handler.ServeHTTP(boRR, boReq)
	c.Assert(boRR.Code, qt.Equals, http.StatusOK)
	var boResp apiserver.ImpersonationStateResponse
	c.Assert(json.Unmarshal(boRR.Body.Bytes(), &boResp), qt.IsNil)
	c.Assert(boResp.Active, qt.IsFalse)

	// Start a real session so the impersonation token is bound to a
	// live slot.
	startRR := doImpersonateStart(c, handler, operatorToken, target.ID, nil)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)

	// (2) Impersonating tenant session: active=true.
	impReq := httptest.NewRequest(http.MethodGet, "/api/v1/admin/impersonation/current", nil)
	impReq.Header.Set("Authorization", "Bearer "+startResp.AccessToken)
	impRR := httptest.NewRecorder()
	handler.ServeHTTP(impRR, impReq)
	c.Assert(impRR.Code, qt.Equals, http.StatusOK)
	var impResp apiserver.ImpersonationStateResponse
	c.Assert(json.Unmarshal(impRR.Body.Bytes(), &impResp), qt.IsNil)
	c.Assert(impResp.Active, qt.IsTrue)
	c.Assert(impResp.TargetUser, qt.IsNotNil)
	c.Assert(impResp.TargetUser.ID, qt.Equals, target.ID)
	c.Assert(impResp.Operator, qt.IsNotNil)
	c.Assert(impResp.Operator.ID, qt.Equals, operator.ID)
	c.Assert(impResp.Operator.Role, qt.Equals, "platform_admin")
	c.Assert(impResp.StartedAt, qt.IsNotNil)
	c.Assert(impResp.ExpiresAt, qt.IsNotNil)
}

// TestImpersonateCurrent_RejectsBareTenantJWT proves the widened gate
// does NOT admit a plain tenant JWT (no aud=backoffice, no imp=true).
// Only a back-office token or an active impersonation token gets in.
func TestImpersonateCurrent_RejectsBareTenantJWT(t *testing.T) {
	c := qt.New(t)
	params, tenantUser, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/impersonation/current", nil)
	addTestUserAuthHeader(req, tenantUser.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

func TestImpersonate_RateLimitEnforced(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	_, operatorToken := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	var lastCode int
	for range 12 {
		rr := doImpersonateStart(c, handler, operatorToken, target.ID, nil)
		lastCode = rr.Code
		if rr.Code == http.StatusTooManyRequests {
			assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateRateLimitedCode)
			break
		}
	}
	c.Assert(lastCode, qt.Equals, http.StatusTooManyRequests,
		qt.Commentf("expected the per-operator impersonation rate limit to trip within 12 attempts"))
}

// TestImpersonate_AuditTrailRecordsImpersonator is the integration
// check from the #1750 acceptance criteria (revalidated for Phase 5):
// the impersonated session hits a (non-admin) GROUP endpoint, and the
// resulting audit-log row carries actor=target (JWT user_id) AND
// impersonated_by = back-office operator id (auto-filled from the
// impersonator_id JWT claim).
func TestImpersonate_AuditTrailRecordsImpersonator(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	operator, operatorToken := WithBackofficeAdmin(t, params)

	target := createTestUserDirect(c, params, baseUser.TenantID, "group-owner@example.com", true, false)
	group := createOwnedTestGroup(c, params, target)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	startRR := doImpersonateStart(c, handler, operatorToken, target.ID, nil)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)

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
	c.Assert(*found.ImpersonatedBy, qt.Equals, operator.ID)
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

// must2 unwraps a (T, error) pair, panicking on error. Used only in
// fixture-style registry reads where an error means the fixture wiring
// is broken, not the behaviour under test.
func must2[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// doImpersonateEnd issues POST /admin/impersonation/end with the
// supplied impersonation access token. Phase 5 dropped the
// marker-cookie binding so no cookies are sent — the JTI-keyed slot
// is the only proof of session ownership.
func doImpersonateEnd(handler http.Handler, accessToken string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/impersonation/end", nil)
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// TestImpersonate_EndAcceptsExpiredTokenWithLiveSlot: an operator who
// lets the impersonation access token expire (idle) must still be
// able to POST /admin/impersonation/end and be restored — `end` is
// mounted bare and tolerates an expired `exp` while still verifying
// the signature + imp=true and proving authorization via the live
// server-side slot.
func TestImpersonate_EndAcceptsExpiredTokenWithLiveSlot(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	operator, operatorToken := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Start a genuine session so the return slot exists; recover the
	// jti from the response token so the forged expired token binds to
	// the same slot.
	startRR := doImpersonateStart(c, handler, operatorToken, target.ID, nil)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)
	claims := parseTestTokenClaims(c, startResp.AccessToken)
	jti, ok := claims["jti"].(string)
	c.Assert(ok, qt.IsTrue)

	// Forge an EXPIRED impersonation token bound to the SAME jti —
	// signed with the real test secret, so the signature is valid;
	// only `exp` is in the past.
	expiredToken := makeImpersonationToken(c, impersonateClaims{
		jti:            jti,
		targetUserID:   target.ID,
		targetTenantID: target.TenantID,
		operatorID:     operator.ID,
		expiresAt:      time.Now().Add(-time.Minute),
	})

	rr := doImpersonateEnd(handler, expiredToken)

	c.Assert(rr.Code, qt.Equals, http.StatusOK,
		qt.Commentf("expired imp token with a live slot must still end the session; body=%s", rr.Body.String()))
	var endResp apiserver.BackofficeLoginResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &endResp), qt.IsNil)
	c.Assert(endResp.User, qt.IsNotNil)
	c.Assert(endResp.User.ID, qt.Equals, operator.ID)
	endClaims := parseTestTokenClaims(c, endResp.AccessToken)
	c.Assert(endClaims["admin_id"], qt.Equals, operator.ID)
	c.Assert(endClaims["aud"], qt.Equals, "backoffice")
}

// TestImpersonate_EndStillRejectsExpiredNonImpersonationToken: the
// expiry relaxation must NOT let an expired plain (non-imp) token
// reach a successful end. parseImpersonationEndToken requires
// imp=true, so an expired admin/backoffice token is an authentication
// failure — rejected with 401.
func TestImpersonate_EndStillRejectsExpiredNonImpersonationToken(t *testing.T) {
	c := qt.New(t)
	params, _, _ := newParams()
	operator, _ := WithBackofficeAdmin(t, params)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// A plain (non-imp) back-office token that has already expired.
	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":        "expired-admin-jti",
		"admin_id":   operator.ID,
		"aud":        "backoffice",
		"role":       "platform_admin",
		"token_type": "access",
		"iat":        time.Now().Add(-2 * time.Hour).Unix(),
		"exp":        time.Now().Add(-time.Hour).Unix(),
	})
	signed, err := expired.SignedString(testJWTSecret)
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/impersonation/end", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

// TestImpersonate_EndRejectsForgedToken: a token with a BAD signature
// must be rejected even when it claims imp=true. The expiry relaxation
// never bypasses signature verification.
func TestImpersonate_EndRejectsForgedToken(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	operator, _ := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	forged := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":               "forged-jti",
		"user_id":           target.ID,
		"tenant_id":         target.TenantID,
		"impersonator_id":   operator.ID,
		"impersonator_type": "backoffice_user",
		"imp":               true,
		"is_system_admin":   false,
		"iat":               time.Now().Add(-time.Minute).Unix(),
		"exp":               time.Now().Add(20 * time.Minute).Unix(),
	})
	signed, err := forged.SignedString([]byte("a-totally-different-32byte-secret-xx"))
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/impersonation/end", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

// TestImpersonate_EndRejectsOperatorMismatch: a forged imp token bound
// to a DIFFERENT operator id than the slot's must be refused at end —
// the slot is the proof, and an operator-mismatch is treated as
// not_active (the slot does not belong to this token's operator).
func TestImpersonate_EndRejectsOperatorMismatch(t *testing.T) {
	c := qt.New(t)
	params, baseUser, _ := newParams()
	_, operatorToken := WithBackofficeAdmin(t, params)
	otherOperator, _ := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseUser.TenantID, "target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	startRR := doImpersonateStart(c, handler, operatorToken, target.ID, nil)
	c.Assert(startRR.Code, qt.Equals, http.StatusOK)
	var startResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(startRR.Body.Bytes(), &startResp), qt.IsNil)
	claims := parseTestTokenClaims(c, startResp.AccessToken)
	jti, ok := claims["jti"].(string)
	c.Assert(ok, qt.IsTrue)

	// Forge a token for the SAME jti but with a different operator id.
	imposter := makeImpersonationToken(c, impersonateClaims{
		jti:            jti,
		targetUserID:   target.ID,
		targetTenantID: target.TenantID,
		operatorID:     otherOperator.ID,
		expiresAt:      time.Now().Add(20 * time.Minute),
	})

	rr := doImpersonateEnd(handler, imposter)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminImpersonateNotActiveCode)
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

// tenantRefreshCookieFromResponse extracts the value the response set
// the LIVE tenant `refresh_token` cookie to. Returns ("", false) when
// no live tenant refresh cookie is set. Skips the legacy-path deletion
// cookie that writeRefreshCookie historically emits (empty value,
// Max-Age=0) so callers see the value a browser would actually carry
// forward. Used by Phase 5's "no tenant cookie at start" assertion.
func tenantRefreshCookieFromResponse(rr *httptest.ResponseRecorder) (string, bool) {
	for _, sc := range rr.Header().Values("Set-Cookie") {
		if !strings.HasPrefix(sc, "refresh_token=") {
			continue
		}
		if strings.HasPrefix(sc, "refresh_token=;") || strings.Contains(sc, "; Max-Age=0") {
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

// newParamsWithCSRF builds the standard test params plus an in-memory
// CSRF service so the impersonation start response carries a real
// (non-empty) csrf_token for the target identity. Returns the service
// too so callers can mint a request CSRF token (the start route is
// CSRF-protected once a real service is wired).
func newParamsWithCSRF(c *qt.C) (apiserver.Params, csrf.Service) {
	c.Helper()
	params, _, _ := newParams()
	csrfSvc := csrfinmemory.New()
	params.CSRFService = csrfSvc
	return params, csrfSvc
}

// TestImpersonate_StartResponseCarriesCSRFToken: CSRF validation on
// the tenant plane is per-user, so the start response must hand the
// SPA a CSRF token minted for the TARGET (the impersonated identity)
// or the impersonated session's first mutating request 403s. The back-
// office plane is not CSRF-protected, so the start REQUEST itself
// needs no X-CSRF-Token header.
func TestImpersonate_StartResponseCarriesCSRFToken(t *testing.T) {
	c := qt.New(t)
	params, _ := newParamsWithCSRF(c)
	_, operatorToken := WithBackofficeAdmin(t, params)
	target := createTestUserDirect(c, params, baseTenantIDFor(c, params),
		"target@example.com", true, false)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	rr := doImpersonateStart(c, handler, operatorToken, target.ID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var resp apiserver.LoginResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.CSRFToken, qt.Not(qt.Equals), "",
		qt.Commentf("start response body must carry a CSRF token for the target"))
	c.Assert(rr.Header().Get("X-CSRF-Token"), qt.Equals, resp.CSRFToken,
		qt.Commentf("start must mirror the CSRF token into the X-CSRF-Token header"))
}

// baseTenantIDFor returns the seeded default tenant id from a
// newParams()-style fixture. Used by tests that built params via
// newParamsWithCSRF (which discards the user pointer) and still need
// to create tenant users.
func baseTenantIDFor(c *qt.C, params apiserver.Params) string {
	c.Helper()
	tenants, err := params.FactorySet.TenantRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(tenants, qt.Not(qt.HasLen), 0)
	return tenants[0].ID
}
