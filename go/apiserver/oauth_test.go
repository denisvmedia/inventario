package apiserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	memreg "github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
	oauthsvc "github.com/denisvmedia/inventario/services/oauth"
)

// stubProvider is a deterministic oauthsvc.Provider used by every OAuth
// handler test. Exchange returns a fixed Profile (settable per-test via
// the embedded `profile` field) and a sticky error so individual cases
// can exercise the exchange-failure branch without standing up a real
// Google/GitHub stub server.
//
// Why a struct stub and not a server: the handler tests are about
// branch coverage in apiserver/oauth.go; the wire-level provider
// behaviour is exercised by services/oauth/{google,github}_test.go. A
// struct stub keeps every test single-process and removes a flaky
// network round-trip.
type stubProvider struct {
	name     models.OAuthProvider
	profile  oauthsvc.Profile
	exchErr  error
	authBase string
}

func (s *stubProvider) Name() models.OAuthProvider { return s.name }

func (s *stubProvider) AuthCodeURL(state, codeChallenge string) string {
	base := s.authBase
	if base == "" {
		base = "https://example-provider.test/authorize"
	}
	v := url.Values{}
	v.Set("state", state)
	v.Set("code_challenge", codeChallenge)
	v.Set("code_challenge_method", "S256")
	return base + "?" + v.Encode()
}

func (s *stubProvider) Exchange(_ context.Context, _, _ string) (oauthsvc.Profile, error) {
	if s.exchErr != nil {
		return oauthsvc.Profile{}, s.exchErr
	}
	return s.profile, nil
}

// stubAuditLogger is the spy that captures AuthEvents the handler sent
// via api.auth.logAuth. SEC-3 + SEC-1 tests assert that link/unlink and
// MFA-refused branches write an audit row with the right Action.
type stubAuditLogger struct {
	mu     sync.Mutex
	events []services.AuthEvent
	admin  []services.AdminEvent
}

func (s *stubAuditLogger) LogAuth(_ context.Context, ev services.AuthEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, ev)
}

func (s *stubAuditLogger) LogAdmin(_ context.Context, ev services.AdminEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.admin = append(s.admin, ev)
}

func (s *stubAuditLogger) snapshotEvents() []services.AuthEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]services.AuthEvent, len(s.events))
	copy(out, s.events)
	return out
}

func (s *stubAuditLogger) actions() []string {
	out := []string{}
	for _, ev := range s.snapshotEvents() {
		out = append(out, ev.Action)
	}
	return out
}

// oauthFixture assembles the router + registries the OAuth handlers
// need. Tests reach into the fixture to (a) seed a user / identity row,
// (b) sign a state token + return its raw value for the request cookie,
// (c) inspect the audit + login_events spies after the call.
type oauthFixture struct {
	t           *testing.T
	jwtSecret   []byte
	stateSigner *oauthsvc.StateSigner
	providers   *oauthsvc.Registry
	stub        *stubProvider

	userRegistry       *memreg.UserRegistry
	identityRegistry   *memreg.OAuthIdentityRegistry
	loginEventRegistry *memreg.LoginEventRegistry
	mfaRegistry        *memreg.UserMFASecretRegistry
	refreshTokenReg    *memreg.RefreshTokenRegistry
	auditLogger        *stubAuditLogger
	tenant             *models.Tenant
	router             chi.Router
}

// oauthFixtureOpts customizes the fixture. Zero values are the
// "happy path" defaults (sign-in flow against a Google stub returning a
// verified email).
type oauthFixtureOpts struct {
	// stubProfile overrides the default Google profile returned by
	// Exchange. Leave zero for the default verified profile.
	stubProfile *oauthsvc.Profile
	// stubExchErr forces Exchange to fail.
	stubExchErr error
}

const oauthTestTenantID = "test-tenant-id"

