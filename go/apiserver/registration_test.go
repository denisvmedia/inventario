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
	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// duplicateEmailUserRegistry simulates the race where the duplicate-check in
// handleRegister passes (no row found) but the subsequent Create() loses a
// concurrent insert and surfaces ErrEmailAlreadyExists. The handler must
// translate that into the same anti-enumeration 200 response.
type duplicateEmailUserRegistry struct {
	*registrationUserRegistry
}

func (d *duplicateEmailUserRegistry) Create(_ context.Context, _ models.User) (*models.User, error) {
	return nil, registry.ErrEmailAlreadyExists
}

// postResendVerification posts a JSON body to /resend-verification. It mirrors
// postRegister in registration_invite_test.go so the tests below stay short.
func postResendVerification(c *qt.C, r chi.Router, payload map[string]string) *httptest.ResponseRecorder {
	body, err := json.Marshal(payload)
	c.Assert(err, qt.IsNil)
	req := httptest.NewRequest(http.MethodPost, "/resend-verification", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// assertNoVerificationEmail fails the test if SendVerificationEmail is invoked
// at any point during the grace window. It polls the atomic counter and the
// channel throughout the window rather than checking once at the end, so a
// goroutine scheduled late on a slow CI worker cannot slip past a single
// end-of-window check while still being incorrectly spawned. The 500ms budget
// matches the cancellation-path tests' window so this helper does not flake on
// the same workers those tests already pass on.
func assertNoVerificationEmail(t *testing.T, emailSvc *blockingEmailService) {
	t.Helper()
	const (
		window   = 500 * time.Millisecond
		pollStep = 20 * time.Millisecond
	)
	deadline := time.Now().Add(window)
	for time.Now().Before(deadline) {
		select {
		case <-emailSvc.verificationCh:
			t.Fatal("expected no verification email, but one was dispatched")
		case <-time.After(pollStep):
		}
		if got := emailSvc.verificationCalls.Load(); got != 0 {
			t.Fatalf("expected no verification email, but SendVerificationEmail was invoked %d time(s)", got)
		}
	}
}

// newVerifyRequest builds a GET request for /verify-email with the given token
// in the query string. Pass an empty string to omit the token entirely (which
// is the "missing token" scenario).
func newVerifyRequest(token string) *http.Request {
	target := "/verify-email"
	if token != "" {
		target += "?token=" + token
	}
	return httptest.NewRequest(http.MethodGet, target, nil)
}

// ---- handleRegister --------------------------------------------------------

func TestHandleRegister_InvalidJSONBodyReturns400(t *testing.T) {
	c := qt.New(t)
	userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	req := httptest.NewRequest(http.MethodPost, "/register", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
	c.Assert(userReg.users, qt.HasLen, 0)
}

func TestHandleRegister_MissingRequiredFieldsReturns400(t *testing.T) {
	cases := []struct {
		name    string
		payload map[string]string
	}{
		{name: "missing email", payload: map[string]string{"name": "Someone", "password": "Password123"}},
		{name: "missing password", payload: map[string]string{"email": "x@example.com", "name": "Someone"}},
		{name: "missing name", payload: map[string]string{"email": "x@example.com", "password": "Password123"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
			r := newRegistrationRouter(apiserver.RegistrationParams{
				UserRegistry:         userReg,
				VerificationRegistry: memory.NewEmailVerificationRegistry(),
				RateLimiter:          services.NewInMemoryAuthRateLimiter(),
			}, models.RegistrationModeOpen)

			w := postRegister(c, r, tc.payload)
			c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
			c.Assert(userReg.users, qt.HasLen, 0)
		})
	}
}

func TestHandleRegister_WeakPasswordReturns400(t *testing.T) {
	c := qt.New(t)
	userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	w := postRegister(c, r, map[string]string{
		"email":    "weak@example.com",
		"name":     "Weak",
		"password": "weak", // too short, no uppercase, no digit
	})
	c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
	c.Assert(userReg.users, qt.HasLen, 0)
}

func TestHandleRegister_DuplicateEmailReturns200WithoutSideEffects(t *testing.T) {
	c := qt.New(t)
	existing := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "existing-user"},
			TenantID: testTenantID,
		},
		Email:    "dupe@example.com",
		Name:     "Existing",
		IsActive: true,
	}
	userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{existing.ID: existing}}}
	emailSvc := &blockingEmailService{verificationCh: make(chan asyncEmailObservation, 1)}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		EmailService:         emailSvc,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	w := postRegister(c, r, map[string]string{
		"email":    existing.Email,
		"name":     "Someone Else",
		"password": "Password123",
	})

	c.Assert(w.Code, qt.Equals, http.StatusOK)
	// Anti-enumeration: even for a duplicate, the response is the open-mode
	// "check your email to verify" message — same wording an attacker would
	// see for a fresh registration.
	c.Assert(w.Body.String(), qt.Contains, "check your email")

	// Only the pre-existing user remains; no new row was inserted.
	c.Assert(userReg.users, qt.HasLen, 1)
	_, ok := userReg.users[existing.ID]
	c.Assert(ok, qt.IsTrue)

	// And no verification email may fire for a duplicate. Polled across the
	// grace window so a late-scheduled goroutine cannot slip past a single
	// end-of-window check.
	assertNoVerificationEmail(t, emailSvc)
}

