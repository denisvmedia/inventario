package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	memreg "github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// recordingMagicLinkEmailService records every SendMagicLinkEmail call so the
// request-handler tests can assert exactly-one (or zero) sends. Every other
// EmailService method is a no-op — the magic-link flow only sends this one
// email. A mutex guards the counter because SendMagicLinkEmail fires from the
// handler's detached goroutine.
type recordingMagicLinkEmailService struct {
	mu        sync.Mutex
	calls     int
	lastTo    string
	lastURL   string
	lastName  string
	returnErr error
}

func (m *recordingMagicLinkEmailService) SendMagicLinkEmail(_ context.Context, to, name, signInURL string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.lastTo = to
	m.lastName = name
	m.lastURL = signInURL
	return m.returnErr
}

func (m *recordingMagicLinkEmailService) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls
}

func (m *recordingMagicLinkEmailService) SendVerificationEmail(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *recordingMagicLinkEmailService) SendPasswordResetEmail(_ context.Context, _, _, _ string) error {
	return nil
}

func (m *recordingMagicLinkEmailService) SendPasswordChangedEmail(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}

func (m *recordingMagicLinkEmailService) SendWelcomeEmail(_ context.Context, _, _ string) error {
	return nil
}

func (m *recordingMagicLinkEmailService) SendWarrantyReminderEmail(_ context.Context, _, _, _, _, _ string, _ int) error {
	return nil
}

func (m *recordingMagicLinkEmailService) SendGroupInviteEmail(_ context.Context, _, _, _, _, _ string, _ time.Time) error {
	return nil
}

func (m *recordingMagicLinkEmailService) SendStorageQuotaWarningEmail(_ context.Context, _, _, _ string, _, _ int, _, _ string, _ []string, _, _ string) error {
	return nil
}

func (m *recordingMagicLinkEmailService) SendLoanReminderEmail(_ context.Context, _, _, _, _, _, _, _, _ string, _ int) error {
	return nil
}

func (m *recordingMagicLinkEmailService) SendMaintenanceReminderEmail(_ context.Context, _, _, _, _, _, _ string, _ int) error {
	return nil
}

func (m *recordingMagicLinkEmailService) SendFeedbackEmail(_ context.Context, _, _, _, _, _, _, _ string, _ []string) error {
	return nil
}

const magicLinkTestTenantID = "test-tenant-id"

// magicLinkFixture wires an Auth router for the magic-link flow. The router
// injects magicLinkTestTenantID into every request context to mimic the tenant
// resolved by PublicTenantMiddleware in production. Tests reach into the
// registries to seed state and assert post-conditions.
type magicLinkFixture struct {
	router       chi.Router
	magicLinkReg *memreg.MagicLinkTokenRegistry
	userReg      *mockUserRegistryForAuth
	emailSvc     *recordingMagicLinkEmailService
}

type magicLinkOption func(*apiserver.AuthParams)

// newMagicLinkRouter builds an Auth router that injects injectedTenantID into
// every request context to mimic PublicTenantMiddleware. An empty string
// injects no tenant at all, exercising the verifyMagicLink "tenant context not
// established" branch.
func newMagicLinkRouter(params apiserver.AuthParams, injectedTenantID string) chi.Router {
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			ctx := req.Context()
			if injectedTenantID != "" {
				ctx = apiserver.WithTenantID(ctx, injectedTenantID)
			}
			next.ServeHTTP(w, req.WithContext(ctx))
		})
	})
	r.Route("/auth", apiserver.Auth(params))
	return r
}

func makeMagicLinkUser(active bool) *models.User {
	return &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "ml-user-1"},
			TenantID: magicLinkTestTenantID,
		},
		Email:    "ml@example.com",
		Name:     "Magic User",
		IsActive: active,
	}
}