func newOAuthFixture(t *testing.T, opts oauthFixtureOpts) *oauthFixture {
	t.Helper()

	jwtSecret := []byte("test-secret-32-bytes-minimum-length")

	// Stateful state signer — the production HMAC key is 32+ bytes; the
	// tests reuse the JWT secret since both inputs are opaque random
	// bytes from the bootstrap's perspective.
	signer, err := oauthsvc.NewStateSigner(jwtSecret)
	if err != nil {
		t.Fatalf("new state signer: %v", err)
	}

	stub := &stubProvider{
		name: models.OAuthProviderGoogle,
		profile: oauthsvc.Profile{
			ProviderUserID: "google-sub-1",
			Email:          "alice@example.com",
			EmailVerified:  true,
			DisplayName:    "Alice",
		},
	}
	if opts.stubProfile != nil {
		stub.profile = *opts.stubProfile
	}
	if opts.stubExchErr != nil {
		stub.exchErr = opts.stubExchErr
	}

	reg := oauthsvc.NewRegistry()
	if err := reg.Register(stub); err != nil {
		t.Fatalf("register stub provider: %v", err)
	}

	userReg := memreg.NewUserRegistry()
	identityReg := memreg.NewOAuthIdentityRegistry()
	loginEventReg := memreg.NewLoginEventRegistry()
	mfaReg := memreg.NewUserMFASecretRegistry()
	refreshReg := memreg.NewRefreshTokenRegistry()

	audit := &stubAuditLogger{}

	authHandler := apiserver.Auth(apiserver.AuthParams{
		UserRegistry:          userReg,
		RefreshTokenRegistry:  refreshReg,
		LoginEventRegistry:    loginEventReg,
		MFARegistry:           mfaReg,
		AuditService:          audit,
		JWTSecret:             jwtSecret,
		OAuthRegistry:         reg,
		OAuthStateSigner:      signer,
		OAuthIdentityRegistry: identityReg,
	})

	tenant := &models.Tenant{
		EntityID: models.EntityID{ID: oauthTestTenantID},
		Status:   models.TenantStatusActive,
	}

	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := apiserver.WithTenant(r.Context(), tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Route("/auth", authHandler)

	return &oauthFixture{
		t:                  t,
		jwtSecret:          jwtSecret,
		stateSigner:        signer,
		providers:          reg,
		stub:               stub,
		userRegistry:       userReg,
		identityRegistry:   identityReg,
		loginEventRegistry: loginEventReg,
		mfaRegistry:        mfaReg,
		refreshTokenReg:    refreshReg,
		auditLogger:        audit,
		tenant:             tenant,
		router:             router,
	}
}

// seedUser stores a user with the given email + IsActive into the
// fixture's UserRegistry and returns the persisted row (with the
// registry-assigned ID).
func (f *oauthFixture) seedUser(email string, isActive bool) *models.User {
	f.t.Helper()
	u := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: oauthTestTenantID,
		},
		Email:    strings.ToLower(strings.TrimSpace(email)),
		Name:     "Test User",
		IsActive: isActive,
	}
	if err := u.SetPassword("Password123"); err != nil {
		f.t.Fatalf("seed user: set password: %v", err)
	}
	created, err := f.userRegistry.Create(context.Background(), u)
	if err != nil {
		f.t.Fatalf("seed user: create: %v", err)
	}
	return created
}

// seedOAuthOnlyUser is the OAuth-only sign-up flavour: a user with no
// password hash (PasswordHash == ""). The default seedUser sets a
// password; the unlink-last-method test needs the empty-hash case.
func (f *oauthFixture) seedOAuthOnlyUser(email string, isActive bool) *models.User {
	f.t.Helper()
	u := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: oauthTestTenantID,
		},
		Email:    strings.ToLower(strings.TrimSpace(email)),
		Name:     "OAuth Only User",
		IsActive: isActive,
	}
	created, err := f.userRegistry.Create(context.Background(), u)
	if err != nil {
		f.t.Fatalf("seed oauth-only user: %v", err)
	}
	return created
}

