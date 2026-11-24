package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/csrf"
	csrfinmemory "go.5x5.cz/inventario/csrf/inmemory"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// These drive the real router, so every rule below is checked with the whole
// middleware chain in the path: which routes accept a token at all, the
// token's scope and group, the owner's live role, CSRF for sessions, rate
// limits and the audit rows.

const apiTokenTestPassword = "Password123"

type apiTokenEnv struct {
	c       *qt.C
	params  apiserver.Params
	handler http.Handler
	csrf    csrf.Service
	user    *models.User
	group   *models.LocationGroup
}

func newAPITokenEnv(c *qt.C, mutate ...func(*apiserver.Params)) *apiTokenEnv {
	c.Helper()
	params, user, group := newParams()
	csrfSvc := csrfinmemory.New()
	params.CSRFService = csrfSvc
	for _, m := range mutate {
		m(&params)
	}
	return &apiTokenEnv{
		c:       c,
		params:  params,
		handler: apiserver.APIServer(params, &mockRestoreWorker{}),
		csrf:    csrfSvc,
		user:    user,
		group:   group,
	}
}

type apiTokenRequest struct {
	method  string
	path    string
	body    any
	headers map[string][]string
	cookies []*http.Cookie
}

func (e *apiTokenEnv) do(req apiTokenRequest) *httptest.ResponseRecorder {
	e.c.Helper()
	var reader *bytes.Reader
	if req.body != nil {
		reader = bytes.NewReader(must.Must(json.Marshal(req.body)))
	} else {
		reader = bytes.NewReader(nil)
	}
	r := httptest.NewRequest(req.method, req.path, reader)
	r.Header.Set("Content-Type", "application/json")
	for k, vs := range req.headers {
		for _, v := range vs {
			r.Header.Add(k, v)
		}
	}
	for _, ck := range req.cookies {
		r.AddCookie(ck)
	}
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, r)
	return rr
}

// session sends a request as the user's browser session: a JWT access token
// and a valid CSRF token.
func (e *apiTokenEnv) session(method, path string, body any) *httptest.ResponseRecorder {
	e.c.Helper()
	return e.do(apiTokenRequest{method: method, path: path, body: body, headers: map[string][]string{
		"Authorization": {"Bearer " + createTestJWTToken(e.user.ID)},
		"X-CSRF-Token":  {must.Must(e.csrf.GenerateToken(context.Background(), e.user.ID))},
	}})
}

// sessionWithoutCSRF is session minus the CSRF header.
func (e *apiTokenEnv) sessionWithoutCSRF(method, path string, body any) *httptest.ResponseRecorder {
	e.c.Helper()
	return e.do(apiTokenRequest{method: method, path: path, body: body, headers: map[string][]string{
		"Authorization": {"Bearer " + createTestJWTToken(e.user.ID)},
	}})
}

// withToken sends a request authenticated by a personal API token and nothing
// else: no cookie, no CSRF header.
func (e *apiTokenEnv) withToken(token, method, path string, body any) *httptest.ResponseRecorder {
	e.c.Helper()
	return e.do(apiTokenRequest{
		method:  method,
		path:    path,
		body:    body,
		headers: map[string][]string{"Authorization": {"Bearer " + token}},
	})
}

// mint creates a token through the HTTP endpoint, as the settings page does.
func (e *apiTokenEnv) mint(fields map[string]any) apiserver.CreateAPITokenResponse {
	e.c.Helper()
	body := map[string]any{"name": "script", "scope": "read_only", "current_password": apiTokenTestPassword}
	maps.Copy(body, fields)
	rr := e.session(http.MethodPost, "/api/v1/users/me/api-tokens", body)
	e.c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("create token: %s", rr.Body.String()))
	var resp apiserver.CreateAPITokenResponse
	e.c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	return resp
}

