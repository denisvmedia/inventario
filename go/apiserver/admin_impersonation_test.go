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

func TestImpersonate_ExpiredTokenRejectedAtEnd(t *testing.T) {
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

	// An expired token is rejected by JWTMiddleware before the handler runs.
	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
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