// seedIdentity persists an OAuthIdentity row for user / provider /
// providerUserID.
func (f *oauthFixture) seedIdentity(user *models.User, provider models.OAuthProvider, providerUserID, email string) *models.OAuthIdentity {
	f.t.Helper()
	identity := models.OAuthIdentity{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		UserID:              user.ID,
		Provider:            provider,
		ProviderUserID:      providerUserID,
		Email:               email,
	}
	created, err := f.identityRegistry.Create(context.Background(), identity)
	if err != nil {
		f.t.Fatalf("seed identity: %v", err)
	}
	return created
}

// enableMFA writes a fully-enrolled UserMFASecret row so api.auth.userMFAEnabled
// returns true for user.
func (f *oauthFixture) enableMFA(user *models.User) {
	f.t.Helper()
	now := time.Now()
	row := models.UserMFASecret{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		SecretEncrypted: "ignored-by-stub",
		EnabledAt:       &now,
	}
	if _, err := f.mfaRegistry.Create(context.Background(), row); err != nil {
		f.t.Fatalf("enable mfa: %v", err)
	}
}

// signState mints a valid OAuth state token for provider, returning the
// raw signed value (used both as the URL `state` query param and as the
// cookie value).
func (f *oauthFixture) signState(provider models.OAuthProvider, linkUserID string) string {
	f.t.Helper()
	nonce, err := oauthsvc.NewNonce()
	if err != nil {
		f.t.Fatalf("nonce: %v", err)
	}
	pkce, err := oauthsvc.NewPKCE()
	if err != nil {
		f.t.Fatalf("pkce: %v", err)
	}
	st := oauthsvc.State{
		Provider:   string(provider),
		Nonce:      nonce,
		Verifier:   pkce.Verifier,
		LinkUserID: linkUserID,
	}
	signed, err := f.stateSigner.Sign(st)
	if err != nil {
		f.t.Fatalf("sign state: %v", err)
	}
	return signed
}

// bearerToken mints an access JWT for the given user — matches the
// access token the production RequireAuth middleware accepts.
func (f *oauthFixture) bearerToken(user *models.User) string {
	f.t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":        uuid.New().String(),
		"user_id":    user.ID,
		"token_type": "access",
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(5 * time.Minute).Unix(),
	})
	signed, err := token.SignedString(f.jwtSecret)
	if err != nil {
		f.t.Fatalf("sign bearer: %v", err)
	}
	return signed
}

// callCallback issues a GET against /auth/oauth/{provider}/callback with
// the supplied state, code, and optional state cookie value. When
// cookieValue is empty, no cookie is attached (the "cookie missing"
// case). Returns the recorder.
func (f *oauthFixture) callCallback(provider models.OAuthProvider, state, code, cookieValue string) *httptest.ResponseRecorder {
	f.t.Helper()
	u := "/auth/oauth/" + string(provider) + "/callback?state=" + url.QueryEscape(state) + "&code=" + url.QueryEscape(code)
	req := httptest.NewRequest(http.MethodGet, u, nil)
	if cookieValue != "" {
		// #nosec G124 -- test-only cookie added to a httptest.NewRequest; transport security is irrelevant here.
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: cookieValue})
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

