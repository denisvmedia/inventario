package apiserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/mail"
	"sort"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/services"
)

// feedbackMaxMessageBytes caps the free-form message payload. Five
// kilobytes matches the issue brief ("a textarea is enough; this isn't
// a bug report attachment surface") and stays well under the 4 KB
// rate-limit body cap used by PasswordResetRateLimitMiddleware — the
// feedback middleware reads the body via the handler's `json.Decoder`
// instead of peeking it in middleware, so this is the real bound.
const feedbackMaxMessageBytes = 5 * 1024

// feedbackMaxDiagnosticsEntries caps the number of diagnostics rows the
// FE may attach. The issue lists ~9 standard fields plus the optional
// "last 10 console errors" tail, so 32 leaves headroom for future
// additions without letting a misbehaving client exhaust the inbox
// owner's screen.
const feedbackMaxDiagnosticsEntries = 32

// feedbackMaxDiagnosticsValueBytes is the per-line cap. A URL plus a
// long UA string fits comfortably under this without padding.
const feedbackMaxDiagnosticsValueBytes = 1024

// feedbackMaxRequestBodyBytes bounds the raw POST body. Sized to
// comfortably hold a max-length message + the diagnostics cap +
// reasonable JSON / quoting overhead, so a legitimate worst-case
// submission never hits a confusing 413 before field-level
// validation runs.
const feedbackMaxRequestBodyBytes = feedbackMaxMessageBytes +
	(feedbackMaxDiagnosticsEntries * feedbackMaxDiagnosticsValueBytes) +
	16*1024

// FeedbackParams wires the dependencies of the /api/v1/feedback route.
//
// SupportEmail is the operator-configured destination address (issue
// #1387 §Backend). An empty value disables the endpoint — the handler
// returns 503 so the FE can show a "feedback is not configured" hint
// rather than silently accepting and dropping the submission.
type FeedbackParams struct {
	EmailService services.EmailService
	SupportEmail string
}

type feedbackAPI struct {
	emailService services.EmailService
	supportEmail string
}

// FeedbackRequest is the wire shape of POST /feedback. `type` and
// `message` are required; the swag `validate:"required"` +
// `enums:"…"` tags propagate that to the generated OpenAPI schema so
// codegen'd clients enforce the same constraints as the handler.
type FeedbackRequest struct {
	// Type is one of "feedback" | "bug" | "feature" | "question". The
	// FE renders these as radio chips; the backend uses the value
	// verbatim in the email subject and body. Unknown values are
	// rejected with 400.
	Type string `json:"type" validate:"required" enums:"feedback,bug,feature,question"`
	// Message is the free-form body. Required, trimmed, capped at
	// feedbackMaxMessageBytes.
	Message string `json:"message" validate:"required"`
	// ReplyToEmail is optional. When set the value goes into the email
	// body and (in the async sender) into the Reply-To header. Empty
	// means "the submitter declined to share a reply-to address".
	ReplyToEmail string `json:"reply_to_email,omitempty"`
	// Diagnostics is the FE-controlled set of debug attributes. Keys
	// are surfaced verbatim — the BE does not whitelist or rewrite
	// them — but the BE caps per-line size and the number of entries.
	Diagnostics map[string]string `json:"diagnostics,omitempty"`
}

// FeedbackResponse is the success envelope. The status field exists so
// the FE can pin a specific success path in the toast text without
// re-parsing the HTTP status.
type FeedbackResponse struct {
	Status string `json:"status"`
}

// validFeedbackTypes is the allow-list checked by the handler. Keep in
// sync with the FE radio chips — adding a new option without updating
// both ends results in a 400 the user can't action.
var validFeedbackTypes = map[string]string{
	"feedback": "Feedback",
	"bug":      "Bug",
	"feature":  "Feature request",
	"question": "Question",
}

// Feedback registers the /api/v1/feedback route group. The caller must
// apply userMiddlewares before this route group — every handler reads
// appctx.UserFromContext and trusts it to be non-nil.
//
// The per-user rate limit is applied here rather than at the apiserver
// wiring level so the route group stays self-contained: feedback is
// the only sub-route, and the limiter is the only middleware specific
// to it.
func Feedback(params FeedbackParams, limiter services.AuthRateLimiter) func(r chi.Router) {
	api := &feedbackAPI{
		emailService: params.EmailService,
		supportEmail: strings.TrimSpace(params.SupportEmail),
	}
	return func(r chi.Router) {
		r.With(FeedbackRateLimitMiddleware(limiter)).Post("/", api.submit)
	}
}

