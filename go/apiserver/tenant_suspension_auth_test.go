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
	"github.com/go-extras/go-kit/must"
	"github.com/golang-jwt/jwt/v5"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
)

// #1037: a suspended tenant's users cannot sign in, and their sessions end.
// Two ways in are covered: through the tenant's own host, where
// PublicTenantMiddleware refuses everything, and through another tenant's
// host, where the credential is checked against the user's own tenant.

type suspensionRequest struct {
	method  string
	path    string
	body    any
	bearer  string
	refresh string
}

func doSuspensionRequest(c *qt.C, handler http.Handler, sr suspensionRequest) *httptest.ResponseRecorder {
	c.Helper()
	var raw []byte
	if sr.body != nil {
		raw = must.Must(json.Marshal(sr.body))
	}
	req := httptest.NewRequest(sr.method, sr.path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if sr.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+sr.bearer)
	}
	if sr.refresh != "" {
		req.AddCookie(&http.Cookie{
			Name:     appctx.RefreshTokenCookieName,
			Value:    sr.refresh,
			Secure:   true,
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

// seedRefreshCookie stores a live refresh token for the user and returns the
// raw value the cookie carries.
func seedRefreshCookie(c *qt.C, params apiserver.Params, user *models.User) string {
	c.Helper()
	raw := "raw-refresh-" + user.ID + "-" + time.Now().Format(time.RFC3339Nano)
	must.Must(params.FactorySet.RefreshTokenRegistry.Create(context.Background(), models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: user.TenantID, UserID: user.ID},
		TokenHash:               models.HashRefreshToken(raw),
		ExpiresAt:               time.Now().Add(24 * time.Hour),
		CreatedAt:               time.Now(),
	}))
	return raw
}

// seedAPIToken stores a personal API token for the user and returns it.
func seedAPIToken(c *qt.C, params apiserver.Params, user *models.User) string {
	c.Helper()
	gen := must.Must(models.GenerateAPIToken())
	must.Must(params.FactorySet.APITokenRegistry.Create(context.Background(), models.APIToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: user.TenantID, UserID: user.ID},
		Name:                    "script",
		TokenHash:               gen.Hash,
		TokenPrefix:             gen.DisplayPrefix,
		Scope:                   models.APITokenScopeReadOnly,
	}))
	return gen.Token
}

// mfaChallengeFor forges the step-1 token POST /auth/login issues to a user
// who has MFA on.
func mfaChallengeFor(c *qt.C, user *models.User) string {
	c.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":          "mfa-" + user.ID,
		"user_id":      user.ID,
		"tenant_id":    user.TenantID,
		"login_method": string(models.LoginMethodPassword),
		"token_type":   "mfa_challenge",
		"iat":          time.Now().Unix(),
		"exp":          time.Now().Add(5 * time.Minute).Unix(),
	})
	return must.Must(token.SignedString(testJWTSecret))
}