// callLinkStart issues GET /auth/oauth/{provider}/link/start with the
// caller's access token (or without, to exercise the 401 branch).
func (f *oauthFixture) callLinkStart(provider models.OAuthProvider, bearer string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/auth/oauth/"+string(provider)+"/link/start", nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

// callUnlink issues DELETE /auth/oauth/{provider} as the supplied user.
func (f *oauthFixture) callUnlink(provider models.OAuthProvider, bearer string) *httptest.ResponseRecorder {
	f.t.Helper()
	req := httptest.NewRequest(http.MethodDelete, "/auth/oauth/"+string(provider), nil)
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

// loginEvents lists every login_event row written during the test.
func (f *oauthFixture) loginEvents() []*models.LoginEvent {
	rows, err := f.loginEventRegistry.List(context.Background())
	if err != nil {
		f.t.Fatalf("list login events: %v", err)
	}
	return rows
}

// listIdentities returns every identity row currently linked to user.
func (f *oauthFixture) listIdentities(user *models.User) []*models.OAuthIdentity {
	f.t.Helper()
	rows, err := f.identityRegistry.ListByUser(context.Background(), user.TenantID, user.ID)
	if err != nil {
		f.t.Fatalf("list identities: %v", err)
	}
	return rows
}

// =============================================================================
// Sign-in branch coverage
// =============================================================================

func TestOAuthCallback_ExistingIdentity_SignsIn(t *testing.T) {
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	user := f.seedUser("alice@example.com", true)
	f.seedIdentity(user, models.OAuthProviderGoogle, "google-sub-1", "alice@example.com")

	state := f.signState(models.OAuthProviderGoogle, "")
	rec := f.callCallback(models.OAuthProviderGoogle, state, "code-1", state)

	// Existing identity: handler 302s the user back to the FE app root and
	// mints a refresh cookie. No tokens are written into the body — the
	// FE picks them up on the next /auth/me round-trip.
	c.Assert(rec.Code, qt.Equals, http.StatusFound)
	c.Assert(rec.Header().Get("Location"), qt.Equals, "/")

	cookies := rec.Result().Cookies()
	hasRefresh := false
	for _, cookie := range cookies {
		if cookie.Name == "refresh_token" && cookie.Value != "" {
			hasRefresh = true
		}
	}
	c.Assert(hasRefresh, qt.IsTrue)

	// login_events: one row, outcome=ok, method=oauth_google.
	events := f.loginEvents()
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Outcome, qt.Equals, models.LoginOutcomeOK)
	c.Assert(events[0].Method, qt.Equals, models.LoginMethodOAuthGoogle)
}

func TestOAuthCallback_EmailVerifiedMatch_AutoLinks(t *testing.T) {
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	user := f.seedUser("alice@example.com", true)

	state := f.signState(models.OAuthProviderGoogle, "")
	rec := f.callCallback(models.OAuthProviderGoogle, state, "code-2", state)

	c.Assert(rec.Code, qt.Equals, http.StatusFound)
	c.Assert(rec.Header().Get("Location"), qt.Equals, "/")

	// Identity row was auto-linked to the existing user.
	rows := f.listIdentities(user)
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Provider, qt.Equals, models.OAuthProviderGoogle)
	c.Assert(rows[0].ProviderUserID, qt.Equals, "google-sub-1")
}

func TestOAuthCallback_EmailUnverified_RedirectsToLinkRequired(t *testing.T) {
	c := qt.New(t)
	profile := oauthsvc.Profile{
		ProviderUserID: "google-sub-unverified",
		Email:          "bob@example.com",
		EmailVerified:  false,
		DisplayName:    "Bob",
	}
	f := newOAuthFixture(t, oauthFixtureOpts{stubProfile: &profile})

	user := f.seedUser("bob@example.com", true)

	state := f.signState(models.OAuthProviderGoogle, "")
	rec := f.callCallback(models.OAuthProviderGoogle, state, "code-3", state)

	c.Assert(rec.Code, qt.Equals, http.StatusFound)
	loc, err := url.Parse(rec.Header().Get("Location"))
	c.Assert(err, qt.IsNil)
	c.Assert(loc.Path, qt.Equals, "/login")
	c.Assert(loc.Query().Get("oauth_link_required"), qt.Equals, "1")
	c.Assert(loc.Query().Get("email"), qt.Equals, "bob@example.com")
	c.Assert(loc.Query().Get("provider"), qt.Equals, "google") // REV-3

	// No identity row created.
	rows := f.listIdentities(user)
	c.Assert(rows, qt.HasLen, 0)

	// login_events captured the email_not_verified branch.
	events := f.loginEvents()
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Outcome, qt.Equals, models.LoginOutcomeEmailNotVerified)
	c.Assert(events[0].Method, qt.Equals, models.LoginMethodOAuthGoogle)
}

