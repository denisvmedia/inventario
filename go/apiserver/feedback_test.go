package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services"
)

// capturingFeedbackEmailService records the most recent SendFeedbackEmail
// call so the test can assert the wire-level shape passed to the email
// layer. Other EmailService methods are no-ops — the feedback handler
// only calls SendFeedbackEmail.
type capturingFeedbackEmailService struct {
	mu               sync.Mutex
	calls            int
	lastTo           string
	lastFromEmail    string
	lastFromName     string
	lastFromUserID   string
	lastFeedbackType string
	lastMessage      string
	lastReplyTo      string
	lastDiagnostics  []string
}

func (c *capturingFeedbackEmailService) SendFeedbackEmail(_ context.Context, to, fromEmail, fromName, fromUserID, feedbackType, message, replyToEmail string, diagnosticsLines []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	c.lastTo = to
	c.lastFromEmail = fromEmail
	c.lastFromName = fromName
	c.lastFromUserID = fromUserID
	c.lastFeedbackType = feedbackType
	c.lastMessage = message
	c.lastReplyTo = replyToEmail
	c.lastDiagnostics = append([]string(nil), diagnosticsLines...)
	return nil
}

func (*capturingFeedbackEmailService) SendVerificationEmail(_ context.Context, _, _, _ string) error {
	return nil
}
func (*capturingFeedbackEmailService) SendPasswordResetEmail(_ context.Context, _, _, _ string) error {
	return nil
}
func (*capturingFeedbackEmailService) SendPasswordChangedEmail(_ context.Context, _, _ string, _ time.Time) error {
	return nil
}
func (*capturingFeedbackEmailService) SendWelcomeEmail(_ context.Context, _, _ string) error {
	return nil
}
func (*capturingFeedbackEmailService) SendWarrantyReminderEmail(_ context.Context, _, _, _, _, _ string, _ int) error {
	return nil
}
func (*capturingFeedbackEmailService) SendGroupInviteEmail(_ context.Context, _, _, _, _, _ string, _ time.Time) error {
	return nil
}
func (*capturingFeedbackEmailService) SendStorageQuotaWarningEmail(_ context.Context, _, _, _ string, _, _ int, _, _ string, _ []string, _, _ string) error {
	return nil
}
func (*capturingFeedbackEmailService) SendLoanReminderEmail(_ context.Context, _, _, _, _, _, _, _, _ string, _ int) error {
	return nil
}
func (*capturingFeedbackEmailService) SendMaintenanceReminderEmail(_ context.Context, _, _, _, _, _, _ string, _ int) error {
	return nil
}

// newFeedbackTestRouter mounts the Feedback route group with a stubbed
// user-context middleware so the test exercises the same handler tree
// production uses — only the auth middleware is swapped out.
func newFeedbackTestRouter(user *models.User, params apiserver.FeedbackParams, limiter services.AuthRateLimiter) http.Handler {
	r := chi.NewRouter()
	r.Route("/feedback", func(sub chi.Router) {
		sub.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				if user != nil {
					ctx := appctx.WithUser(req.Context(), user)
					next.ServeHTTP(w, req.WithContext(ctx))
					return
				}
				next.ServeHTTP(w, req)
			})
		})
		apiserver.Feedback(params, limiter)(sub)
	})
	return r
}

func newFeedbackTestUser() *models.User {
	return &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
		Email:    "alex@example.com",
		Name:     "Alex Submitter",
		IsActive: true,
	}
}

// TestFeedback_HappyPath asserts the documented success contract: 202
// Accepted, a stable JSON envelope, and the email service called once
// with all the submitter context populated. Diagnostics are sorted by
// key so the inbox owner sees a stable line ordering across submissions.
func TestFeedback_HappyPath(t *testing.T) {
	c := qt.New(t)
	email := &capturingFeedbackEmailService{}
	user := newFeedbackTestUser()
	handler := newFeedbackTestRouter(user, apiserver.FeedbackParams{
		EmailService: email,
		SupportEmail: "support@example.test",
	}, services.NewNoOpAuthRateLimiter())

	body := map[string]any{
		"type":           "bug",
		"message":        "Login page bounces me back after 2FA.",
		"reply_to_email": "alex@example.com",
		"diagnostics": map[string]string{
			"url":     "https://app.example.test/login",
			"ua":      "Mozilla/5.0 ...",
			"version": "0.42.0",
		},
	}
	raw, err := json.Marshal(body)
	c.Assert(err, qt.IsNil)
	req := httptest.NewRequest(http.MethodPost, "/feedback", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusAccepted)
	c.Assert(rr.Header().Get("Content-Type"), qt.Equals, "application/json")

	var resp apiserver.FeedbackResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Status, qt.Equals, "accepted")

	c.Assert(email.calls, qt.Equals, 1)
	c.Assert(email.lastTo, qt.Equals, "support@example.test")
	c.Assert(email.lastFromEmail, qt.Equals, "alex@example.com")
	c.Assert(email.lastFromName, qt.Equals, "Alex Submitter")
	c.Assert(email.lastFromUserID, qt.Equals, "user-1")
	c.Assert(email.lastFeedbackType, qt.Equals, "Bug")
	c.Assert(email.lastMessage, qt.Equals, "Login page bounces me back after 2FA.")
	c.Assert(email.lastReplyTo, qt.Equals, "alex@example.com")
	// Sorted by key — the FE may submit in any order, the BE renders
	// in stable alphabetical order so the inbox owner can scan it.
	c.Assert(email.lastDiagnostics, qt.DeepEquals, []string{
		"ua: Mozilla/5.0 ...",
		"url: https://app.example.test/login",
		"version: 0.42.0",
	})
}

