package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

type asyncEmailObservation struct {
	tenantID    string
	ctxErr      error
	hasDeadline bool
	deadlineIn  time.Duration
}

type blockingEmailService struct {
	release         <-chan struct{}
	passwordResetCh chan asyncEmailObservation
	verificationCh  chan asyncEmailObservation
	passwordChanged chan struct{}
	welcome         chan struct{}
	// verificationCalls counts every SendVerificationEmail invocation so
	// absence-of-send assertions can check an exact zero atomically rather
	// than racing a channel receive against a timeout.
	verificationCalls atomic.Int32
}

func (m *blockingEmailService) SendVerificationEmail(ctx context.Context, _ string, _ string, _ string) error {
	m.verificationCalls.Add(1)
	if m.release != nil {
		<-m.release
	}
	if m.verificationCh != nil {
		deadline, hasDeadline := ctx.Deadline()
		deadlineIn := time.Duration(0)
		if hasDeadline {
			deadlineIn = time.Until(deadline)
		}
		m.verificationCh <- asyncEmailObservation{
			tenantID:    apiserver.TenantIDFromContext(ctx),
			ctxErr:      ctx.Err(),
			hasDeadline: hasDeadline,
			deadlineIn:  deadlineIn,
		}
	}
	return nil
}

func (m *blockingEmailService) SendPasswordResetEmail(ctx context.Context, _ string, _ string, _ string) error {
	if m.release != nil {
		<-m.release
	}
	if m.passwordResetCh != nil {
		deadline, hasDeadline := ctx.Deadline()
		deadlineIn := time.Duration(0)
		if hasDeadline {
			deadlineIn = time.Until(deadline)
		}
		m.passwordResetCh <- asyncEmailObservation{
			tenantID:    apiserver.TenantIDFromContext(ctx),
			ctxErr:      ctx.Err(),
			hasDeadline: hasDeadline,
			deadlineIn:  deadlineIn,
		}
	}
	return nil
}

func (m *blockingEmailService) SendPasswordChangedEmail(_ context.Context, _ string, _ string, _ time.Time) error {
	if m.passwordChanged != nil {
		m.passwordChanged <- struct{}{}
	}
	return nil
}

func (m *blockingEmailService) SendWelcomeEmail(_ context.Context, _ string, _ string) error {
	if m.welcome != nil {
		m.welcome <- struct{}{}
	}
	return nil
}

type registrationUserRegistry struct {
	*mockUserRegistryForAuth
}

func (m *registrationUserRegistry) Create(_ context.Context, user models.User) (*models.User, error) {
	if m.users == nil {
		m.users = map[string]*models.User{}
	}
	m.users[user.ID] = new(user)
	return m.users[user.ID], nil
}

func newRegistrationRouter(params apiserver.RegistrationParams, mode models.RegistrationMode) chi.Router {
	tenant := &models.Tenant{
		Status:           models.TenantStatusActive,
		RegistrationMode: mode,
	}
	tenant.ID = testTenantID
	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			next.ServeHTTP(w, req.WithContext(apiserver.WithTenant(req.Context(), tenant)))
		})
	})
	r.Group(apiserver.Registration(params))
	return r
}

func TestHandleForgotPassword_EmailIsSentAfterRequestCancellation(t *testing.T) {
	c := qt.New(t)

	user := makePasswordResetUser()
	release := make(chan struct{})
	emailSvc := &blockingEmailService{
		release:         release,
		passwordResetCh: make(chan asyncEmailObservation, 1),
	}
	r := newPasswordResetRouter(apiserver.PasswordResetParams{
		UserRegistry:          &mockUserRegistryForAuth{users: map[string]*models.User{user.ID: user}},
		PasswordResetRegistry: memory.NewPasswordResetRegistry(),
		EmailService:          emailSvc,
	})

	body, err := json.Marshal(map[string]string{"email": user.Email})
	c.Assert(err, qt.IsNil)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/forgot-password", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	cancel()
	close(release)

	select {
	case obs := <-emailSvc.passwordResetCh:
		c.Assert(obs.tenantID, qt.Equals, testTenantID)
		c.Assert(obs.ctxErr, qt.IsNil)
		c.Assert(obs.hasDeadline, qt.IsTrue)
		c.Assert(obs.deadlineIn > 20*time.Second, qt.IsTrue)
		c.Assert(obs.deadlineIn <= 31*time.Second, qt.IsTrue)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected password reset email to be sent")
	}
}

func TestHandleRegister_VerificationEmailIsSentAfterRequestCancellation(t *testing.T) {
	c := qt.New(t)

	release := make(chan struct{})
	emailSvc := &blockingEmailService{
		release:        release,
		verificationCh: make(chan asyncEmailObservation, 1),
	}
	userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:         userReg,
		VerificationRegistry: memory.NewEmailVerificationRegistry(),
		EmailService:         emailSvc,
		RateLimiter:          services.NewInMemoryAuthRateLimiter(),
	}, models.RegistrationModeOpen)

	body, err := json.Marshal(map[string]string{
		"email":    "new-user@example.com",
		"name":     "New User",
		"password": "Password123",
	})
	c.Assert(err, qt.IsNil)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body)).WithContext(ctx)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK)

	cancel()
	close(release)

	select {
	case obs := <-emailSvc.verificationCh:
		c.Assert(obs.tenantID, qt.Equals, testTenantID)
		c.Assert(obs.ctxErr, qt.IsNil)
		c.Assert(obs.hasDeadline, qt.IsTrue)
		c.Assert(obs.deadlineIn > 20*time.Second, qt.IsTrue)
		c.Assert(obs.deadlineIn <= 31*time.Second, qt.IsTrue)
	case <-time.After(500 * time.Millisecond):
		t.Fatal("expected verification email to be sent")
	}
}
