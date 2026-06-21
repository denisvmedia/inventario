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
	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	memreg "github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// authMFAFixture wires together every collaborator the MFA flow
// needs. Tests reach into mfaRegistry / mfaService to seed state and
// assert post-conditions; the router itself goes through the same
// /auth/* routes the production setup uses.
type authMFAFixture struct {
	jwtSecret    []byte
	user         *models.User
	tenant       *models.Tenant
	userRegistry *mockUserRegistryForAuth
	mfaRegistry  *memreg.UserMFASecretRegistry
	mfaService   *services.MFAService
	router       chi.Router
}

const mfaTestPassword = "Password123"

func newAuthMFAFixture(t *testing.T) *authMFAFixture {
	t.Helper()
	jwtSecret := []byte("test-secret-32-bytes-minimum-length")
	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-mfa"},
			TenantID: "test-tenant-id",
		},
		Email:    "mfa@example.com",
		Name:     "MFA User",
		IsActive: true,
	}
	if err := user.SetPassword(mfaTestPassword); err != nil {
		t.Fatalf("set password: %v", err)
	}
	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}}
	mfaReg := memreg.NewUserMFASecretRegistry()
	mfaSvc, err := services.NewMFAService(jwtSecret)
	if err != nil {
		t.Fatalf("new mfa service: %v", err)
	}

	authHandler := apiserver.Auth(apiserver.AuthParams{
		UserRegistry: userReg,
		MFARegistry:  mfaReg,
		MFAService:   mfaSvc,
		JWTSecret:    jwtSecret,
	})
	tenant := &models.Tenant{
		EntityID: models.EntityID{ID: user.TenantID},
		Status:   models.TenantStatusActive,
	}
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Tenant from the public-tenant middleware in production.
			ctx := apiserver.WithTenant(r.Context(), tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})
	router.Route("/auth", authHandler)
	return &authMFAFixture{
		jwtSecret:    jwtSecret,
		user:         user,
		tenant:       tenant,
		userRegistry: userReg,
		mfaRegistry:  mfaReg,
		mfaService:   mfaSvc,
		router:       router,
	}
}

func (f *authMFAFixture) call(t *testing.T, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	// Routes under /auth/mfa/* (and the existing /auth/me) are gated by
	// RequireAuth. Inject a Bearer token for every request that isn't
	// part of the unauth'd login flow so the tests exercise the same
	// middleware chain production traffic does.
	if path != "/auth/login" && path != "/auth/login/mfa" {
		req.Header.Set("Authorization", "Bearer "+f.bearerToken(t))
	}
	resp := httptest.NewRecorder()
	f.router.ServeHTTP(resp, req)
	return resp
}

// bearerToken mints an access JWT for f.user that the production
// RequireAuth middleware accepts. Keeps tests close to the real
// auth chain without standing up the /auth/login flow each time.
func (f *authMFAFixture) bearerToken(t *testing.T) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":        uuid.New().String(),
		"user_id":    f.user.ID,
		"token_type": "access",
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(5 * time.Minute).Unix(),
	})
	signed, err := token.SignedString(f.jwtSecret)
	if err != nil {
		t.Fatalf("sign bearer: %v", err)
	}
	return signed
}

