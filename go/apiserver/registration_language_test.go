package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// registerWithLanguage registers a new account in open mode with the given
// Accept-Language header. It returns what the verification email was sent
// with and the language setting the account was given.
func registerWithLanguage(c *qt.C, acceptLanguage string) (accountEmailSent, *string) {
	c.Helper()
	userReg := &registrationUserRegistry{mockUserRegistryForAuth: &mockUserRegistryForAuth{users: map[string]*models.User{}}}
	settings := memory.NewSettingsRegistryFactory()
	emailSvc := &accountEmailRecorder{sent: make(chan accountEmailSent, 1)}
	r := newRegistrationRouter(apiserver.RegistrationParams{
		UserRegistry:            userReg,
		VerificationRegistry:    memory.NewEmailVerificationRegistry(),
		EmailService:            emailSvc,
		RateLimiter:             services.NewInMemoryAuthRateLimiter(),
		SettingsRegistryFactory: settings,
	}, models.RegistrationModeOpen)

	body := must.Must(json.Marshal(apiserver.RegisterRequest{Email: "new@example.com", Name: "New", Password: "Password123"}))
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept-Language", acceptLanguage)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf(w.Body.String()))

	sent := emailSvc.next(c)
	c.Assert(userReg.users, qt.HasLen, 1)
	var created *models.User
	for _, u := range userReg.users {
		created = u
	}
	ctx := appctx.WithUser(context.Background(), created)
	stored := must.Must(must.Must(settings.CreateUserRegistry(ctx)).Get(ctx))
	return sent, stored.AppearanceLanguage
}

func TestHandleRegister_VerificationEmailIsInTheLanguageTheAccountRegisteredIn(t *testing.T) {
	c := qt.New(t)

	sent, stored := registerWithLanguage(c, "de-DE, ru;q=0.8, en;q=0.5")

	c.Assert(sent, qt.Equals, accountEmailSent{kind: "verification", language: "ru"})
	c.Assert(stored, qt.DeepEquals, new("ru"))
}

func TestHandleRegister_NoSupportedLanguageStoresNone(t *testing.T) {
	c := qt.New(t)

	sent, stored := registerWithLanguage(c, "de-DE, fr;q=0.8")

	c.Assert(sent, qt.Equals, accountEmailSent{kind: "verification"})
	c.Assert(stored, qt.IsNil)
}
