package services

import (
	"context"
	"encoding/json"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// AuthEvent carries the fields LogAuth persists into an audit row. The
// struct shape keeps optional fields nameable so callers can omit
// (e.g. anonymous login attempts pass UserID=nil) without threading a
// long positional list through every audit log call.
//
// Action is the full event name (e.g. "login", "logout", "password_change").
// Success is explicit so failures with no underlying error message
// (e.g. "user not found") remain expressible as Success=false, ErrMsg=nil.
type AuthEvent struct {
	Action   string
	UserID   *string
	TenantID *string
	Success  bool
	Request  *http.Request
	ErrMsg   *string
}

// AdminEvent carries the fields LogAdmin persists into an audit row.
// The struct-shape lets callers omit optional fields by name (rather
// than threading 9 positional arguments through several layers) and
// makes "failure without an error message" — e.g. the last-admin guard
// rejecting a revoke — naturally expressible: Success=false, ErrMsg=nil.
//
// Action is the full event name (e.g. "admin.grant_system_admin"). It
// is required; everything else is optional and zero-valued by default.
//
// ImpersonatedBy is intentionally NOT a field on the struct: the helper
// fills it from the request's JWT claims (`imp` / `impersonated_by`),
// so the call site never needs to know whether an impersonation session
// is active.
//
// Reason / Forced / Extra carry the action-specific breadcrumb that
// LogAdmin persists alongside the row. The audit_logs schema doesn't
// have a generic context column today (#1747) — stuffing the breadcrumb
// into user_agent as a JSON blob with the real UA preserved under a
// sub-key keeps the row self-describing without a schema bump. Mirrors
// insertCurrencyMigrationAuditLog. When Reason/Forced/Extra are all
// zero-valued the helper falls back to writing the raw User-Agent
// header so existing callers (grant/revoke) continue to write a plain
// UA string, not a noisy {"ua":"..."} blob.
type AdminEvent struct {
	Action      string
	ActorID     *string
	TenantID    *string
	SubjectType *string
	SubjectID   *string
	Success     bool
	Request     *http.Request
	ErrMsg      *string
	// Reason is the operator-supplied free-form text accompanying a
	// state-changing admin action (e.g. the body field on block/unblock).
	Reason string
	// Forced flags a sensitive admin action that bypassed a guard via an
	// explicit `force: true` body flag (e.g. blocking another system
	// admin). The audit consumer can pivot on this in queries even
	// when Action is identical to the non-forced variant.
	Forced bool
	// Extra carries action-specific breadcrumb fields. Merged into the
	// user_agent JSON blob alongside reason / forced / ua. Keep the
	// values small and JSON-serialisable — this column is text, not
	// jsonb, and oversized blobs hurt audit log scans.
	Extra map[string]any
}

// AuditLogger is the interface for logging security-relevant events.
type AuditLogger interface {
	// LogAuth records an authentication or authorization event (login, logout, password change, etc.).
	// The call is best-effort: errors are logged but do not propagate to callers.
	// See AuthEvent for the struct shape.
	LogAuth(ctx context.Context, ev AuthEvent)

	// LogAdmin records a platform-administrative event (grant/revoke
	// system-admin, future impersonation start/end). The call is
	// best-effort like LogAuth: errors are logged but do not propagate.
	// See AdminEvent for the struct shape — Success is explicit so
	// callers can record a failed admin action (e.g. last-admin guard
	// rejecting a revoke) without inventing an error message.
	LogAdmin(ctx context.Context, ev AdminEvent)
}

// AuditService persists security-relevant events via the AuditLogRegistry.
type AuditService struct {
	auditRegistry registry.AuditLogRegistry
}

// NewAuditService creates a new AuditService backed by the given registry.
func NewAuditService(auditRegistry registry.AuditLogRegistry) *AuditService {
	return &AuditService{auditRegistry: auditRegistry}
}

// LogAuth records an authentication event.
// It is intentionally best-effort: if persisting the log entry fails the error is
// logged with slog but not returned to the caller so that auth flows are never
// blocked by audit-log write failures.
func (s *AuditService) LogAuth(ctx context.Context, ev AuthEvent) {
	if s == nil || s.auditRegistry == nil {
		return
	}

	entry := models.AuditLog{
		Action:       ev.Action,
		UserID:       ev.UserID,
		TenantID:     ev.TenantID,
		Success:      ev.Success,
		ErrorMessage: ev.ErrMsg,
	}

	if ev.Request != nil {
		entry.IPAddress = clientIPFromRequest(ev.Request)
		entry.UserAgent = ev.Request.UserAgent()
	}

	if _, err := s.auditRegistry.Create(ctx, entry); err != nil {
		slog.Error("Failed to write audit log entry", "action", ev.Action, "error", err)
	}
}

// LogAdmin records a platform-administrative event (grant/revoke
// system-admin, future impersonation start/end). ev.Action is the full
// event name supplied by the caller (e.g. "admin.grant_system_admin")
// so audit consumers can filter on a single column.
//
// When the request context carries impersonation claims (`imp` == true
// from the access token), the persisted row's ImpersonatedBy column
// records the *operator-of-record* — the system admin who initiated the
// impersonation — so the audit trail reflects "denis acting as alice",
// not just "alice". The impersonation primitive itself ships in #1750;
// this helper is provisioned now so the column is populated as soon as
// the primitive lands without a follow-up wiring change.
//
// The helper is best-effort: like LogAuth, write failures are logged via
// slog but not returned to the caller so admin flows are never blocked by
// audit-log write blips.
func (s *AuditService) LogAdmin(ctx context.Context, ev AdminEvent) {
	if s == nil || s.auditRegistry == nil {
		return
	}

	entry := models.AuditLog{
		Action:         ev.Action,
		UserID:         ev.ActorID,
		TenantID:       ev.TenantID,
		EntityType:     ev.SubjectType,
		EntityID:       ev.SubjectID,
		Success:        ev.Success,
		ErrorMessage:   ev.ErrMsg,
		ImpersonatedBy: impersonatorFromContext(ctx),
	}

	if ev.Request != nil {
		entry.IPAddress = clientIPFromRequest(ev.Request)
		entry.UserAgent = ev.Request.UserAgent()
	}

	// When the caller supplied any breadcrumb fields, encode them as a
	// JSON blob in user_agent and tuck the real UA under "ua" so the
	// row carries the full context — see AdminEvent doc-comment for
	// the rationale.
	if breadcrumb := adminAuditBreadcrumb(ev, entry.UserAgent); breadcrumb != "" {
		entry.UserAgent = breadcrumb
	}

	if _, err := s.auditRegistry.Create(ctx, entry); err != nil {
		slog.Error("Failed to write admin audit log entry", "action", ev.Action, "error", err)
	}
}

// adminBreadcrumbReasonMaxLen caps the breadcrumb reason at 500
// characters as defence-in-depth against non-HTTP callers (future CLI
// admin actions, in-process service helpers) that don't share the
// HTTP-decoder's input validation. The audit_logs.user_agent column
// tunnels the JSON breadcrumb today (#1747) — a multi-KB reason would
// bloat the row without an upper bound here.
const adminBreadcrumbReasonMaxLen = 500

// adminBreadcrumbTruncateMarker is the suffix appended to a reason that
// exceeds the cap so post-hoc readers can tell the value was truncated
// rather than provided verbatim.
const adminBreadcrumbTruncateMarker = "…"

// adminAuditBreadcrumb returns a JSON-encoded breadcrumb if any of the
// optional context fields are populated, or "" to signal that the
// caller should keep the existing user_agent value (raw UA string).
// Keeping the fall-through path explicit avoids retroactively rewriting
// every existing admin.* audit row to a noisy {"ua":"..."} blob.
func adminAuditBreadcrumb(ev AdminEvent, rawUA string) string {
	if ev.Reason == "" && !ev.Forced && len(ev.Extra) == 0 {
		return ""
	}
	payload := make(map[string]any, len(ev.Extra)+3)
	maps.Copy(payload, ev.Extra)
	if ev.Reason != "" {
		payload["reason"] = truncateReason(ev.Reason)
	}
	if ev.Forced {
		payload["forced"] = true
	}
	if rawUA != "" {
		payload["ua"] = rawUA
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		// Falling back to the raw UA keeps the row intact when the
		// breadcrumb payload contains an unsupported type — the
		// alternative would silently drop the audit row, which is
		// strictly worse than losing the breadcrumb context.
		slog.Error("Failed to marshal admin audit breadcrumb", "action", ev.Action, "error", err)
		return ""
	}
	return string(encoded)
}

// truncateReason enforces the audit-breadcrumb reason cap. Defence-in-
// depth: the HTTP decoder already rejects oversized reasons with a 422,
// but a future CLI / in-process caller might call LogAdmin directly with
// a multi-KB string. Truncating here keeps the user_agent column bounded
// regardless of caller hygiene.
//
// The cap is measured in runes (Unicode code points), consistent with
// the OpenAPI `maxLength` semantics on the request schema. The
// truncation marker counts toward the cap so the returned string is
// always ≤ adminBreadcrumbReasonMaxLen runes — never one rune over.
func truncateReason(reason string) string {
	if utf8.RuneCountInString(reason) <= adminBreadcrumbReasonMaxLen {
		return reason
	}
	markerRunes := utf8.RuneCountInString(adminBreadcrumbTruncateMarker)
	keep := adminBreadcrumbReasonMaxLen - markerRunes
	if keep <= 0 {
		// Pathological config (marker alone exceeds the cap). Fall back
		// to the marker truncated to the cap so the caller still gets
		// a bounded string rather than a panic.
		runes := []rune(adminBreadcrumbTruncateMarker)
		return string(runes[:adminBreadcrumbReasonMaxLen])
	}
	runes := []rune(reason)
	return string(runes[:keep]) + adminBreadcrumbTruncateMarker
}

// ImpersonatorIDFromContext returns the operator's user ID when the
// request context carries impersonation claims (`imp` == true with a
// non-empty `impersonated_by`), and nil otherwise. Exported so the HTTP
// layer can resolve "who actually initiated this request" without
// re-parsing the bearer token — for example to prevent an operator from
// self-blocking through an impersonated session (#1747).
//
// Returns nil for the common case where no impersonation is active.
func ImpersonatorIDFromContext(ctx context.Context) *string {
	return impersonatorFromContext(ctx)
}

// impersonatorFromContext returns the user ID stored in the `impersonated_by`
// JWT claim when the `imp` claim is true, otherwise nil. Nil is the common
// case: the vast majority of admin actions are performed without an active
// impersonation session.
func impersonatorFromContext(ctx context.Context) *string {
	claims := appctx.JWTClaimsFromContext(ctx)
	if claims == nil {
		return nil
	}
	imp, _ := claims["imp"].(bool)
	if !imp {
		return nil
	}
	by, ok := claims["impersonated_by"].(string)
	if !ok || by == "" {
		return nil
	}
	return &by
}

// clientIPFromRequest extracts the real client IP from the request, respecting
// common proxy headers. Duplicated here to keep the services package self-contained.
func clientIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (original client) IP from the comma-separated list and trim whitespace.
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr; strip the port using net.SplitHostPort so that
	// IPv6 addresses (with brackets) and host:port pairs are both handled correctly.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