// insert stores a token directly in the registry, for states the create
// endpoint refuses to produce (an expiry in the past).
func (e *apiTokenEnv) insert(mutate func(*models.APIToken)) string {
	e.c.Helper()
	gen := must.Must(models.GenerateAPIToken())
	tk := models.APIToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: e.user.TenantID, UserID: e.user.ID},
		Name:                    "inserted",
		TokenHash:               gen.Hash,
		TokenPrefix:             gen.DisplayPrefix,
		Scope:                   models.APITokenScopeReadWrite,
	}
	if mutate != nil {
		mutate(&tk)
	}
	must.Must(e.params.FactorySet.APITokenRegistry.Create(context.Background(), tk))
	return gen.Token
}

func (e *apiTokenEnv) groupPath(group *models.LocationGroup, suffix string) string {
	return "/api/v1/g/" + group.Slug + suffix
}

func (e *apiTokenEnv) newLocationBody(name string) *jsonapi.LocationRequest {
	return &jsonapi.LocationRequest{
		Data: &jsonapi.LocationData{
			Type:       "locations",
			Attributes: &models.Location{Name: name, Address: "Somewhere"},
		},
	}
}

// addGroup creates another active group and makes the user a member with role.
func (e *apiTokenEnv) addGroup(role models.GroupRole) *models.LocationGroup {
	e.c.Helper()
	ctx := context.Background()
	group := must.Must(e.params.FactorySet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: e.user.TenantID},
		Name:                "Second Group",
		Slug:                must.Must(models.GenerateGroupSlug()),
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           e.user.ID,
		GroupCurrency:       models.Currency("USD"),
	}))
	must.Must(e.params.FactorySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: e.user.TenantID},
		GroupID:             group.ID,
		MemberUserID:        e.user.ID,
		Role:                role,
	}))
	return group
}

func (e *apiTokenEnv) setRole(group *models.LocationGroup, role models.GroupRole) {
	e.c.Helper()
	ctx := context.Background()
	memberships := must.Must(e.params.FactorySet.GroupMembershipRegistry.ListByUser(ctx, e.user.TenantID, e.user.ID))
	for _, m := range memberships {
		if m.GroupID == group.ID {
			m.Role = role
			must.Must(e.params.FactorySet.GroupMembershipRegistry.Update(ctx, *m))
			return
		}
	}
	e.c.Fatalf("user is not a member of group %s", group.ID)
}

func (e *apiTokenEnv) auditRows(action string) []*models.AuditLog {
	e.c.Helper()
	return must.Must(e.params.FactorySet.AuditLogRegistry.ListByAction(context.Background(), action))
}

func errorCode(c *qt.C, rr *httptest.ResponseRecorder) string {
	c.Helper()
	var body struct {
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || len(body.Errors) == 0 {
		return ""
	}
	return body.Errors[0].Code
}

func TestAPITokens_CreateRevealsTheSecretOnce(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)

	rr := env.session(http.MethodPost, "/api/v1/users/me/api-tokens", map[string]any{
		"name":             "Home Assistant",
		"scope":            "read_write",
		"group_id":         env.group.ID,
		"current_password": apiTokenTestPassword,
	})
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
	c.Assert(rr.Header().Get("Cache-Control"), qt.Equals, "no-store")

	var created apiserver.CreateAPITokenResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &created), qt.IsNil)
	c.Assert(models.IsWellFormedAPIToken(created.Secret), qt.IsTrue)
	c.Assert(strings.HasPrefix(created.Secret, created.Token.TokenPrefix), qt.IsTrue)
	c.Assert(created.Token.Name, qt.Equals, "Home Assistant")
	c.Assert(created.Token.Scope, qt.Equals, models.APITokenScopeReadWrite)
	c.Assert(*created.Token.GroupID, qt.Equals, env.group.ID)
	c.Assert(rr.Body.String(), qt.Not(qt.Contains), "token_hash")

	// The list never carries the secret or its digest.
	list := env.sessionWithoutCSRF(http.MethodGet, "/api/v1/users/me/api-tokens", nil)
	c.Assert(list.Code, qt.Equals, http.StatusOK)
	c.Assert(list.Body.String(), qt.Not(qt.Contains), created.Secret)
	c.Assert(list.Body.String(), qt.Not(qt.Contains), models.HashAPIToken(created.Secret))
	c.Assert(list.Body.String(), qt.Not(qt.Contains), "token_hash")
	var resp apiserver.APITokensListResponse
	c.Assert(json.Unmarshal(list.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Tokens, qt.HasLen, 1)
	c.Assert(resp.Tokens[0].ID, qt.Equals, created.Token.ID)
	c.Assert(resp.Tokens[0].LastUsedAt, qt.IsNil)

	rows := env.auditRows("api_token_create")
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Success, qt.IsTrue)
	c.Assert(*rows[0].EntityID, qt.Equals, created.Token.ID)
}