func TestHandleRegister_CreateRaceErrEmailAlreadyExistsReturns200(t *testing.T) {
	c := qt.New(t)
	base := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
	userReg := &duplicateEmailUserRegistry{registrationUserRegistry: base}
	emailSvc := &blockingEmailService{verificationCh: make(chan asyncEmailObservation, 1)}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		EmailService:         emailSvc,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	w := postRegister(c, r, map[string]string{
		"email":    "race@example.com",
		"name":     "Race",
		"password": "Password123",
	})

	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Body.String(), qt.Contains, "check your email")
	c.Assert(base.users, qt.HasLen, 0,
		qt.Commentf("Create returned ErrEmailAlreadyExists, so no user must be persisted"))

	// No verification email when the create-side race is detected.
	assertNoVerificationEmail(t, emailSvc)
}

func TestHandleRegister_ApprovalModeCreatesPendingUserNoEmail(t *testing.T) {
	c := qt.New(t)
	userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
	emailSvc := &blockingEmailService{
		verificationCh: make(chan asyncEmailObservation, 1),
		welcome:        make(chan struct{}, 1),
	}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		EmailService:         emailSvc,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeApproval)

	w := postRegister(c, r, map[string]string{
		"email":    "pending@example.com",
		"name":     "Pending",
		"password": "Password123",
	})

	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Body.String(), qt.Contains, "pending administrator approval")
	c.Assert(userReg.users, qt.HasLen, 1)
	for _, u := range userReg.users {
		c.Assert(u.IsActive, qt.IsFalse,
			qt.Commentf("approval-mode users must wait for an admin to activate them"))
		c.Assert(u.Email, qt.Equals, "pending@example.com")
	}

	// Approval mode must never fire a verification email (only open mode does).
	assertNoVerificationEmail(t, emailSvc)
}

// ---- handleVerifyEmail -----------------------------------------------------

func TestHandleVerifyEmail_MissingTokenReturns400(t *testing.T) {
	c := qt.New(t)
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         &mockUserRegistryForAuth{users: map[string]*models.User{}},
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newVerifyRequest(""))
	c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
}

func TestHandleVerifyEmail_UnknownTokenReturns400(t *testing.T) {
	c := qt.New(t)
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         &mockUserRegistryForAuth{users: map[string]*models.User{}},
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newVerifyRequest("no-such-token"))
	c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
	c.Assert(w.Body.String(), qt.Contains, "Invalid or expired")
}

func TestHandleVerifyEmail_AlreadyVerifiedTokenReturns200(t *testing.T) {
	c := qt.New(t)

	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "verified-user"},
			TenantID: testTenantID,
		},
		Email:    "verified@example.com",
		Name:     "Verified",
		IsActive: true,
	}
	verReg := memory.NewEmailVerificationRegistry()
	now := time.Now()
	ev, err := verReg.Create(context.Background(), models.EmailVerification{
		UserID:     user.ID,
		TenantID:   testTenantID,
		Email:      user.Email,
		Token:      "already-verified-token",
		ExpiresAt:  now.Add(time.Hour),
		VerifiedAt: &now,
	})
	c.Assert(err, qt.IsNil)

	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: verReg,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newVerifyRequest(ev.Token))
	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Body.String(), qt.Contains, "already verified")
}

func TestHandleVerifyEmail_ExpiredTokenReturns400(t *testing.T) {
	c := qt.New(t)

	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "expired-user"},
			TenantID: testTenantID,
		},
		Email: "expired@example.com",
		Name:  "Expired",
	}
	verReg := memory.NewEmailVerificationRegistry()
	ev, err := verReg.Create(context.Background(), models.EmailVerification{
		UserID:    user.ID,
		TenantID:  testTenantID,
		Email:     user.Email,
		Token:     "expired-token",
		ExpiresAt: time.Now().Add(-time.Minute),
	})
	c.Assert(err, qt.IsNil)

	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: verReg,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newVerifyRequest(ev.Token))
	c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
	c.Assert(w.Body.String(), qt.Contains, "expired")

	// User stays inactive when the verification token has already expired.
	got, err := userReg.Get(context.Background(), user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.IsActive, qt.IsFalse)
}