// newMagicLinkFixture builds a fully-wired magic-link fixture with the given
// user seeded. opts can tweak the AuthParams (e.g. flip the gate off).
func newMagicLinkFixture(user *models.User, opts ...magicLinkOption) *magicLinkFixture {
	users := map[string]*models.User{}
	if user != nil {
		users[user.ID] = user
	}
	userReg := &mockUserRegistryForAuth{users: users}
	mlReg := memreg.NewMagicLinkTokenRegistry()
	emailSvc := &recordingMagicLinkEmailService{}

	params := apiserver.AuthParams{
		UserRegistry:          userReg,
		RefreshTokenRegistry:  memreg.NewRefreshTokenRegistry(),
		MagicLinkRegistry:     mlReg,
		EmailService:          emailSvc,
		PublicBaseURL:         "https://app.example.com",
		MagicLinkLoginEnabled: true,
		JWTSecret:             []byte("test-secret-32-bytes-minimum-length"),
	}
	for _, opt := range opts {
		opt(&params)
	}

	return &magicLinkFixture{
		router:       newMagicLinkRouter(params, magicLinkTestTenantID),
		magicLinkReg: mlReg,
		userReg:      userReg,
		emailSvc:     emailSvc,
	}
}

func magicLinkRequest(t *testing.T, router chi.Router, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if raw, ok := body.([]byte); ok {
		buf.Write(raw)
	} else if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(http.MethodPost, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

// -----------------------------------------------------------------------
// POST /auth/magic-link/request
// -----------------------------------------------------------------------

func TestMagicLinkRequest_GateDisabled(t *testing.T) {
	c := qt.New(t)
	user := makeMagicLinkUser(true)
	f := newMagicLinkFixture(user, func(p *apiserver.AuthParams) {
		p.MagicLinkLoginEnabled = false
	})
	resp := magicLinkRequest(t, f.router, "/auth/magic-link/request", map[string]string{"email": user.Email})
	c.Assert(resp.Code, qt.Equals, http.StatusNotFound)
}

func TestMagicLinkRequest_MalformedJSON(t *testing.T) {
	c := qt.New(t)
	f := newMagicLinkFixture(makeMagicLinkUser(true))
	resp := magicLinkRequest(t, f.router, "/auth/magic-link/request", []byte("{not-json"))
	c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
}

func TestMagicLinkRequest_EmptyEmail(t *testing.T) {
	c := qt.New(t)
	f := newMagicLinkFixture(makeMagicLinkUser(true))
	resp := magicLinkRequest(t, f.router, "/auth/magic-link/request", map[string]string{"email": ""})
	c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
}

// TestMagicLinkRequest_UnknownEmail pins the anti-enumeration contract: an
// unknown email returns a neutral 200, persists no token, and sends no email.
func TestMagicLinkRequest_UnknownEmail(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := qt.New(t)
		f := newMagicLinkFixture(nil) // no users seeded
		resp := magicLinkRequest(t, f.router, "/auth/magic-link/request", map[string]string{"email": "nobody@example.com"})
		c.Assert(resp.Code, qt.Equals, http.StatusOK)

		synctest.Wait()
		c.Assert(f.emailSvc.callCount(), qt.Equals, 0)
		all, err := f.magicLinkReg.List(t.Context())
		c.Assert(err, qt.IsNil)
		c.Assert(all, qt.HasLen, 0)
	})
}

// TestMagicLinkRequest_InactiveUser pins that a disabled account gets the same
// neutral 200 with no token persisted and no email sent.
func TestMagicLinkRequest_InactiveUser(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := qt.New(t)
		f := newMagicLinkFixture(makeMagicLinkUser(false))
		resp := magicLinkRequest(t, f.router, "/auth/magic-link/request", map[string]string{"email": "ml@example.com"})
		c.Assert(resp.Code, qt.Equals, http.StatusOK)

		synctest.Wait()
		c.Assert(f.emailSvc.callCount(), qt.Equals, 0)
		all, err := f.magicLinkReg.List(t.Context())
		c.Assert(err, qt.IsNil)
		c.Assert(all, qt.HasLen, 0)
	})
}