func TestAPITokens_CreateRequiresTheCurrentPassword(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)

	for _, password := range []string{"", "wrong-password"} {
		rr := env.session(http.MethodPost, "/api/v1/users/me/api-tokens", map[string]any{
			"name": "script", "scope": "read_only", "current_password": password,
		})
		c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
		c.Assert(errorCode(c, rr), qt.Equals, "api_token.invalid_password")
	}
	list := env.sessionWithoutCSRF(http.MethodGet, "/api/v1/users/me/api-tokens", nil)
	c.Assert(list.Body.String(), qt.Contains, `"tokens":[]`)
	c.Assert(env.auditRows("api_token_create"), qt.HasLen, 2)
}

// Session token management is a state-changing session request like any
// other: the CSRF check applies to it unchanged.
func TestAPITokens_CreateNeedsCSRF(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)

	rr := env.sessionWithoutCSRF(http.MethodPost, "/api/v1/users/me/api-tokens", map[string]any{
		"name": "script", "scope": "read_only", "current_password": apiTokenTestPassword,
	})
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
}

func TestAPITokens_CreateValidation(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)
	foreign := createTestGroupForUser(env.params.FactorySet, env.user.TenantID, "someone-else")

	tests := []struct {
		name   string
		fields map[string]any
		code   string
	}{
		{"no name", map[string]any{"name": ""}, "api_token.name_required"},
		{"bad scope", map[string]any{"scope": "admin"}, "api_token.invalid_scope"},
		{"past expiry", map[string]any{"expires_at": time.Now().Add(-time.Hour).Format(time.RFC3339)}, "api_token.expiry_in_past"},
		{"group the user is not in", map[string]any{"group_id": foreign.ID}, "api_token.group_not_allowed"},
	}
	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			body := map[string]any{"name": "script", "scope": "read_only", "current_password": apiTokenTestPassword}
			maps.Copy(body, tt.fields)
			rr := env.session(http.MethodPost, "/api/v1/users/me/api-tokens", body)
			c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf(rr.Body.String()))
			c.Assert(errorCode(c, rr), qt.Equals, tt.code)
		})
	}
}