func TestHandleVerifyEmail_HappyPathActivatesUserAndSendsWelcome(t *testing.T) {
	c := qt.New(t)

	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "to-activate"},
			TenantID: testTenantID,
		},
		Email: "activate@example.com",
		Name:  "To Activate",
	}
	verReg := memory.NewEmailVerificationRegistry()
	ev, err := verReg.Create(context.Background(), models.EmailVerification{
		UserID:    user.ID,
		TenantID:  testTenantID,
		Email:     user.Email,
		Token:     "good-token",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)

	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}}
	emailSvc := &blockingEmailService{welcome: make(chan struct{}, 1)}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: verReg,
		EmailService:         emailSvc,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newVerifyRequest(ev.Token))
	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Body.String(), qt.Contains, "Email verified")

	got, err := userReg.Get(context.Background(), user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.IsActive, qt.IsTrue, qt.Commentf("happy-path verification must flip IsActive to true"))

	updated, err := verReg.GetByToken(context.Background(), ev.Token)
	c.Assert(err, qt.IsNil)
	c.Assert(updated.IsVerified(), qt.IsTrue,
		qt.Commentf("verification record must be marked verified after a successful verify"))

	select {
	case <-emailSvc.welcome:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("welcome email should be dispatched after a successful verification")
	}
}

// claimLosingVerificationRegistry simulates the #1005 race loser: GetByToken
// (and everything else) behaves like the embedded real registry, but the
// atomic MarkVerified claim always reports false — i.e. a concurrent request
// already won the token. The handler must then take the idempotent
// "already verified" path and skip the one-time side effects.
type claimLosingVerificationRegistry struct {
	registry.EmailVerificationRegistry
}

func (*claimLosingVerificationRegistry) MarkVerified(context.Context, string) (bool, error) {
	return false, nil
}

// TestHandleVerifyEmail_ConcurrentLoserIsIdempotentAndSkipsWelcome pins the
// #1005 fix at the handler level: a request that loses the atomic claim still
// returns a 200 "already verified" but must NOT dispatch a second welcome
// email or otherwise re-run first-verification side effects.
func TestHandleVerifyEmail_ConcurrentLoserIsIdempotentAndSkipsWelcome(t *testing.T) {
	c := qt.New(t)

	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "race-loser"},
			TenantID: testTenantID,
		},
		Email: "loser@example.com",
		Name:  "Race Loser",
	}
	mem := memory.NewEmailVerificationRegistry()
	ev, err := mem.Create(context.Background(), models.EmailVerification{
		UserID:    user.ID,
		TenantID:  testTenantID,
		Email:     user.Email,
		Token:     "loser-token",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)

	verReg := &claimLosingVerificationRegistry{EmailVerificationRegistry: mem}
	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}}
	emailSvc := &blockingEmailService{welcome: make(chan struct{}, 1)}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: verReg,
		EmailService:         emailSvc,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, newVerifyRequest(ev.Token))
	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Body.String(), qt.Contains, "already verified")

	select {
	case <-emailSvc.welcome:
		t.Fatal("a request that lost the verification claim must not send a welcome email")
	case <-time.After(500 * time.Millisecond):
	}
}

// TestHandleVerifyEmail_SecondVerifyIsIdempotent drives the real memory
// registry end-to-end: after a successful verify marks the token, a second
// request with the same token takes the IsVerified() fast path, returns 200,
// and dispatches no further welcome email.
func TestHandleVerifyEmail_SecondVerifyIsIdempotent(t *testing.T) {
	c := qt.New(t)

	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "verify-twice"},
			TenantID: testTenantID,
		},
		Email: "twice@example.com",
		Name:  "Verify Twice",
	}
	verReg := memory.NewEmailVerificationRegistry()
	ev, err := verReg.Create(context.Background(), models.EmailVerification{
		UserID:    user.ID,
		TenantID:  testTenantID,
		Email:     user.Email,
		Token:     "twice-token",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)

	userReg := &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}}
	// Buffer 2 so a (buggy) second welcome would land instead of deadlocking.
	emailSvc := &blockingEmailService{welcome: make(chan struct{}, 2)}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: verReg,
		EmailService:         emailSvc,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	// First verify: succeeds and sends exactly one welcome.
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, newVerifyRequest(ev.Token))
	c.Assert(w1.Code, qt.Equals, http.StatusOK)
	c.Assert(w1.Body.String(), qt.Contains, "Email verified")
	select {
	case <-emailSvc.welcome:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("first verification should dispatch a welcome email")
	}

	// Second verify with the same token: idempotent, no new welcome.
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, newVerifyRequest(ev.Token))
	c.Assert(w2.Code, qt.Equals, http.StatusOK)
	c.Assert(w2.Body.String(), qt.Contains, "already verified")
	select {
	case <-emailSvc.welcome:
		t.Fatal("a repeat verification must not dispatch a second welcome email")
	case <-time.After(500 * time.Millisecond):
	}
}