func TestOAuthCallback_DeactivatedUserEmailMatch_Refused(t *testing.T) {
	// SEC-2: a local user matching the verified provider email but
	// IsActive=false must not be auto-linked. Handler 302s to /login with
	// oauth_error=account_disabled and records account_disabled.
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	user := f.seedUser("disabled@example.com", false)
	// Override stub profile to match the disabled user's email.
	f.stub.profile = oauthsvc.Profile{
		ProviderUserID: "google-sub-disabled",
		Email:          "disabled@example.com",
		EmailVerified:  true,
		DisplayName:    "Disabled",
	}

	state := f.signState(models.OAuthProviderGoogle, "")
	rec := f.callCallback(models.OAuthProviderGoogle, state, "code-disabled", state)

	c.Assert(rec.Code, qt.Equals, http.StatusFound)
	loc, err := url.Parse(rec.Header().Get("Location"))
	c.Assert(err, qt.IsNil)
	c.Assert(loc.Path, qt.Equals, "/login")
	c.Assert(loc.Query().Get("oauth_error"), qt.Equals, "account_disabled")

	// No identity row was created.
	rows := f.listIdentities(user)
	c.Assert(rows, qt.HasLen, 0)

	// login_events captured the disabled branch.
	events := f.loginEvents()
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Outcome, qt.Equals, models.LoginOutcomeAccountDisabled)
}

func TestOAuthCallback_MFAEnrolled_RefusesOAuthOnlySignIn(t *testing.T) {
	// SEC-1: a user with TOTP enrolled cannot be signed in via OAuth-only
	// — the callback 302s to /login?mfa_required=1 and never mints a
	// refresh cookie.
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	user := f.seedUser("alice@example.com", true)
	f.seedIdentity(user, models.OAuthProviderGoogle, "google-sub-1", "alice@example.com")
	f.enableMFA(user)

	state := f.signState(models.OAuthProviderGoogle, "")
	rec := f.callCallback(models.OAuthProviderGoogle, state, "code-mfa", state)

	c.Assert(rec.Code, qt.Equals, http.StatusFound)
	loc, err := url.Parse(rec.Header().Get("Location"))
	c.Assert(err, qt.IsNil)
	c.Assert(loc.Path, qt.Equals, "/login")
	c.Assert(loc.Query().Get("mfa_required"), qt.Equals, "1")
	c.Assert(loc.Query().Get("oauth_provider"), qt.Equals, "google")
	// Email is masked: keep the first char, then "...", then the domain.
	c.Assert(loc.Query().Get("email"), qt.Equals, "a...@example.com")

	// No refresh cookie issued — token-issue path was skipped.
	for _, cookie := range rec.Result().Cookies() {
		c.Assert(cookie.Name, qt.Not(qt.Equals), "refresh_token")
	}

	// login_events row with outcome=mfa_required + method=oauth_google.
	events := f.loginEvents()
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Outcome, qt.Equals, models.LoginOutcomeMFARequired)
	c.Assert(events[0].Method, qt.Equals, models.LoginMethodOAuthGoogle)

	// Audit row written (the "oauth_login_mfa_required" verb).
	actions := f.auditLogger.actions()
	c.Assert(actions, qt.Contains, "oauth_login_mfa_required")
}

func TestOAuthCallback_NoMatch_ProvisionsNewUser(t *testing.T) {
	c := qt.New(t)
	profile := oauthsvc.Profile{
		ProviderUserID: "google-sub-new",
		Email:          "newuser@example.com",
		EmailVerified:  true,
		DisplayName:    "New User",
	}
	f := newOAuthFixture(t, oauthFixtureOpts{stubProfile: &profile})

	state := f.signState(models.OAuthProviderGoogle, "")
	rec := f.callCallback(models.OAuthProviderGoogle, state, "code-new", state)

	c.Assert(rec.Code, qt.Equals, http.StatusFound)
	c.Assert(rec.Header().Get("Location"), qt.Equals, "/")

	// User provisioned: no password hash, active, identity linked.
	created, err := f.userRegistry.GetByEmail(context.Background(), oauthTestTenantID, "newuser@example.com")
	c.Assert(err, qt.IsNil)
	c.Assert(created.PasswordHash, qt.Equals, "")
	c.Assert(created.IsActive, qt.IsTrue)
	c.Assert(created.Email, qt.Equals, "newuser@example.com")

	rows := f.listIdentities(created)
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Provider, qt.Equals, models.OAuthProviderGoogle)
}