// An account with two-factor authentication must present a fresh code to
// mint a token, and an invalid-input request must not consume it.
func TestAPITokens_CreateRequiresTheSecondFactorWhenEnabled(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)

	setup := env.session(http.MethodPost, "/api/v1/auth/mfa/setup", nil)
	c.Assert(setup.Code, qt.Equals, http.StatusOK, qt.Commentf(setup.Body.String()))
	var setupResp apiserver.MFASetupResponse
	c.Assert(json.Unmarshal(setup.Body.Bytes(), &setupResp), qt.IsNil)
	code := must.Must(totp.GenerateCodeCustom(setupResp.Secret, time.Now(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	}))
	verify := env.session(http.MethodPost, "/api/v1/auth/mfa/verify", apiserver.MFAVerifyRequest{Code: code})
	c.Assert(verify.Code, qt.Equals, http.StatusOK, qt.Commentf(verify.Body.String()))
	var verifyResp apiserver.MFAVerifyResponse
	c.Assert(json.Unmarshal(verify.Body.Bytes(), &verifyResp), qt.IsNil)
	backup := verifyResp.BackupCodes[0]

	base := map[string]any{"name": "script", "scope": "read_only", "current_password": apiTokenTestPassword}

	rr := env.session(http.MethodPost, "/api/v1/users/me/api-tokens", base)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	c.Assert(errorCode(c, rr), qt.Equals, "api_token.invalid_mfa_code")

	// Validation runs before the code is checked, so this does not burn it.
	invalid := map[string]any{"name": "", "scope": "read_only", "current_password": apiTokenTestPassword, "backup_code": backup}
	rr = env.session(http.MethodPost, "/api/v1/users/me/api-tokens", invalid)
	c.Assert(errorCode(c, rr), qt.Equals, "api_token.name_required")

	withCode := map[string]any{"name": "script", "scope": "read_only", "current_password": apiTokenTestPassword, "backup_code": backup}
	rr = env.session(http.MethodPost, "/api/v1/users/me/api-tokens", withCode)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))

	// A backup code is single-use.
	rr = env.session(http.MethodPost, "/api/v1/users/me/api-tokens", withCode)
	c.Assert(errorCode(c, rr), qt.Equals, "api_token.invalid_mfa_code")
}