// On the suspended tenant's own host every way in is refused before it
// reaches a handler.
func TestSuspendedTenant_RefusedOnItsOwnHost(t *testing.T) {
	c := qt.New(t)
	params, user, group := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	accessToken := createTestJWTToken(user.ID)
	refresh := seedRefreshCookie(c, params, user)
	apiToken := seedAPIToken(c, params, user)

	tenant := must.Must(params.FactorySet.TenantRegistry.Get(context.Background(), user.TenantID))
	tenant.Status = models.TenantStatusSuspended
	must.Must(params.FactorySet.TenantRegistry.Update(context.Background(), *tenant))

	paths := []struct {
		name string
		req  suspensionRequest
	}{
		{"password login", suspensionRequest{method: http.MethodPost, path: "/api/v1/auth/login", body: map[string]string{"email": user.Email, "password": "Password123"}}},
		{"MFA login", suspensionRequest{method: http.MethodPost, path: "/api/v1/auth/login/mfa", body: map[string]string{"mfa_token": mfaChallengeFor(c, user), "totp_code": "000000"}}},
		{"refresh", suspensionRequest{method: http.MethodPost, path: "/api/v1/auth/refresh", body: map[string]string{}, refresh: refresh}},
		{"magic link request", suspensionRequest{method: http.MethodPost, path: "/api/v1/auth/magic-link/request", body: map[string]string{"email": user.Email}}},
		{"magic link verify", suspensionRequest{method: http.MethodPost, path: "/api/v1/auth/magic-link/verify", body: map[string]string{"token": "anything"}}},
		{"OAuth start", suspensionRequest{method: http.MethodGet, path: "/api/v1/auth/oauth/google/start"}},
		{"OAuth callback", suspensionRequest{method: http.MethodGet, path: "/api/v1/auth/oauth/google/callback?state=x&code=y"}},
		{"access token", suspensionRequest{method: http.MethodGet, path: "/api/v1/auth/me", bearer: accessToken}},
		{"API token", suspensionRequest{method: http.MethodGet, path: "/api/v1/g/" + group.Slug + "/locations", bearer: apiToken}},
	}
	for _, p := range paths {
		c.Run(p.name, func(c *qt.C) {
			rr := doSuspensionRequest(c, handler, p.req)
			c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf(rr.Body.String()))
			c.Assert(rr.Body.String(), qt.Contains, "Tenant suspended")
		})
	}
}