func TestOAuthCallback_MixedCaseEmail_MatchesExistingUser(t *testing.T) {
	// REV-1: the provider returns "User@Example.com"; the local user is
	// stored as "user@example.com" (lowercased on insert). Without REV-1
	// the email-match branch would never fire and the callback would
	// provision a duplicate row.
	c := qt.New(t)
	profile := oauthsvc.Profile{
		ProviderUserID: "google-sub-mixed",
		Email:          "User@Example.com",
		EmailVerified:  true,
		DisplayName:    "Mixed Case",
	}
	f := newOAuthFixture(t, oauthFixtureOpts{stubProfile: &profile})

	user := f.seedUser("user@example.com", true)

	state := f.signState(models.OAuthProviderGoogle, "")
	rec := f.callCallback(models.OAuthProviderGoogle, state, "code-mixed", state)

	c.Assert(rec.Code, qt.Equals, http.StatusFound)

	// Exactly one user; identity attached to the original lowercase row.
	all, err := f.userRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 1)

	rows := f.listIdentities(user)
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].UserID, qt.Equals, user.ID)
	// The persisted identity email is the lowercased form too.
	c.Assert(rows[0].Email, qt.Equals, "user@example.com")
}

// =============================================================================
// State / cookie checks
// =============================================================================

func TestOAuthCallback_CookieMissing_Rejects(t *testing.T) {
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	state := f.signState(models.OAuthProviderGoogle, "")
	rec := f.callCallback(models.OAuthProviderGoogle, state, "code-x", "")

	c.Assert(rec.Code, qt.Equals, http.StatusBadRequest)
	c.Assert(rec.Body.String(), qt.Contains, "cookie")
}

func TestOAuthCallback_CookieMismatch_Rejects(t *testing.T) {
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	state := f.signState(models.OAuthProviderGoogle, "")
	rec := f.callCallback(models.OAuthProviderGoogle, state, "code-y", "different-cookie-value")

	c.Assert(rec.Code, qt.Equals, http.StatusBadRequest)
}

func TestOAuthCallback_ProviderMismatchOnState_Rejects(t *testing.T) {
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	// State minted for google; callback URL is /github/callback. The
	// state.Provider != URL provider branch fires. We also need a github
	// provider in the registry so the URL doesn't 404 before the state
	// check runs.
	githubStub := &stubProvider{name: models.OAuthProviderGitHub}
	c.Assert(f.providers.Register(githubStub), qt.IsNil, qt.Commentf("register github stub"))

	state := f.signState(models.OAuthProviderGoogle, "")
	rec := f.callCallback(models.OAuthProviderGitHub, state, "code-z", state)

	c.Assert(rec.Code, qt.Equals, http.StatusBadRequest)
	c.Assert(rec.Body.String(), qt.Contains, "provider mismatch")
}

// =============================================================================
// Link flow
// =============================================================================

func TestOAuthLinkStart_NoAuth_Rejects(t *testing.T) {
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	rec := f.callLinkStart(models.OAuthProviderGoogle, "")
	c.Assert(rec.Code, qt.Equals, http.StatusUnauthorized)
}

func TestOAuthLinkCallback_ProviderAccountOnDifferentUser_Conflicts(t *testing.T) {
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	alice := f.seedUser("alice@example.com", true)
	bob := f.seedUser("bob@example.com", true)
	// Bob already linked google-sub-1.
	f.seedIdentity(bob, models.OAuthProviderGoogle, "google-sub-1", "bob@example.com")
	// Alice tries to link the same google account.
	state := f.signState(models.OAuthProviderGoogle, alice.ID)
	rec := f.callCallback(models.OAuthProviderGoogle, state, "code-link-conflict", state)

	c.Assert(rec.Code, qt.Equals, http.StatusConflict)

	// Bob still owns the identity row.
	bobRows := f.listIdentities(bob)
	c.Assert(bobRows, qt.HasLen, 1)
	aliceRows := f.listIdentities(alice)
	c.Assert(aliceRows, qt.HasLen, 0)
}