func TestAPITokens_ReadWriteTokenWritesWithoutCSRF(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)
	tok := env.mint(map[string]any{"scope": "read_write"})

	list := env.withToken(tok.Secret, http.MethodGet, env.groupPath(env.group, "/locations"), nil)
	c.Assert(list.Code, qt.Equals, http.StatusOK, qt.Commentf(list.Body.String()))

	rr := env.withToken(tok.Secret, http.MethodPost, env.groupPath(env.group, "/locations"), env.newLocationBody("From a script"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
}

func TestAPITokens_ReadOnlyTokenCannotWrite(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)
	tok := env.mint(map[string]any{"scope": "read_only"})

	rr := env.withToken(tok.Secret, http.MethodGet, env.groupPath(env.group, "/locations"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	writes := []struct {
		method string
		path   string
		body   any
	}{
		{http.MethodPost, "/locations", env.newLocationBody("Nope")},
		{http.MethodPut, "/locations/any", env.newLocationBody("Nope")},
		{http.MethodPatch, "/locations/any", env.newLocationBody("Nope")},
		{http.MethodDelete, "/locations/any", nil},
		{http.MethodPost, "/uploads/files", nil},
	}
	for _, w := range writes {
		rr := env.withToken(tok.Secret, w.method, env.groupPath(env.group, w.path), w.body)
		c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("%s %s: %s", w.method, w.path, rr.Body.String()))
		c.Assert(errorCode(c, rr), qt.Equals, "api_token.read_only")
	}
}

func TestAPITokens_GroupRestriction(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)
	other := env.addGroup(models.GroupRoleAdmin)
	tok := env.mint(map[string]any{"scope": "read_write", "group_id": env.group.ID})

	rr := env.withToken(tok.Secret, http.MethodGet, env.groupPath(env.group, "/locations"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	// The user is an admin of the other group, but the token is not for it.
	for _, req := range []struct{ method, path string }{
		{http.MethodGet, env.groupPath(other, "/locations")},
		{http.MethodPost, env.groupPath(other, "/locations")},
		{http.MethodPost, env.groupPath(other, "/uploads/files")},
		{http.MethodGet, "/api/v1/groups/" + other.ID},
		{http.MethodGet, "/api/v1/groups/" + other.ID + "/members"},
	} {
		rr := env.withToken(tok.Secret, req.method, req.path, env.newLocationBody("x"))
		c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("%s %s: %s", req.method, req.path, rr.Body.String()))
		c.Assert(errorCode(c, rr), qt.Equals, "api_token.group_not_allowed")
	}

	// The group list shows the token's group only.
	rr = env.withToken(tok.Secret, http.MethodGet, "/api/v1/groups", nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	c.Assert(rr.Body.String(), qt.Contains, env.group.ID)
	c.Assert(rr.Body.String(), qt.Not(qt.Contains), other.ID)

	// An unrestricted token reaches both.
	all := env.mint(map[string]any{"scope": "read_only"})
	rr = env.withToken(all.Secret, http.MethodGet, env.groupPath(other, "/locations"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	rr = env.withToken(all.Secret, http.MethodGet, "/api/v1/groups", nil)
	c.Assert(rr.Body.String(), qt.Contains, other.ID)
}

// The token carries the owner's identity, not a frozen copy of their rights:
// the group role is read on every request.
func TestAPITokens_OwnerRoleStillApplies(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)
	tok := env.mint(map[string]any{"scope": "read_write"})

	rr := env.withToken(tok.Secret, http.MethodPost, env.groupPath(env.group, "/locations"), env.newLocationBody("Before"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated)

	env.setRole(env.group, models.GroupRoleViewer)
	rr = env.withToken(tok.Secret, http.MethodPost, env.groupPath(env.group, "/locations"), env.newLocationBody("After"))
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf(rr.Body.String()))

	// Removed from the group entirely: not even reads.
	memberships := must.Must(env.params.FactorySet.GroupMembershipRegistry.ListByUser(context.Background(), env.user.TenantID, env.user.ID))
	for _, m := range memberships {
		c.Assert(env.params.FactorySet.GroupMembershipRegistry.Delete(context.Background(), m.ID), qt.IsNil)
	}
	rr = env.withToken(tok.Secret, http.MethodGet, env.groupPath(env.group, "/locations"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf(rr.Body.String()))
}

func TestAPITokens_RejectedTokensAnswer401(t *testing.T) {
	tests := []struct {
		name  string
		token func(env *apiTokenEnv) string
	}{
		{"unknown", func(*apiTokenEnv) string {
			return must.Must(models.GenerateAPIToken()).Token
		}},
		{"malformed", func(*apiTokenEnv) string { return "inv_pat_not-a-real-token" }},
		{"revoked", func(env *apiTokenEnv) string {
			tok := env.mint(nil)
			rr := env.session(http.MethodDelete, "/api/v1/users/me/api-tokens/"+tok.Token.ID, nil)
			env.c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
			return tok.Secret
		}},
		{"expired", func(env *apiTokenEnv) string {
			return env.insert(func(tk *models.APIToken) { tk.ExpiresAt = new(time.Now().Add(-time.Second)) })
		}},
		{"owner deactivated", func(env *apiTokenEnv) string {
			tok := env.mint(nil)
			u := must.Must(env.params.FactorySet.UserRegistry.Get(context.Background(), env.user.ID))
			u.IsActive = false
			must.Must(env.params.FactorySet.UserRegistry.Update(context.Background(), *u))
			return tok.Secret
		}},
		{"owner moved to another tenant", func(env *apiTokenEnv) string {
			tok := env.mint(nil)
			u := must.Must(env.params.FactorySet.UserRegistry.Get(context.Background(), env.user.ID))
			u.TenantID = "some-other-tenant"
			must.Must(env.params.FactorySet.UserRegistry.Update(context.Background(), *u))
			return tok.Secret
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newAPITokenEnv(c)
			token := tt.token(env)

			rr := env.withToken(token, http.MethodGet, env.groupPath(env.group, "/locations"), nil)
			c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized, qt.Commentf(rr.Body.String()))
			c.Assert(errorCode(c, rr), qt.Equals, "api_token.invalid")
			// One answer for every cause: the body does not say which.
			c.Assert(rr.Body.String(), qt.Not(qt.Contains), "revoked")
			c.Assert(rr.Body.String(), qt.Not(qt.Contains), "expired")
			c.Assert(rr.Body.String(), qt.Not(qt.Contains), "inactive")
		})
	}
}

// Account, security, MFA, session, token-management and admin endpoints
// refuse a token outright, and say so the same way whether or not the token
// is valid.
func TestAPITokens_RefusedOnAccountSecurityAndAdminEndpoints(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)
	valid := env.mint(map[string]any{"scope": "read_write"}).Secret
	bogus := models.APITokenPrefix + "bogus"

	routes := []struct{ method, path string }{
		{http.MethodGet, "/api/v1/auth/me"},
		{http.MethodPut, "/api/v1/auth/me"},
		{http.MethodDelete, "/api/v1/auth/me"},
		{http.MethodPost, "/api/v1/auth/change-password"},
		{http.MethodPost, "/api/v1/auth/logout"},
		{http.MethodPost, "/api/v1/auth/refresh"},
		{http.MethodGet, "/api/v1/auth/mfa/status"},
		{http.MethodPost, "/api/v1/auth/mfa/setup"},
		{http.MethodPost, "/api/v1/auth/mfa/disable"},
		{http.MethodGet, "/api/v1/users/me/sessions"},
		{http.MethodDelete, "/api/v1/users/me/sessions"},
		{http.MethodGet, "/api/v1/users/me/login-history"},
		{http.MethodGet, "/api/v1/users/me/api-tokens"},
		{http.MethodPost, "/api/v1/users/me/api-tokens"},
		{http.MethodDelete, "/api/v1/users/me/api-tokens/some-id"},
		{http.MethodGet, "/api/v1/admin/users"},
		{http.MethodPost, "/api/v1/backoffice/auth/login"},
		{http.MethodGet, "/api/v1/system"},
		{http.MethodPost, "/api/v1/feedback"},
		{http.MethodPost, "/api/v1/invites/some-token/accept"},
	}
	for _, token := range []string{valid, bogus} {
		for _, route := range routes {
			rr := env.withToken(token, route.method, route.path, map[string]any{"name": "x", "current_password": apiTokenTestPassword})
			c.Assert(rr.Code, qt.Equals, http.StatusForbidden,
				qt.Commentf("%s %s with token %q: %s", route.method, route.path, token[:12], rr.Body.String()))
			c.Assert(errorCode(c, rr), qt.Equals, "api_token.endpoint_not_allowed",
				qt.Commentf("%s %s", route.method, route.path))
		}
	}

	// Group management accepts tokens, but only to read, whatever the scope.
	// The token is authenticated first there, so only a valid one gets this
	// far.
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/groups"},
		{http.MethodPatch, "/api/v1/groups/" + env.group.ID},
		{http.MethodDelete, "/api/v1/groups/" + env.group.ID},
		{http.MethodPost, "/api/v1/groups/" + env.group.ID + "/invites"},
		{http.MethodPost, "/api/v1/groups/" + env.group.ID + "/leave"},
	} {
		rr := env.withToken(valid, route.method, route.path, map[string]any{"name": "x"})
		c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("%s %s: %s", route.method, route.path, rr.Body.String()))
		c.Assert(errorCode(c, rr), qt.Equals, "api_token.endpoint_not_allowed")
	}

	// Nothing was minted with the token.
	list := env.sessionWithoutCSRF(http.MethodGet, "/api/v1/users/me/api-tokens", nil)
	var resp apiserver.APITokensListResponse
	c.Assert(json.Unmarshal(list.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Tokens, qt.HasLen, 1)
}

// The CSRF rule for browser sessions is unchanged: only a request the token
// branch authenticated skips it, and a cookie never makes a request a token
// request or a token request a session request.
func TestAPITokens_SessionsStillNeedCSRF(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)
	tok := env.mint(map[string]any{"scope": "read_write"})
	refreshCookie := &http.Cookie{
		Name:     appctx.RefreshTokenCookieName,
		Value:    "a-refresh-token",
		Path:     "/api/v1",
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
	path := env.groupPath(env.group, "/locations")

	c.Run("session write without CSRF", func(c *qt.C) {
		rr := env.sessionWithoutCSRF(http.MethodPost, path, env.newLocationBody("x"))
		c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
		c.Assert(rr.Body.String(), qt.Contains, "CSRF token required")
	})
	c.Run("session write with a wrong CSRF token", func(c *qt.C) {
		rr := env.do(apiTokenRequest{
			method: http.MethodPost, path: path, body: env.newLocationBody("x"),
			headers: map[string][]string{
				"Authorization": {"Bearer " + createTestJWTToken(env.user.ID)},
				"X-CSRF-Token":  {"not-the-token"},
			},
		})
		c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	})
	c.Run("session write with CSRF", func(c *qt.C) {
		rr := env.session(http.MethodPost, path, env.newLocationBody("x"))
		c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
	})
	c.Run("session write with the refresh cookie and no CSRF", func(c *qt.C) {
		rr := env.do(apiTokenRequest{
			method: http.MethodPost, path: path, body: env.newLocationBody("x"),
			headers: map[string][]string{"Authorization": {"Bearer " + createTestJWTToken(env.user.ID)}},
			cookies: []*http.Cookie{refreshCookie},
		})
		c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	})
	c.Run("refresh cookie alone authenticates nothing", func(c *qt.C) {
		rr := env.do(apiTokenRequest{
			method: http.MethodGet, path: path,
			cookies: []*http.Cookie{refreshCookie},
		})
		c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
	})
	c.Run("token with a cookie is still a token request", func(c *qt.C) {
		rr := env.do(apiTokenRequest{
			method: http.MethodPost, path: path, body: env.newLocationBody("x"),
			headers: map[string][]string{"Authorization": {"Bearer " + tok.Secret}},
			cookies: []*http.Cookie{refreshCookie},
		})
		c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf(rr.Body.String()))
	})
	c.Run("a token and a JWT together are refused", func(c *qt.C) {
		rr := env.do(apiTokenRequest{
			method: http.MethodGet, path: path,
			headers: map[string][]string{"Authorization": {
				"Bearer " + createTestJWTToken(env.user.ID),
				"Bearer " + tok.Secret,
			}},
		})
		c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
		c.Assert(errorCode(c, rr), qt.Equals, "api_token.ambiguous_credentials")
	})
}