// submit forwards an authenticated user's feedback submission to the
// configured support address.
// @Summary Submit feedback
// @Description Forward an authenticated user's in-app feedback / bug report / feature request to the operator-configured support inbox. Per-user rate-limited (5/hour).
// @Tags feedback
// @Accept json
// @Produce json
// @Param body body FeedbackRequest true "Feedback payload"
// @Success 202 {object} FeedbackResponse "Accepted"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Failure 429 {string} string "Too Many Requests"
// @Failure 503 {string} string "Feedback not configured"
// @Router /feedback [post]
func (api *feedbackAPI) submit(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	if api.supportEmail == "" {
		// Endpoint mounted but no destination address configured.
		// The FE renders a static help message in that case (the
		// SettingsPage Help section already has a mailto fallback);
		// we surface a 503 so the toast can be specific.
		slog.Warn("Feedback submission rejected: SUPPORT_EMAIL not configured", "user_id", user.ID)
		http.Error(w, "Feedback is not configured on this deployment", http.StatusServiceUnavailable)
		return
	}

	// Limit body to a few tens of KB. The handler reads the body itself
	// (FeedbackRateLimitMiddleware does not peek), so this is the real
	// upper bound for the request payload — sized to hold a max-length
	// message + the diagnostics cap + JSON overhead so a legitimate
	// worst-case payload never hits 413 before field validation runs.
	r.Body = http.MaxBytesReader(w, r.Body, feedbackMaxRequestBodyBytes)
	defer func() { _ = r.Body.Close() }()

	var req FeedbackRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "Invalid JSON body", http.StatusBadRequest)
		return
	}

	typeLabel, ok := validFeedbackTypes[strings.ToLower(strings.TrimSpace(req.Type))]
	if !ok {
		http.Error(w, "Invalid feedback type. Expected one of: feedback, bug, feature, question", http.StatusBadRequest)
		return
	}

	message := strings.TrimSpace(req.Message)
	if message == "" {
		http.Error(w, "Message is required", http.StatusBadRequest)
		return
	}
	if len(message) > feedbackMaxMessageBytes {
		http.Error(w, fmt.Sprintf("Message exceeds %d byte limit", feedbackMaxMessageBytes), http.StatusBadRequest)
		return
	}

	replyTo := strings.TrimSpace(req.ReplyToEmail)
	// The async sender stuffs this value into the outbound `Reply-To`
	// header, so we must reject anything that could enable header
	// injection (CR/LF) or render the resulting envelope ambiguous
	// (multiple "@", malformed local part). Use net/mail.ParseAddress
	// instead of a hand-rolled regex — it covers the full RFC 5322
	// surface and matches the rest of the email layer.
	if replyTo != "" {
		if strings.ContainsAny(replyTo, "\r\n") {
			http.Error(w, "reply_to_email must not contain newlines", http.StatusBadRequest)
			return
		}
		if _, err := mail.ParseAddress(replyTo); err != nil {
			http.Error(w, "reply_to_email must be a valid email address", http.StatusBadRequest)
			return
		}
	}

	if len(req.Diagnostics) > feedbackMaxDiagnosticsEntries {
		http.Error(w, fmt.Sprintf("diagnostics may contain at most %d entries", feedbackMaxDiagnosticsEntries), http.StatusBadRequest)
		return
	}

	diagnostics := formatFeedbackDiagnostics(req.Diagnostics)

	if err := api.emailService.SendFeedbackEmail(
		r.Context(),
		api.supportEmail,
		user.Email,
		strings.TrimSpace(user.Name),
		user.ID,
		typeLabel,
		message,
		replyTo,
		diagnostics,
	); err != nil {
		slog.Error("Failed to enqueue feedback email", "error", err, "user_id", user.ID, "type", typeLabel)
		http.Error(w, "Failed to send feedback", http.StatusInternalServerError)
		return
	}

	slog.Info("Feedback submitted",
		"user_id", user.ID,
		"type", typeLabel,
		"message_chars", len(message),
		"diagnostics_count", len(diagnostics),
		"reply_to_provided", replyTo != "",
	)

	writeJSON(w, http.StatusAccepted, FeedbackResponse{Status: "accepted"})
}

// formatFeedbackDiagnostics produces a stable, sorted "label: value"
// slice suitable for the email template. Keys are sorted so the email
// reads consistently across submissions; values are trimmed and
// truncated so a misbehaving client cannot dump arbitrary payloads
// into the inbox.
//
// Empty keys are dropped; whitespace-only values are dropped along
// with their key (no point rendering "URL: " with no value).
func formatFeedbackDiagnostics(in map[string]string) []string {
	if len(in) == 0 {
		return nil
	}
	keys := make([]string, 0, len(in))
	for k := range in {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		val := strings.TrimSpace(in[k])
		if val == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]string, 0, len(keys))
	for _, k := range keys {
		val := strings.TrimSpace(in[k])
		if len(val) > feedbackMaxDiagnosticsValueBytes {
			val = val[:feedbackMaxDiagnosticsValueBytes] + "…"
		}
		out = append(out, fmt.Sprintf("%s: %s", strings.TrimSpace(k), val))
	}
	return out
}