// A credential of a suspended tenant's user is refused on another tenant's
// host too, where the host check passes.
func TestSuspendedTenant_RefusedOnAnotherTenantsHost(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)

	// Before the suspension the member's session works on the seeded
	// tenant's host, which is the only host these requests use.
	rr := doSuspensionRequest(c, env.handler, suspensionRequest{method: http.MethodGet, path: "/api/v1/auth/me", bearer: createTestJWTToken(env.member.ID)})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	rr = env.do(c, http.MethodPost, tenantsPath("/"+env.acme.ID+"/suspend"), map[string]any{"reason": "unpaid invoice"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	// Credentials minted after the suspension, so neither the revocation
	// nor the blacklist stamp catches them: only the tenant check is left.
	time.Sleep(1100 * time.Millisecond)
	accessToken := mintAccessTokenForUser(t, env.member.ID, 0)
	refresh := seedRefreshCookie(c, env.params, env.member)
	apiToken := seedAPIToken(c, env.params, env.member)

	paths := []struct {
		name     string
		req      suspensionRequest
		wantCode int
		wantErr  string
	}{
		{"access token on an /auth route", suspensionRequest{method: http.MethodGet, path: "/api/v1/auth/me", bearer: accessToken}, http.StatusForbidden, ""},
		{"access token on a data route", suspensionRequest{method: http.MethodGet, path: "/api/v1/users/me/sessions", bearer: accessToken}, http.StatusForbidden, ""},
		{"refresh", suspensionRequest{method: http.MethodPost, path: "/api/v1/auth/refresh", body: map[string]string{}, refresh: refresh}, http.StatusForbidden, ""},
		{"MFA login", suspensionRequest{method: http.MethodPost, path: "/api/v1/auth/login/mfa", body: map[string]string{"mfa_token": mfaChallengeFor(c, env.member), "totp_code": "000000"}}, http.StatusForbidden, ""},
		// An API token only authenticates on its own tenant's host.
		{"API token", suspensionRequest{method: http.MethodGet, path: "/api/v1/g/any/locations", bearer: apiToken}, http.StatusUnauthorized, "api_token.invalid"},
	}
	for _, p := range paths {
		c.Run(p.name, func(c *qt.C) {
			rr := doSuspensionRequest(c, env.handler, p.req)
			c.Assert(rr.Code, qt.Equals, p.wantCode, qt.Commentf(rr.Body.String()))
			if p.wantErr != "" {
				c.Assert(errorCode(c, rr), qt.Equals, p.wantErr)
			}
		})
	}

	// The refused refresh also clears the cookie.
	rr = doSuspensionRequest(c, env.handler, suspensionRequest{method: http.MethodPost, path: "/api/v1/auth/refresh", body: map[string]string{}, refresh: seedRefreshCookie(c, env.params, env.member)})
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	c.Assert(rr.Header().Get("Set-Cookie"), qt.Contains, appctx.RefreshTokenCookieName+"=;")

	// Once the tenant is active again, a new session works.
	rr = env.do(c, http.MethodPost, tenantsPath("/"+env.acme.ID+"/reactivate"), map[string]any{"reason": "paid"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	rr = doSuspensionRequest(c, env.handler, suspensionRequest{method: http.MethodGet, path: "/api/v1/auth/me", bearer: accessToken})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
}

// Two more ways in on another tenant's host: a magic link issued on that
// host before its user moved to the suspended tenant, and the OAuth
// link-start navigation, which resolves the caller itself and redirects to
// the sign-in page instead of answering 403.
func TestSuspendedTenant_MagicLinkAndOAuthLinkStartOnAnotherTenantsHost(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	env := newTenantWriteEnv(c)
	params := env.params
	params.MagicLinkLoginEnabled = true
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	host := must.Must(params.FactorySet.TenantRegistry.GetBySlug(ctx, "test-org"))

	rr := env.do(c, http.MethodPost, tenantsPath("/"+env.acme.ID+"/suspend"), map[string]any{"reason": "unpaid invoice"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	time.Sleep(1100 * time.Millisecond)

	token := must.Must(models.GenerateMagicLinkToken())
	must.Must(params.FactorySet.MagicLinkTokenRegistry.Create(ctx, models.MagicLinkToken{
		UserID:    env.member.ID,
		TenantID:  host.ID,
		Email:     env.member.Email,
		Token:     token,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}))
	verify := suspensionRequest{method: http.MethodPost, path: "/api/v1/auth/magic-link/verify", body: map[string]string{"token": token}}
	linkStart := suspensionRequest{method: http.MethodGet, path: "/api/v1/auth/oauth/google/link/start", bearer: mintAccessTokenForUser(t, env.member.ID, 0)}

	rr = doSuspensionRequest(c, handler, verify)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf(rr.Body.String()))
	c.Assert(rr.Body.String(), qt.Contains, "Tenant suspended")

	rr = doSuspensionRequest(c, handler, linkStart)
	c.Assert(rr.Code, qt.Equals, http.StatusFound, qt.Commentf(rr.Body.String()))
	c.Assert(rr.Header().Get("Location"), qt.Contains, "reason=session_expired")

	// Once the tenant is active again both get past the tenant check: the
	// link still signs in, because the refusal did not use it up, and
	// link-start reaches the provider lookup, which knows no provider here.
	rr = env.do(c, http.MethodPost, tenantsPath("/"+env.acme.ID+"/reactivate"), map[string]any{"reason": "paid"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	rr = doSuspensionRequest(c, handler, verify)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	rr = doSuspensionRequest(c, handler, linkStart)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound, qt.Commentf(rr.Body.String()))
}

// The access token a user held when the tenant was suspended is refused at
// once, through the blacklist stamp the suspension writes, and stays refused
// after the tenant is reactivated, as after a user is unblocked.
func TestSuspendedTenant_EndsTheLiveAccessToken(t *testing.T) {
	c := qt.New(t)
	env := newTenantWriteEnv(c)
	live := mintAccessTokenForUser(t, env.member.ID, -2*time.Second)

	rr := doSuspensionRequest(c, env.handler, suspensionRequest{method: http.MethodGet, path: "/api/v1/auth/me", bearer: live})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	rr = env.do(c, http.MethodPost, tenantsPath("/"+env.acme.ID+"/suspend"), map[string]any{"reason": "unpaid invoice"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	rr = doSuspensionRequest(c, env.handler, suspensionRequest{method: http.MethodGet, path: "/api/v1/auth/me", bearer: live})
	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized, qt.Commentf(rr.Body.String()))

	rr = env.do(c, http.MethodPost, tenantsPath("/"+env.acme.ID+"/reactivate"), map[string]any{"reason": "paid"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))
	rr = doSuspensionRequest(c, env.handler, suspensionRequest{method: http.MethodGet, path: "/api/v1/auth/me", bearer: live})
	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized, qt.Commentf(rr.Body.String()))
}