func TestAPITokens_CannotBeManagedWhileImpersonating(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)
	existing := env.mint(nil)
	_, operatorToken := WithBackofficeAdmin(t, env.params)
	handler := env.handler

	rr := doImpersonateStart(c, handler, operatorToken, env.user.ID, map[string]any{"reason": "support ticket"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	var login apiserver.LoginResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &login), qt.IsNil)
	headers := map[string][]string{
		"Authorization": {"Bearer " + login.AccessToken},
		"X-CSRF-Token":  {login.CSRFToken},
	}

	create := env.do(apiTokenRequest{
		method: http.MethodPost, path: "/api/v1/users/me/api-tokens", headers: headers,
		body: map[string]any{"name": "backdoor", "scope": "read_write", "current_password": apiTokenTestPassword},
	})
	c.Assert(create.Code, qt.Equals, http.StatusForbidden, qt.Commentf(create.Body.String()))
	c.Assert(errorCode(c, create), qt.Equals, "api_token.impersonation_forbidden")

	revoke := env.do(apiTokenRequest{method: http.MethodDelete, path: "/api/v1/users/me/api-tokens/" + existing.Token.ID, headers: headers})
	c.Assert(revoke.Code, qt.Equals, http.StatusForbidden)
	c.Assert(errorCode(c, revoke), qt.Equals, "api_token.impersonation_forbidden")

	list := env.do(apiTokenRequest{method: http.MethodGet, path: "/api/v1/users/me/api-tokens", headers: headers})
	c.Assert(list.Code, qt.Equals, http.StatusOK)
	c.Assert(list.Body.String(), qt.Not(qt.Contains), existing.Secret)
}