// ---- handleResendVerification ---------------------------------------------

func TestHandleResendVerification_InvalidJSONReturns400(t *testing.T) {
	c := qt.New(t)
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         &mockUserRegistryForAuth{users: map[string]*models.User{}},
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	req := httptest.NewRequest(http.MethodPost, "/resend-verification", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusBadRequest)
}

func TestHandleResendVerification_UnknownEmailReturns200(t *testing.T) {
	c := qt.New(t)
	emailSvc := &blockingEmailService{verificationCh: make(chan asyncEmailObservation, 1)}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         &mockUserRegistryForAuth{users: map[string]*models.User{}},
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		EmailService:         emailSvc,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	w := postResendVerification(c, r, map[string]string{"email": "ghost@example.com"})
	c.Assert(w.Code, qt.Equals, http.StatusOK)
	// Anti-enumeration: identical wording an attacker would see for a real
	// pending account, so the response cannot leak whether the email is known.
	c.Assert(w.Body.String(), qt.Contains, "If the email exists")

	assertNoVerificationEmail(t, emailSvc)
}

func TestHandleResendVerification_AlreadyActiveUserReturns200WithoutEmail(t *testing.T) {
	c := qt.New(t)
	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "active-user"},
			TenantID: testTenantID,
		},
		Email:    "active@example.com",
		Name:     "Active",
		IsActive: true,
	}
	emailSvc := &blockingEmailService{verificationCh: make(chan asyncEmailObservation, 1)}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}},
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		EmailService:         emailSvc,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	w := postResendVerification(c, r, map[string]string{"email": user.Email})
	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Body.String(), qt.Contains, "If the email exists")

	assertNoVerificationEmail(t, emailSvc)
}

func TestHandleResendVerification_HappyPathDeletesOldTokensAndIssuesFreshOne(t *testing.T) {
	c := qt.New(t)

	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "pending-user"},
			TenantID: testTenantID,
		},
		Email:    "pending@example.com",
		Name:     "Pending",
		IsActive: false,
	}
	verReg := memory.NewEmailVerificationRegistry()
	oldA, err := verReg.Create(context.Background(), models.EmailVerification{
		UserID:    user.ID,
		TenantID:  testTenantID,
		Email:     user.Email,
		Token:     "old-token-a",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)
	oldB, err := verReg.Create(context.Background(), models.EmailVerification{
		UserID:    user.ID,
		TenantID:  testTenantID,
		Email:     user.Email,
		Token:     "old-token-b",
		ExpiresAt: time.Now().Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)

	emailSvc := &blockingEmailService{verificationCh: make(chan asyncEmailObservation, 1)}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}},
		VerificationRegistry: verReg,
		EmailService:         emailSvc,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	w := postResendVerification(c, r, map[string]string{"email": user.Email})
	c.Assert(w.Code, qt.Equals, http.StatusOK)
	c.Assert(w.Body.String(), qt.Contains, "If the email exists")

	// Wait for the async send to complete. sendVerification deletes the old
	// rows and creates the fresh one synchronously *before* spawning the
	// email goroutine, so once the goroutine has fired we know the registry
	// state is settled.
	select {
	case <-emailSvc.verificationCh:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected a verification email to be dispatched on resend happy path")
	}

	current, err := verReg.GetByUserID(context.Background(), user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(current, qt.HasLen, 1, qt.Commentf("only the freshly issued token should remain"))
	c.Assert(current[0].Token, qt.Not(qt.Equals), oldA.Token)
	c.Assert(current[0].Token, qt.Not(qt.Equals), oldB.Token)

	_, err = verReg.GetByToken(context.Background(), oldA.Token)
	c.Assert(err, qt.IsNotNil, qt.Commentf("old token A must be deleted"))
	_, err = verReg.GetByToken(context.Background(), oldB.Token)
	c.Assert(err, qt.IsNotNil, qt.Commentf("old token B must be deleted"))
}