func TestOAuthLinkCallback_Success_WritesAuditAndLoginEvent(t *testing.T) {
	// SEC-3 + REV-8: the link success path emits an audit row
	// (oauth_link_added) and a login_event with outcome=identity_linked.
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	user := f.seedUser("link@example.com", true)
	state := f.signState(models.OAuthProviderGoogle, user.ID)
	rec := f.callCallback(models.OAuthProviderGoogle, state, "code-link-ok", state)

	c.Assert(rec.Code, qt.Equals, http.StatusFound)

	rows := f.listIdentities(user)
	c.Assert(rows, qt.HasLen, 1)

	// REV-8: outcome=identity_linked.
	events := f.loginEvents()
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Outcome, qt.Equals, models.LoginOutcomeIdentityLinked)
	c.Assert(events[0].Method, qt.Equals, models.LoginMethodOAuthGoogle)

	// SEC-3: audit row written with action=oauth_link_added.
	c.Assert(f.auditLogger.actions(), qt.Contains, "oauth_link_added")
}

// =============================================================================
// Unlink flow
// =============================================================================

func TestOAuthUnlink_LastMethod_Conflicts(t *testing.T) {
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	// OAuth-only user with a single linked provider — unlinking it would
	// lock them out.
	user := f.seedOAuthOnlyUser("oo@example.com", true)
	f.seedIdentity(user, models.OAuthProviderGoogle, "google-sub-oo", "oo@example.com")

	bearer := f.bearerToken(user)
	rec := f.callUnlink(models.OAuthProviderGoogle, bearer)

	c.Assert(rec.Code, qt.Equals, http.StatusConflict)

	rows := f.listIdentities(user)
	c.Assert(rows, qt.HasLen, 1)
}

func TestOAuthUnlink_WithPassword_Succeeds(t *testing.T) {
	// SEC-3 also fires here — every unlink writes oauth_link_removed.
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	user := f.seedUser("withpw@example.com", true) // seedUser sets a password
	f.seedIdentity(user, models.OAuthProviderGoogle, "google-sub-pw", "withpw@example.com")

	bearer := f.bearerToken(user)
	rec := f.callUnlink(models.OAuthProviderGoogle, bearer)

	c.Assert(rec.Code, qt.Equals, http.StatusNoContent)

	rows := f.listIdentities(user)
	c.Assert(rows, qt.HasLen, 0)

	c.Assert(f.auditLogger.actions(), qt.Contains, "oauth_link_removed")
}

func TestOAuthUnlink_TwoIdentities_AllowedWithoutPassword(t *testing.T) {
	// Two linked providers + no password → unlinking one is safe because
	// the second one is still a working sign-in method.
	c := qt.New(t)
	f := newOAuthFixture(t, oauthFixtureOpts{})

	user := f.seedOAuthOnlyUser("two@example.com", true)
	f.seedIdentity(user, models.OAuthProviderGoogle, "google-sub-two", "two@example.com")
	f.seedIdentity(user, models.OAuthProviderGitHub, "github-id-two", "two@example.com")

	bearer := f.bearerToken(user)
	rec := f.callUnlink(models.OAuthProviderGoogle, bearer)

	c.Assert(rec.Code, qt.Equals, http.StatusNoContent)

	rows := f.listIdentities(user)
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].Provider, qt.Equals, models.OAuthProviderGitHub)

	c.Assert(f.auditLogger.actions(), qt.Contains, "oauth_link_removed")
}

// (maskEmail is covered indirectly via TestOAuthCallback_MFAEnrolled_RefusesOAuthOnlySignIn,
// which asserts the masked email lands in the redirect URL.)