func TestAPITokens_RevokedByAccountSecurityEvents(t *testing.T) {
	c := qt.New(t)

	c.Run("password change", func(c *qt.C) {
		env := newAPITokenEnv(c)
		tok := env.mint(nil)
		rr := env.session(http.MethodPost, "/api/v1/auth/change-password", apiserver.ChangePasswordRequest{
			CurrentPassword: apiTokenTestPassword,
			NewPassword:     "AnotherPassword456",
		})
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

		rr = env.withToken(tok.Secret, http.MethodGet, env.groupPath(env.group, "/locations"), nil)
		c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
		c.Assert(env.auditRows("api_token_revoke_all"), qt.HasLen, 1)
	})

	c.Run("admin block", func(c *qt.C) {
		env := newAPITokenEnv(c)
		tok := env.mint(nil)
		_, operatorToken := WithBackofficeAdmin(t, env.params)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users/"+env.user.ID+"/block",
			bytes.NewReader(must.Must(json.Marshal(blockBody("compromised account", true)))))
		req.Header.Set("Content-Type", "application/json")
		addBackofficeAuthHeader(req, operatorToken)
		rr := httptest.NewRecorder()
		env.handler.ServeHTTP(rr, req)
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

		// Unblocking does not bring the token back.
		u := must.Must(env.params.FactorySet.UserRegistry.Get(context.Background(), env.user.ID))
		u.IsActive = true
		must.Must(env.params.FactorySet.UserRegistry.Update(context.Background(), *u))
		rr = env.withToken(tok.Secret, http.MethodGet, env.groupPath(env.group, "/locations"), nil)
		c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
	})
}