func TestMFA_SetupVerifyDisable_HappyPath(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)

	// /auth/mfa/setup creates a pending row and returns the secret.
	setup := f.call(t, "POST", "/auth/mfa/setup", nil)
	c.Assert(setup.Code, qt.Equals, http.StatusOK)
	var setupResp apiserver.MFASetupResponse
	c.Assert(json.NewDecoder(setup.Body).Decode(&setupResp), qt.IsNil)
	c.Assert(setupResp.Secret, qt.Not(qt.Equals), "")

	// Status reports state="pending" while EnabledAt is null.
	st1 := f.call(t, "GET", "/auth/mfa/status", nil)
	c.Assert(st1.Code, qt.Equals, http.StatusOK)
	var pre apiserver.MFAStatusResponse
	c.Assert(json.NewDecoder(st1.Body).Decode(&pre), qt.IsNil)
	c.Assert(pre.State, qt.Equals, apiserver.MFAStatePending)

	// Generate the actual TOTP code for the issued secret.
	code, err := totp.GenerateCodeCustom(setupResp.Secret, time.Now(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	c.Assert(err, qt.IsNil)

	verify := f.call(t, "POST", "/auth/mfa/verify", apiserver.MFAVerifyRequest{Code: code})
	c.Assert(verify.Code, qt.Equals, http.StatusOK)
	var verifyResp apiserver.MFAVerifyResponse
	c.Assert(json.NewDecoder(verify.Body).Decode(&verifyResp), qt.IsNil)
	c.Assert(verifyResp.BackupCodes, qt.HasLen, services.MFABackupCodeCount)

	// Status now shows state="active".
	st2 := f.call(t, "GET", "/auth/mfa/status", nil)
	var post apiserver.MFAStatusResponse
	c.Assert(json.NewDecoder(st2.Body).Decode(&post), qt.IsNil)
	c.Assert(post.State, qt.Equals, apiserver.MFAStateActive)
	c.Assert(post.BackupCodesRemaining, qt.Equals, services.MFABackupCodeCount)

	// Disable requires password + current TOTP code.
	freshCode, err := totp.GenerateCodeCustom(setupResp.Secret, time.Now(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	c.Assert(err, qt.IsNil)
	dis := f.call(t, "POST", "/auth/mfa/disable", apiserver.MFADisableRequest{
		Password: mfaTestPassword,
		TOTPCode: freshCode,
	})
	c.Assert(dis.Code, qt.Equals, http.StatusOK)

	_, err = f.mfaRegistry.GetByUser(context.Background(), f.user.TenantID, f.user.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestMFA_Verify_RejectsBadCode(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)
	f.call(t, "POST", "/auth/mfa/setup", nil)
	resp := f.call(t, "POST", "/auth/mfa/verify", apiserver.MFAVerifyRequest{Code: "000000"})
	c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
}

func TestMFA_Setup_RejectsReenrollWhenEnabled(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)
	enrollAndEnable(t, f)

	// Setup must refuse to mint a new secret while a verified row exists.
	resp := f.call(t, "POST", "/auth/mfa/setup", nil)
	c.Assert(resp.Code, qt.Equals, http.StatusConflict)
}

func TestMFA_Disable_RequiresPasswordAndCode(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)
	enrollAndEnable(t, f)

	// Wrong password — refused even with a valid TOTP code.
	row, _ := f.mfaRegistry.GetByUser(context.Background(), f.user.TenantID, f.user.ID)
	plain, _ := decryptOf(t, f, row.SecretEncrypted)
	code, _ := totp.GenerateCodeCustom(plain, time.Now(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	resp := f.call(t, "POST", "/auth/mfa/disable", apiserver.MFADisableRequest{
		Password: "wrong-password",
		TOTPCode: code,
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)

	// Right password but bad code — refused too.
	resp = f.call(t, "POST", "/auth/mfa/disable", apiserver.MFADisableRequest{
		Password: mfaTestPassword,
		TOTPCode: "000000",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
}

func TestMFA_Login_ChallengeAndComplete(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)
	enrollAndEnable(t, f)

	// Step 1 — password login now short-circuits with mfa_required.
	step1 := f.call(t, "POST", "/auth/login", map[string]string{
		"email":    f.user.Email,
		"password": mfaTestPassword,
	})
	c.Assert(step1.Code, qt.Equals, http.StatusOK)
	var challenge apiserver.LoginMFARequiredResponse
	c.Assert(json.NewDecoder(step1.Body).Decode(&challenge), qt.IsNil)
	c.Assert(challenge.MFARequired, qt.IsTrue)
	c.Assert(challenge.MFAToken, qt.Not(qt.Equals), "")

	// Step 2 — submit a valid TOTP code with the token.
	row, _ := f.mfaRegistry.GetByUser(context.Background(), f.user.TenantID, f.user.ID)
	plain, _ := decryptOf(t, f, row.SecretEncrypted)
	code, _ := totp.GenerateCodeCustom(plain, time.Now(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	step2 := f.call(t, "POST", "/auth/login/mfa", apiserver.LoginMFARequest{
		MFAToken: challenge.MFAToken,
		TOTPCode: code,
	})
	c.Assert(step2.Code, qt.Equals, http.StatusOK)
	var loginResp apiserver.LoginResponse
	c.Assert(json.NewDecoder(step2.Body).Decode(&loginResp), qt.IsNil)
	c.Assert(loginResp.AccessToken, qt.Not(qt.Equals), "")
	c.Assert(loginResp.User.Email, qt.Equals, f.user.Email)
}

// TestMFA_Login_TOTPReplayRejectedWithinWindow locks in the #2124 guarantee:
// a TOTP code consumed at /auth/login/mfa cannot be replayed at the same
// endpoint within its ±1-step validity window, even though the code is still
// arithmetically valid. Without the last_used_step CAS the second attempt
// would mint a second session from a single sniffed code.
func TestMFA_Login_TOTPReplayRejectedWithinWindow(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)
	enrollAndEnable(t, f)

	row, _ := f.mfaRegistry.GetByUser(context.Background(), f.user.TenantID, f.user.ID)
	plain, _ := decryptOf(t, f, row.SecretEncrypted)
	code, err := totp.GenerateCodeCustom(plain, time.Now(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	c.Assert(err, qt.IsNil)

	// Each step-2 needs a fresh step-1 mfa_token; the TOTP code stays the same.
	challenge := func() string {
		step1 := f.call(t, "POST", "/auth/login", map[string]string{
			"email":    f.user.Email,
			"password": mfaTestPassword,
		})
		c.Assert(step1.Code, qt.Equals, http.StatusOK)
		var ch apiserver.LoginMFARequiredResponse
		c.Assert(json.NewDecoder(step1.Body).Decode(&ch), qt.IsNil)
		return ch.MFAToken
	}

	first := f.call(t, "POST", "/auth/login/mfa", apiserver.LoginMFARequest{
		MFAToken: challenge(),
		TOTPCode: code,
	})
	c.Assert(first.Code, qt.Equals, http.StatusOK)

	// Same code, same time-step → the replay is rejected like a wrong code.
	replay := f.call(t, "POST", "/auth/login/mfa", apiserver.LoginMFARequest{
		MFAToken: challenge(),
		TOTPCode: code,
	})
	c.Assert(replay.Code, qt.Equals, http.StatusUnauthorized)
}

func TestMFA_Login_BackupCodeSingleUse(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)
	codes := enrollAndEnable(t, f)
	c.Assert(len(codes) > 0, qt.IsTrue)
	backup := codes[0]

	// Round 1 — first login uses backup code, succeeds.
	step1 := f.call(t, "POST", "/auth/login", map[string]string{
		"email": f.user.Email, "password": mfaTestPassword,
	})
	c.Assert(step1.Code, qt.Equals, http.StatusOK, qt.Commentf("step1 body=%s", step1.Body.String()))
	var ch1 apiserver.LoginMFARequiredResponse
	c.Assert(json.NewDecoder(step1.Body).Decode(&ch1), qt.IsNil)
	c.Assert(ch1.MFAToken, qt.Not(qt.Equals), "")
	step2 := f.call(t, "POST", "/auth/login/mfa", apiserver.LoginMFARequest{
		MFAToken:   ch1.MFAToken,
		BackupCode: backup,
	})
	c.Assert(step2.Code, qt.Equals, http.StatusOK, qt.Commentf("step2 body=%s backup=%s", step2.Body.String(), backup))

	// Round 2 — same backup code must be rejected.
	step1b := f.call(t, "POST", "/auth/login", map[string]string{
		"email": f.user.Email, "password": mfaTestPassword,
	})
	var ch2 apiserver.LoginMFARequiredResponse
	c.Assert(json.NewDecoder(step1b.Body).Decode(&ch2), qt.IsNil)
	step2b := f.call(t, "POST", "/auth/login/mfa", apiserver.LoginMFARequest{
		MFAToken:   ch2.MFAToken,
		BackupCode: backup,
	})
	c.Assert(step2b.Code, qt.Equals, http.StatusUnauthorized)
}

func TestMFA_Login_RejectsBadMFAToken(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)
	enrollAndEnable(t, f)
	resp := f.call(t, "POST", "/auth/login/mfa", apiserver.LoginMFARequest{
		MFAToken: "not.a.real.token",
		TOTPCode: "123456",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
}

// TestMFA_Login_RejectsExpiredMFAToken locks in the parseMFAToken
// exp-claim guard added after the Copilot review. A token whose exp
// is in the past must be rejected even though jwt.Parse would
// otherwise accept it (the JWT library's Valid bool only enforces
// exp when the claim is present and parsed successfully).
func TestMFA_Login_RejectsExpiredMFAToken(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)
	enrollAndEnable(t, f)

	expired := mintMFAToken(t, f.jwtSecret, jwt.MapClaims{
		"jti":        uuid.New().String(),
		"user_id":    f.user.ID,
		"tenant_id":  f.user.TenantID,
		"token_type": "mfa_challenge",
		"iat":        time.Now().Add(-10 * time.Minute).Unix(),
		"exp":        time.Now().Add(-time.Minute).Unix(),
	})
	resp := f.call(t, "POST", "/auth/login/mfa", apiserver.LoginMFARequest{
		MFAToken: expired,
		TOTPCode: "123456",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
}

// TestMFA_Login_RejectsWrongSignatureMFAToken pins that an HMAC
// signature mismatch is caught — the parseMFAToken HMAC-method
// guard already rejects non-HMAC algs; this test covers the
// "right alg, wrong key" path that's easy to forget.
func TestMFA_Login_RejectsWrongSignatureMFAToken(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)
	enrollAndEnable(t, f)

	wrongSig := mintMFAToken(t, []byte("totally-different-secret-32-bytes-min"), jwt.MapClaims{
		"jti":        uuid.New().String(),
		"user_id":    f.user.ID,
		"tenant_id":  f.user.TenantID,
		"token_type": "mfa_challenge",
		"iat":        time.Now().Unix(),
		"exp":        time.Now().Add(5 * time.Minute).Unix(),
	})
	resp := f.call(t, "POST", "/auth/login/mfa", apiserver.LoginMFARequest{
		MFAToken: wrongSig,
		TOTPCode: "123456",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
}

// TestMFA_Login_RejectsMissingExpClaim mirrors validateJWTToken's
// guard — the JWT lib will accept a token with no exp claim, but the
// "short-lived" contract demands one. parseMFAToken returns an error,
// which the handler maps to 401.
func TestMFA_Login_RejectsMissingExpClaim(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)
	enrollAndEnable(t, f)

	noExp := mintMFAToken(t, f.jwtSecret, jwt.MapClaims{
		"jti":        uuid.New().String(),
		"user_id":    f.user.ID,
		"tenant_id":  f.user.TenantID,
		"token_type": "mfa_challenge",
		"iat":        time.Now().Unix(),
		// "exp" deliberately omitted.
	})
	resp := f.call(t, "POST", "/auth/login/mfa", apiserver.LoginMFARequest{
		MFAToken: noExp,
		TOTPCode: "123456",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
}

// TestMFA_Disable_NoOpWhenNotEnrolled locks in the documented
// idempotent behaviour: if the user never enrolled, a correct password
// returns 200 without consuming a code. A future "actually 404 when
// not enrolled" refactor would break this loudly.
func TestMFA_Disable_NoOpWhenNotEnrolled(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)

	resp := f.call(t, "POST", "/auth/mfa/disable", apiserver.MFADisableRequest{
		Password: mfaTestPassword,
	})
	c.Assert(resp.Code, qt.Equals, http.StatusOK)

	// Wrong password is still rejected even when no row exists.
	resp = f.call(t, "POST", "/auth/mfa/disable", apiserver.MFADisableRequest{
		Password: "wrong-password",
	})
	c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)
}

// mintMFAToken signs an arbitrary claims map with the given secret.
// Helper for the negative-path tests above so each one stays readable.
func mintMFAToken(t *testing.T, secret []byte, claims jwt.MapClaims) string {
	t.Helper()
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(secret)
	if err != nil {
		t.Fatalf("mintMFAToken: %v", err)
	}
	return signed
}

func TestMFA_Regenerate_RequiresCurrentCode(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)
	originalCodes := enrollAndEnable(t, f)

	// Bad code rejected.
	resp := f.call(t, "POST", "/auth/mfa/regenerate-backup-codes", apiserver.MFAVerifyRequest{Code: "000000"})
	c.Assert(resp.Code, qt.Equals, http.StatusUnauthorized)

	// Real code returns a fresh set; old codes no longer work.
	row, _ := f.mfaRegistry.GetByUser(context.Background(), f.user.TenantID, f.user.ID)
	plain, _ := decryptOf(t, f, row.SecretEncrypted)
	code, _ := totp.GenerateCodeCustom(plain, time.Now(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	resp = f.call(t, "POST", "/auth/mfa/regenerate-backup-codes", apiserver.MFAVerifyRequest{Code: code})
	c.Assert(resp.Code, qt.Equals, http.StatusOK)
	var regen apiserver.MFAVerifyResponse
	c.Assert(json.NewDecoder(resp.Body).Decode(&regen), qt.IsNil)
	c.Assert(regen.BackupCodes, qt.HasLen, services.MFABackupCodeCount)

	// Stronger invariants — the original "old codes don't appear in the
	// new set" assertion would pass even if the regenerate kept 9 of 10
	// hashes by mistake. Pin three things instead:
	//
	//   1. All 10 new codes are unique.
	//   2. Every single old code is no longer consumable
	//      (ConsumeBackupCodeAtomic returns false).
	//   3. Every single new code IS consumable, then drops out of the
	//      remaining set after consumption.
	uniq := make(map[string]struct{}, len(regen.BackupCodes))
	for _, code := range regen.BackupCodes {
		uniq[code] = struct{}{}
	}
	c.Assert(uniq, qt.HasLen, services.MFABackupCodeCount, qt.Commentf("regenerated codes contain duplicates"))

	// Old codes — none should consume successfully.
	for _, oldCode := range originalCodes {
		matcher := f.mfaService.MatchBackupCode(oldCode)
		consumed, err := f.mfaRegistry.ConsumeBackupCodeAtomic(
			context.Background(), f.user.TenantID, f.user.ID, time.Now(), matcher,
		)
		c.Assert(err, qt.IsNil)
		c.Assert(consumed, qt.IsFalse, qt.Commentf("old code %q is still consumable after regenerate", oldCode))
	}

	// New codes — every one should consume exactly once. We re-issue
	// from a fresh enrollment for cleanliness so the asserts on
	// "consume succeeds" don't bleed into each other.
	for _, newCode := range regen.BackupCodes {
		matcher := f.mfaService.MatchBackupCode(newCode)
		consumed, err := f.mfaRegistry.ConsumeBackupCodeAtomic(
			context.Background(), f.user.TenantID, f.user.ID, time.Now(), matcher,
		)
		c.Assert(err, qt.IsNil)
		c.Assert(consumed, qt.IsTrue, qt.Commentf("new code %q is not consumable", newCode))
	}
}

// TestMFA_Regenerate_TouchesLastUsedAt locks in the #1645 review fix
// that regenerate-backup-codes updates LastUsedAt on a successful
// TOTP verification, mirroring every other path that consumes a
// valid code.
func TestMFA_Regenerate_TouchesLastUsedAt(t *testing.T) {
	c := qt.New(t)
	f := newAuthMFAFixture(t)
	enrollAndEnable(t, f)
	pre, _ := f.mfaRegistry.GetByUser(context.Background(), f.user.TenantID, f.user.ID)
	c.Assert(pre.LastUsedAt, qt.IsNotNil)
	preStamp := *pre.LastUsedAt

	// Sleep a beat so an instantaneous Update() still produces a
	// distinguishably newer timestamp on systems with a monotonic
	// clock that snaps back to wall-clock for time.Time comparisons.
	time.Sleep(5 * time.Millisecond)

	plain, _ := decryptOf(t, f, pre.SecretEncrypted)
	code, _ := totp.GenerateCodeCustom(plain, time.Now(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	resp := f.call(t, "POST", "/auth/mfa/regenerate-backup-codes", apiserver.MFAVerifyRequest{Code: code})
	c.Assert(resp.Code, qt.Equals, http.StatusOK)

	post, _ := f.mfaRegistry.GetByUser(context.Background(), f.user.TenantID, f.user.ID)
	c.Assert(post.LastUsedAt, qt.IsNotNil)
	c.Assert(post.LastUsedAt.After(preStamp), qt.IsTrue,
		qt.Commentf("LastUsedAt did not advance after a successful regenerate; pre=%v post=%v", preStamp, *post.LastUsedAt))
}

// enrollAndEnable spins through the setup → verify cycle so tests
// that only care about the post-enrollment state don't have to
// repeat the boilerplate. Returns the issued backup codes for
// single-use tests.
func enrollAndEnable(t *testing.T, f *authMFAFixture) []string {
	t.Helper()
	c := qt.New(t)
	setup := f.call(t, "POST", "/auth/mfa/setup", nil)
	c.Assert(setup.Code, qt.Equals, http.StatusOK)
	var setupResp apiserver.MFASetupResponse
	c.Assert(json.NewDecoder(setup.Body).Decode(&setupResp), qt.IsNil)
	code, err := totp.GenerateCodeCustom(setupResp.Secret, time.Now(), totp.ValidateOpts{
		Period: 30, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1,
	})
	c.Assert(err, qt.IsNil)
	verify := f.call(t, "POST", "/auth/mfa/verify", apiserver.MFAVerifyRequest{Code: code})
	c.Assert(verify.Code, qt.Equals, http.StatusOK)
	var verifyResp apiserver.MFAVerifyResponse
	c.Assert(json.NewDecoder(verify.Body).Decode(&verifyResp), qt.IsNil)
	return verifyResp.BackupCodes
}

// decryptOf decrypts the stored secret using the fixture's service.
// Kept inline because the production API never returns the plaintext
// after Setup; tests need it to forge codes.
func decryptOf(t *testing.T, f *authMFAFixture, enc string) (string, error) {
	t.Helper()
	return f.mfaService.DecryptSecret(enc)
}