// TestMagicLinkRequest_ActiveUser pins the happy path: a token is persisted for
// the user and exactly one SendMagicLinkEmail is dispatched (asserted after the
// detached goroutine settles via synctest.Wait).
func TestMagicLinkRequest_ActiveUser(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := qt.New(t)
		user := makeMagicLinkUser(true)
		f := newMagicLinkFixture(user)
		resp := magicLinkRequest(t, f.router, "/auth/magic-link/request", map[string]string{"email": user.Email})
		c.Assert(resp.Code, qt.Equals, http.StatusOK)

		// A token row is persisted for the user.
		all, err := f.magicLinkReg.List(t.Context())
		c.Assert(err, qt.IsNil)
		c.Assert(all, qt.HasLen, 1)
		c.Assert(all[0].UserID, qt.Equals, user.ID)

		// The detached email goroutine sent exactly one magic-link email.
		synctest.Wait()
		c.Assert(f.emailSvc.callCount(), qt.Equals, 1)
		c.Assert(f.emailSvc.lastTo, qt.Equals, user.Email)
	})
}

// -----------------------------------------------------------------------
// POST /auth/magic-link/verify
// -----------------------------------------------------------------------

// seedMagicLinkToken creates a magic-link token row for the given user with the
// supplied expiry and returns its token value.
func seedMagicLinkToken(t *testing.T, f *magicLinkFixture, user *models.User, expiresAt time.Time) string {
	t.Helper()
	token, err := models.GenerateMagicLinkToken()
	if err != nil {
		t.Fatalf("generate magic-link token: %v", err)
	}
	_, err = f.magicLinkReg.Create(t.Context(), models.MagicLinkToken{
		UserID:    user.ID,
		TenantID:  user.TenantID,
		Email:     user.Email,
		Token:     token,
		ExpiresAt: expiresAt,
	})
	if err != nil {
		t.Fatalf("seed magic-link token: %v", err)
	}
	return token
}

func TestMagicLinkVerify_GateDisabled(t *testing.T) {
	c := qt.New(t)
	user := makeMagicLinkUser(true)
	f := newMagicLinkFixture(user, func(p *apiserver.AuthParams) {
		p.MagicLinkLoginEnabled = false
	})
	resp := magicLinkRequest(t, f.router, "/auth/magic-link/verify", map[string]string{"token": "anything"})
	c.Assert(resp.Code, qt.Equals, http.StatusNotFound)
}

func TestMagicLinkVerify_EmptyToken(t *testing.T) {
	c := qt.New(t)
	f := newMagicLinkFixture(makeMagicLinkUser(true))
	resp := magicLinkRequest(t, f.router, "/auth/magic-link/verify", map[string]string{"token": ""})
	c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
}

func TestMagicLinkVerify_UnknownToken(t *testing.T) {
	c := qt.New(t)
	f := newMagicLinkFixture(makeMagicLinkUser(true))
	resp := magicLinkRequest(t, f.router, "/auth/magic-link/verify", map[string]string{"token": "no-such-token"})
	c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
}

func TestMagicLinkVerify_ExpiredToken(t *testing.T) {
	c := qt.New(t)
	user := makeMagicLinkUser(true)
	f := newMagicLinkFixture(user)
	token := seedMagicLinkToken(t, f, user, time.Now().Add(-time.Minute))
	resp := magicLinkRequest(t, f.router, "/auth/magic-link/verify", map[string]string{"token": token})
	c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
}

func TestMagicLinkVerify_AlreadyClaimed(t *testing.T) {
	c := qt.New(t)
	user := makeMagicLinkUser(true)
	f := newMagicLinkFixture(user)
	token := seedMagicLinkToken(t, f, user, time.Now().Add(15*time.Minute))

	// Burn the token once via the registry, then replay it through the handler.
	claimed, err := f.magicLinkReg.MarkClaimed(t.Context(), token)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)

	resp := magicLinkRequest(t, f.router, "/auth/magic-link/verify", map[string]string{"token": token})
	c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
}

