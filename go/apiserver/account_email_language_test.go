package apiserver_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// emailLanguageRequestID is the X-Request-ID the tests send, so each email can
// be checked for the id of the request that caused it.
const emailLanguageRequestID = "account-email-language-test"

// accountEmailSent is what the email queue would have received: which email,
// the language it would render in, and the request id its log lines carry.
type accountEmailSent struct {
	kind      string
	language  string
	requestID string
}

// accountEmailRecorder captures the context each account email is sent with.
type accountEmailRecorder struct {
	// Embedded so the emails these tests do not send need no method here.
	services.StubEmailService
	sent chan accountEmailSent
}

func (m *accountEmailRecorder) record(ctx context.Context, kind string) error {
	m.sent <- accountEmailSent{
		kind:      kind,
		language:  appctx.EmailLanguageFromContext(ctx),
		requestID: appctx.RequestIDFromContext(ctx),
	}
	return nil
}

func (m *accountEmailRecorder) SendPasswordChangedEmail(ctx context.Context, _, _ string, _ time.Time) error {
	return m.record(ctx, "password_changed")
}

func (m *accountEmailRecorder) SendWelcomeEmail(ctx context.Context, _, _ string) error {
	return m.record(ctx, "welcome")
}

func (m *accountEmailRecorder) SendPasswordResetEmail(ctx context.Context, _, _, _ string) error {
	return m.record(ctx, "password_reset")
}

func (m *accountEmailRecorder) SendMagicLinkEmail(ctx context.Context, _, _, _ string) error {
	return m.record(ctx, "magic_link")
}

// next waits for the one email the request sends.
func (m *accountEmailRecorder) next(c *qt.C) accountEmailSent {
	c.Helper()
	select {
	case got := <-m.sent:
		return got
	case <-time.After(2 * time.Second):
		c.Fatal("no email was sent")
		return accountEmailSent{}
	}
}

// newAccountEmailEnv is a full API server whose test user chose Czech in
// their settings.
func newAccountEmailEnv(c *qt.C) (*apiTokenEnv, *accountEmailRecorder) {
	c.Helper()
	emailSvc := &accountEmailRecorder{sent: make(chan accountEmailSent, 4)}
	env := newAPITokenEnv(c, func(p *apiserver.Params) {
		p.EmailService = emailSvc
		p.PublicURL = "https://app.example.com"
		p.MagicLinkLoginEnabled = true
	})
	ctx := appctx.WithUser(context.Background(), env.user)
	settings := must.Must(env.params.FactorySet.SettingsRegistryFactory.CreateUserRegistry(ctx))
	c.Assert(settings.Save(ctx, models.SettingsObject{AppearanceLanguage: new("cs")}), qt.IsNil)
	return env, emailSvc
}

func publicRequest(method, path string, body any) apiTokenRequest {
	return apiTokenRequest{method: method, path: path, body: body, headers: map[string][]string{
		apiserver.RequestIDHeader: {emailLanguageRequestID},
	}}
}

func TestAccountEmails_PasswordChangedAfterAChangeIsInTheUsersLanguage(t *testing.T) {
	c := qt.New(t)
	env, emailSvc := newAccountEmailEnv(c)

	rr := env.do(apiTokenRequest{
		method: http.MethodPost,
		path:   "/api/v1/auth/change-password",
		body: apiserver.ChangePasswordRequest{
			CurrentPassword: apiTokenTestPassword,
			NewPassword:     "AnotherPassword456",
		},
		headers: map[string][]string{
			"Authorization":           {"Bearer " + createTestJWTToken(env.user.ID)},
			"X-CSRF-Token":            {must.Must(env.csrf.GenerateToken(context.Background(), env.user.ID))},
			apiserver.RequestIDHeader: {emailLanguageRequestID},
		},
	})
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	c.Assert(emailSvc.next(c), qt.Equals, accountEmailSent{
		kind:      "password_changed",
		language:  "cs",
		requestID: emailLanguageRequestID,
	})
}

func TestAccountEmails_PasswordChangedAfterAResetIsInTheUsersLanguage(t *testing.T) {
	c := qt.New(t)
	env, emailSvc := newAccountEmailEnv(c)

	rr := env.do(publicRequest(http.MethodPost, "/api/v1/reset-password", apiserver.ResetPasswordRequest{
		Token:       env.addPasswordReset(),
		NewPassword: "AnotherPassword456",
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	c.Assert(emailSvc.next(c), qt.Equals, accountEmailSent{
		kind:      "password_changed",
		language:  "cs",
		requestID: emailLanguageRequestID,
	})
}

func TestAccountEmails_WelcomeIsInTheUsersLanguage(t *testing.T) {
	c := qt.New(t)
	env, emailSvc := newAccountEmailEnv(c)
	verification := must.Must(env.params.FactorySet.EmailVerificationRegistry.Create(context.Background(), models.EmailVerification{
		UserID:    env.user.ID,
		TenantID:  env.user.TenantID,
		Email:     env.user.Email,
		Token:     "account-email-language-verification",
		ExpiresAt: time.Now().Add(time.Hour),
	}))

	rr := env.do(publicRequest(http.MethodGet, "/api/v1/verify-email?"+url.Values{"token": {verification.Token}}.Encode(), nil))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	c.Assert(emailSvc.next(c), qt.Equals, accountEmailSent{
		kind:      "welcome",
		language:  "cs",
		requestID: emailLanguageRequestID,
	})
}

func TestAccountEmails_PasswordResetIsInTheUsersLanguage(t *testing.T) {
	c := qt.New(t)
	env, emailSvc := newAccountEmailEnv(c)

	rr := env.do(publicRequest(http.MethodPost, "/api/v1/forgot-password", apiserver.ForgotPasswordRequest{Email: env.user.Email}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	c.Assert(emailSvc.next(c), qt.Equals, accountEmailSent{
		kind:      "password_reset",
		language:  "cs",
		requestID: emailLanguageRequestID,
	})
}

func TestAccountEmails_MagicLinkIsInTheUsersLanguage(t *testing.T) {
	c := qt.New(t)
	env, emailSvc := newAccountEmailEnv(c)

	rr := env.do(publicRequest(http.MethodPost, "/api/v1/auth/magic-link/request", map[string]string{"email": env.user.Email}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf(rr.Body.String()))

	c.Assert(emailSvc.next(c), qt.Equals, accountEmailSent{
		kind:      "magic_link",
		language:  "cs",
		requestID: emailLanguageRequestID,
	})
}
