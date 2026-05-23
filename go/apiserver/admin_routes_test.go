package apiserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
)

// promoteToSystemAdmin grants the seeded test user system-admin via
// the system_admin_grants registry (#1784). After the #1785 Phase 3
// migration the admin CRUD gate is RequireBackofficeAuth, NOT
// RequireSystemAdmin — so the grant is only load-bearing for tests
// that exercise the legacy impersonation routes (which still run under
// the tenant gate). Cross-tenant CRUD tests use WithBackofficeAdmin
// instead. Idempotent.
func promoteToSystemAdmin(c *qt.C, params apiserver.Params, user *models.User) {
	c.Helper()
	must.Must(params.FactorySet.SystemAdminGrantRegistry.Grant(context.Background(), user.ID, nil))
}

// WithBackofficeAdmin seeds an active back-office admin in
// params.FactorySet.BackofficeUserRegistry and returns the persisted
// row alongside a signed back-office access token. The token reuses the
// shared testJWTSecret because production runs the tenant + back-office
// planes on the same JWT secret — `aud` is the boundary, not the key.
// Use this helper (together with addBackofficeAuthHeader) wherever a
// test needs to hit a /api/v1/admin/* CRUD endpoint after the #1785
// Phase 3 migration.
func WithBackofficeAdmin(t *testing.T, params apiserver.Params) (*models.BackofficeUser, string) {
	t.Helper()
	c := qt.New(t)

	hash, err := bcrypt.GenerateFromPassword([]byte("UnusedTestPassword1!"), bcrypt.DefaultCost)
	c.Assert(err, qt.IsNil)

	u := models.BackofficeUser{
		// Email is unique-per-row at the registry layer; suffix with a
		// monotonic nanosecond so repeated calls inside one test (or
		// across two tests sharing a registry) never collide.
		Email:        fmt.Sprintf("ops-%d@example.com", time.Now().UnixNano()),
		Name:         "Test Operator",
		PasswordHash: string(hash),
		Role:         models.BackofficeRolePlatformAdmin,
		IsActive:     true,
		MFAEnforced:  false,
	}
	created, err := params.FactorySet.BackofficeUserRegistry.Create(context.Background(), u)
	c.Assert(err, qt.IsNil)

	token := signBackofficeAccessToken(t, created.ID, string(created.Role))
	return created, token
}

// signBackofficeAccessToken mints a `aud=backoffice` access token
// stamped with the supplied admin_id + role and signed with the shared
// testJWTSecret. Mirrors the production back-office login mint at the
// claim level — token_type=access, jti, iat, exp — so the token
// validates cleanly through RequireBackofficeAuth.
//
// Note: omits rti claim — fine for admin CRUD tests; refresh-path tests
// must set rti explicitly.
func signBackofficeAccessToken(t *testing.T, adminID, role string) string {
	t.Helper()
	now := time.Now()
	claims := jwt.MapClaims{
		"jti":        fmt.Sprintf("test-%d", now.UnixNano()),
		"admin_id":   adminID,
		"aud":        "backoffice",
		"role":       role,
		"token_type": "access",
		"iat":        now.Unix(),
		"exp":        now.Add(time.Hour).Unix(),
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(testJWTSecret)
	if err != nil {
		t.Fatalf("signBackofficeAccessToken: %v", err)
	}
	return signed
}

// addBackofficeAuthHeader sets a Bearer back-office access token on the
// request. Drop-in replacement for addTestUserAuthHeader at every admin
// CRUD call site after the #1785 Phase 3 migration.
func addBackofficeAuthHeader(req *http.Request, token string) {
	req.Header.Set("Authorization", "Bearer "+token)
}

// backofficeAdminForToken parses the supplied back-office token and
// loads the matching BackofficeUser row from the params' registry. Used
// by audit-row assertions that need the actor's persisted ID: a test
// that built its admin via WithBackofficeAdmin but discarded the user
// pointer can recover it without re-wiring the helper signature.
func backofficeAdminForToken(t *testing.T, params apiserver.Params, token string) *models.BackofficeUser {
	t.Helper()
	c := qt.New(t)
	parsed, err := jwt.Parse(token, func(_ *jwt.Token) (any, error) { return testJWTSecret, nil })
	c.Assert(err, qt.IsNil)
	claims, ok := parsed.Claims.(jwt.MapClaims)
	c.Assert(ok, qt.IsTrue)
	adminID, _ := claims["admin_id"].(string)
	c.Assert(adminID, qt.Not(qt.Equals), "")
	admin, err := params.FactorySet.BackofficeUserRegistry.Get(context.Background(), adminID)
	c.Assert(err, qt.IsNil)
	return admin
}

func TestAdminPing_DeniesTenantUser(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/_ping", nil)
	// Tenant JWT — RequireBackofficeAuth rejects it (audience mismatch /
	// missing admin_id), regardless of the user's IsSystemAdmin flag.
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

func TestAdminPing_AllowsBackofficeAdmin(t *testing.T) {
	c := qt.New(t)
	params, _, _ := newParams()
	_, token := WithBackofficeAdmin(t, params)
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/_ping", nil)
	addBackofficeAuthHeader(req, token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var body apiserver.AdminPingResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	c.Assert(body.Ok, qt.IsTrue)
	c.Assert(body.Timestamp.IsZero(), qt.IsFalse)
}

func TestAdminPing_DeniesUnauthenticated(t *testing.T) {
	c := qt.New(t)
	params, _, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/_ping", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}