// TestMagicLinkVerify_NoTenantContext pins that a verify with no tenant resolved
// (PublicTenantMiddleware failed upstream) is a 500, not a silent success.
func TestMagicLinkVerify_NoTenantContext(t *testing.T) {
	c := qt.New(t)
	user := makeMagicLinkUser(true)
	params := apiserver.AuthParams{
		UserRegistry:          user2Reg(user),
		RefreshTokenRegistry:  memreg.NewRefreshTokenRegistry(),
		MagicLinkRegistry:     memreg.NewMagicLinkTokenRegistry(),
		EmailService:          &recordingMagicLinkEmailService{},
		MagicLinkLoginEnabled: true,
		JWTSecret:             []byte("test-secret-32-bytes-minimum-length"),
	}
	router := newMagicLinkRouter(params, "") // no tenant injected
	resp := magicLinkRequest(t, router, "/auth/magic-link/verify", map[string]string{"token": "whatever"})
	c.Assert(resp.Code, qt.Equals, http.StatusInternalServerError)
}

func user2Reg(user *models.User) *mockUserRegistryForAuth {
	return &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}}
}

// TestMagicLinkVerify_TenantMismatch pins the tenant guard: a token whose
// TenantID differs from the resolved tenant is rejected with 400.
func TestMagicLinkVerify_TenantMismatch(t *testing.T) {
	c := qt.New(t)
	user := makeMagicLinkUser(true)
	f := newMagicLinkFixture(user)

	// Seed a token belonging to a different tenant.
	token, err := models.GenerateMagicLinkToken()
	c.Assert(err, qt.IsNil)
	_, err = f.magicLinkReg.Create(t.Context(), models.MagicLinkToken{
		UserID:    user.ID,
		TenantID:  "some-other-tenant",
		Email:     user.Email,
		Token:     token,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	c.Assert(err, qt.IsNil)

	resp := magicLinkRequest(t, f.router, "/auth/magic-link/verify", map[string]string{"token": token})
	c.Assert(resp.Code, qt.Equals, http.StatusBadRequest)
}

// TestMagicLinkVerify_InactiveUser pins that a disabled account holding a valid
// token is refused at verify with 403 (mirrors login()).
func TestMagicLinkVerify_InactiveUser(t *testing.T) {
	c := qt.New(t)
	user := makeMagicLinkUser(false)
	f := newMagicLinkFixture(user)
	token := seedMagicLinkToken(t, f, user, time.Now().Add(15*time.Minute))
	resp := magicLinkRequest(t, f.router, "/auth/magic-link/verify", map[string]string{"token": token})
	c.Assert(resp.Code, qt.Equals, http.StatusForbidden)
}

// TestMagicLinkVerify_HappyPath pins the non-MFA completion: a valid token
// yields a LoginResponse with an access token, sets a refresh-token cookie, and
// is single-use (a second verify of the same token is rejected).
func TestMagicLinkVerify_HappyPath(t *testing.T) {
	c := qt.New(t)
	user := makeMagicLinkUser(true)
	f := newMagicLinkFixture(user)
	token := seedMagicLinkToken(t, f, user, time.Now().Add(15*time.Minute))

	resp := magicLinkRequest(t, f.router, "/auth/magic-link/verify", map[string]string{"token": token})
	c.Assert(resp.Code, qt.Equals, http.StatusOK)

	var loginResp apiserver.LoginResponse
	c.Assert(json.Unmarshal(resp.Body.Bytes(), &loginResp), qt.IsNil)
	c.Assert(loginResp.AccessToken, qt.Not(qt.Equals), "")
	c.Assert(loginResp.User, qt.IsNotNil)
	c.Assert(loginResp.User.ID, qt.Equals, user.ID)

	// A refresh-token cookie is set on the response.
	c.Assert(refreshCookieSet(resp), qt.IsTrue)

	// Single-use: replaying the same token is rejected.
	replay := magicLinkRequest(t, f.router, "/auth/magic-link/verify", map[string]string{"token": token})
	c.Assert(replay.Code, qt.Equals, http.StatusBadRequest)
}

// TestMagicLinkVerify_MFAEnrolled pins the MFA hand-off: a verify for a
// TOTP-enrolled user returns mfa_required + an mfa_token and does NOT mint a
// full session (no refresh cookie), mirroring loginMFA.
func TestMagicLinkVerify_MFAEnrolled(t *testing.T) {
	c := qt.New(t)
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	user := makeMagicLinkUser(true)

	userReg := user2Reg(user)
	mlReg := memreg.NewMagicLinkTokenRegistry()
	mfaReg := memreg.NewUserMFASecretRegistry()
	mfaSvc, err := services.NewMFAService(jwtSecret)
	c.Assert(err, qt.IsNil)

	// Enroll + enable MFA for the user via the real MFA setup/verify routes,
	// which require a Bearer token (mirrors newAuthMFAFixture/enrollAndEnable).
	tenant := &models.Tenant{
		EntityID: models.EntityID{ID: user.TenantID},
		Status:   models.TenantStatusActive,
	}
	authHandler := apiserver.Auth(apiserver.AuthParams{
		UserRegistry:          userReg,
		RefreshTokenRegistry:  memreg.NewRefreshTokenRegistry(),
		MagicLinkRegistry:     mlReg,
		MFARegistry:           mfaReg,
		MFAService:            mfaSvc,
		EmailService:          &recordingMagicLinkEmailService{},
		MagicLinkLoginEnabled: true,
		JWTSecret:             jwtSecret,
	})
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(apiserver.WithTenant(r.Context(), tenant)))
		})
	})
	router.Route("/auth", authHandler)

	bearer := mintMagicLinkBearer(t, jwtSecret, user.ID)
	callAuthed := func(method, path string, body any) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		if body != nil {
			c.Assert(json.NewEncoder(&buf).Encode(body), qt.IsNil)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+bearer)
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, req)
		return resp
	}

	setup := callAuthed(http.MethodPost, "/auth/mfa/setup", nil)
	c.Assert(setup.Code, qt.Equals, http.StatusOK)
	var setupResp apiserver.MFASetupResponse
	c.Assert(json.NewDecoder(setup.Body).Decode(&setupResp), qt.IsNil)
	code, err := totp.GenerateCodeCustom(setupResp.Secret, time.Now(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	c.Assert(err, qt.IsNil)
	verify := callAuthed(http.MethodPost, "/auth/mfa/verify", apiserver.MFAVerifyRequest{Code: code})
	c.Assert(verify.Code, qt.Equals, http.StatusOK)

	// Seed a live magic-link token and verify it: the user has MFA enabled,
	// so the response is an MFA challenge, not a full session.
	token, err := models.GenerateMagicLinkToken()
	c.Assert(err, qt.IsNil)
	_, err = mlReg.Create(t.Context(), models.MagicLinkToken{
		UserID:    user.ID,
		TenantID:  user.TenantID,
		Email:     user.Email,
		Token:     token,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	c.Assert(err, qt.IsNil)

	req := httptest.NewRequest(http.MethodPost, "/auth/magic-link/verify", bytes.NewBufferString(`{"token":"`+token+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	c.Assert(resp.Code, qt.Equals, http.StatusOK)
	var challenge apiserver.LoginMFARequiredResponse
	c.Assert(json.NewDecoder(resp.Body).Decode(&challenge), qt.IsNil)
	c.Assert(challenge.MFARequired, qt.IsTrue)
	c.Assert(challenge.MFAToken, qt.Not(qt.Equals), "")
	// No full session: the MFA hand-off must not set a refresh cookie.
	c.Assert(refreshCookieSet(resp), qt.IsFalse)
}

// mintMagicLinkBearer mints an access JWT the production RequireAuth middleware
// accepts, used to drive the Bearer-gated MFA enrollment routes.
func mintMagicLinkBearer(t *testing.T, secret []byte, userID string) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":        "ml-bearer-jti",
		"user_id":    userID,
		"token_type": "access",
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(5 * time.Minute).Unix(),
	})
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("sign bearer: %v", err)
	}
	return signed
}

// refreshCookieSet reports whether the response sets a non-empty refresh_token
// cookie (i.e. a full session was minted).
func refreshCookieSet(resp *httptest.ResponseRecorder) bool {
	for _, sc := range resp.Result().Cookies() {
		if sc.Name == "refresh_token" && sc.Value != "" && sc.MaxAge >= 0 {
			return true
		}
	}
	return false
}