// TestFeedback_NoSupportEmail asserts the operator-misconfiguration
// path: the route stays mounted (so the FE has a stable URL), but
// without SUPPORT_EMAIL the handler responds 503 and never calls the
// email service. The FE relies on this status to surface the static
// mailto fallback in the toast.
func TestFeedback_NoSupportEmail(t *testing.T) {
	c := qt.New(t)
	email := &capturingFeedbackEmailService{}
	user := newFeedbackTestUser()
	handler := newFeedbackTestRouter(user, apiserver.FeedbackParams{
		EmailService: email,
		SupportEmail: "",
	}, services.NewNoOpAuthRateLimiter())

	body := strings.NewReader(`{"type":"bug","message":"x"}`)
	req := httptest.NewRequest(http.MethodPost, "/feedback", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusServiceUnavailable)
	c.Assert(email.calls, qt.Equals, 0)
}

// TestFeedback_Unauthenticated guards against the regression where the
// outer middleware stack is misconfigured and a request lands on the
// feedback handler without a user. The handler MUST refuse — defence in
// depth on top of JWTMiddleware.
func TestFeedback_Unauthenticated(t *testing.T) {
	c := qt.New(t)
	email := &capturingFeedbackEmailService{}
	handler := newFeedbackTestRouter(nil, apiserver.FeedbackParams{
		EmailService: email,
		SupportEmail: "support@example.test",
	}, services.NewNoOpAuthRateLimiter())

	body := strings.NewReader(`{"type":"bug","message":"x"}`)
	req := httptest.NewRequest(http.MethodPost, "/feedback", body)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
	c.Assert(email.calls, qt.Equals, 0)
}

// TestFeedback_Validation walks the documented 400 surface: unknown
// type, missing message, oversize message, and obviously-broken
// reply_to_email all fail before the email service is touched.
func TestFeedback_Validation(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "unknown_type", body: `{"type":"bogus","message":"x"}`},
		{name: "missing_message", body: `{"type":"bug","message":"   "}`},
		{
			name: "message_too_long",
			body: fmt.Sprintf(`{"type":"bug","message":%q}`, strings.Repeat("a", 5*1024+1)),
		},
		{name: "bad_reply_to", body: `{"type":"bug","message":"x","reply_to_email":"not-an-email"}`},
		{name: "malformed_json", body: `not-json`},
		{
			name: "too_many_diagnostics_lines",
			body: func() string {
				// 33 entries — one over the documented cap of 32. Distinct
				// keys are required because Diagnostics is a map.
				diag := make(map[string]string, 33)
				for i := range 33 {
					diag[fmt.Sprintf("k%02d", i)] = "v"
				}
				out, err := json.Marshal(struct {
					Type        string            `json:"type"`
					Message     string            `json:"message"`
					Diagnostics map[string]string `json:"diagnostics"`
				}{Type: "bug", Message: "x", Diagnostics: diag})
				if err != nil {
					panic(err)
				}
				return string(out)
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			email := &capturingFeedbackEmailService{}
			user := newFeedbackTestUser()
			handler := newFeedbackTestRouter(user, apiserver.FeedbackParams{
				EmailService: email,
				SupportEmail: "support@example.test",
			}, services.NewNoOpAuthRateLimiter())

			req := httptest.NewRequest(http.MethodPost, "/feedback", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
			c.Assert(email.calls, qt.Equals, 0)
		})
	}
}

// TestFeedback_RateLimit pins the documented per-user limit: five
// submissions in the window succeed, the sixth gets 429 with a
// Retry-After header so the FE can render a "try again in N minutes"
// toast.
func TestFeedback_RateLimit(t *testing.T) {
	c := qt.New(t)
	email := &capturingFeedbackEmailService{}
	user := newFeedbackTestUser()
	// Use the in-memory limiter explicitly so we exercise the same
	// sliding-window code path production uses (the no-op limiter
	// never blocks and would mask a regression).
	limiter := services.NewInMemoryAuthRateLimiter()
	handler := newFeedbackTestRouter(user, apiserver.FeedbackParams{
		EmailService: email,
		SupportEmail: "support@example.test",
	}, limiter)

	body := `{"type":"feedback","message":"hello"}`

	// First five submissions are accepted.
	for i := range 5 {
		req := httptest.NewRequest(http.MethodPost, "/feedback", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		c.Assert(rr.Code, qt.Equals, http.StatusAccepted, qt.Commentf("attempt %d", i+1))
	}

	// Sixth attempt is rejected.
	req := httptest.NewRequest(http.MethodPost, "/feedback", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusTooManyRequests)
	c.Assert(rr.Header().Get("Retry-After"), qt.Not(qt.Equals), "")
	c.Assert(rr.Header().Get("X-RateLimit-Limit"), qt.Equals, "5")
	c.Assert(rr.Header().Get("X-RateLimit-Remaining"), qt.Equals, "0")
	// Only the first five made it through to the email layer.
	c.Assert(email.calls, qt.Equals, 5)
}