// last_used_at is written once per interval, not on every request, and each
// write leaves one api_token_use audit row. Rows written during a token
// request carry the token's id.
func TestAPITokens_UseIsRecordedAndThrottled(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)
	tok := env.mint(map[string]any{"scope": "read_write"})

	for range 3 {
		rr := env.withToken(tok.Secret, http.MethodGet, env.groupPath(env.group, "/locations"), nil)
		c.Assert(rr.Code, qt.Equals, http.StatusOK)
	}

	uses := env.auditRows("api_token_use")
	c.Assert(uses, qt.HasLen, 1)
	c.Assert(*uses[0].EntityID, qt.Equals, tok.Token.ID)
	c.Assert(uses[0].APITokenID, qt.IsNotNil)
	c.Assert(*uses[0].APITokenID, qt.Equals, tok.Token.ID)

	list := env.sessionWithoutCSRF(http.MethodGet, "/api/v1/users/me/api-tokens", nil)
	var resp apiserver.APITokensListResponse
	c.Assert(json.Unmarshal(list.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Tokens[0].LastUsedAt, qt.IsNotNil)

	// The create and revoke rows of a session carry no token id.
	creates := env.auditRows("api_token_create")
	c.Assert(creates[0].APITokenID, qt.IsNil)
}

func TestAPITokens_AreRateLimited(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c, func(p *apiserver.Params) {
		p.GlobalRateLimiter = services.NewInMemoryGlobalRateLimiter(2, time.Hour)
	})
	tok := env.mint(nil)
	path := env.groupPath(env.group, "/locations")

	for range 2 {
		rr := env.withToken(tok.Secret, http.MethodGet, path, nil)
		c.Assert(rr.Code, qt.Equals, http.StatusOK)
		c.Assert(rr.Header().Get("X-RateLimit-Limit"), qt.Equals, "2")
	}
	rr := env.withToken(tok.Secret, http.MethodGet, path, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusTooManyRequests)
	c.Assert(errorCode(c, rr), qt.Equals, "api_token.rate_limited")
	c.Assert(rr.Header().Get("Retry-After"), qt.Not(qt.Equals), "")

	// Guessing tokens draws on the same per-address budget.
	rr = env.withToken(must.Must(models.GenerateAPIToken()).Token, http.MethodGet, path, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusTooManyRequests)

	// Browser sessions are not affected.
	rr = env.sessionWithoutCSRF(http.MethodGet, path, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
}
