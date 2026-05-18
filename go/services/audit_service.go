package services

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// AuditLogger is the interface for logging security-relevant events.
type AuditLogger interface {
	// LogAuth records an authentication or authorization event (login, logout, password change, etc.).
	// The call is best-effort: errors are logged but do not propagate to callers.
	LogAuth(ctx context.Context, action string, userID, tenantID *string, success bool, r *http.Request, errMsg *string)

	// LogAdmin records a platform-administrative event (grant/revoke
	// system-admin, future impersonation start/end). success is explicit
	// (matching LogAuth) so callers can record a *failed* admin action
	// without inventing an error message — e.g. last-admin guard rejected
	// the revoke before any handler error existed. The call is
	// best-effort like LogAuth: errors are logged but do not propagate.
	LogAdmin(ctx context.Context, action string, actorID, tenantID, subjectType, subjectID *string, success bool, r *http.Request, errMsg *string)
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
func (s *AuditService) LogAuth(ctx context.Context, action string, userID, tenantID *string, success bool, r *http.Request, errMsg *string) {
	if s == nil || s.auditRegistry == nil {
		return
	}

	entry := models.AuditLog{
		Action:       action,
		UserID:       userID,
		TenantID:     tenantID,
		Success:      success,
		ErrorMessage: errMsg,
	}

	if r != nil {
		entry.IPAddress = clientIPFromRequest(r)
		entry.UserAgent = r.UserAgent()
	}

	if _, err := s.auditRegistry.Create(ctx, entry); err != nil {
		slog.Error("Failed to write audit log entry", "action", action, "error", err)
	}
}

// LogAdmin records a platform-administrative event (grant/revoke
// system-admin, future impersonation start/end). The action string is the
// full event name supplied by the caller (e.g. "admin.grant_system_admin")
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
func (s *AuditService) LogAdmin(ctx context.Context, action string, actorID, tenantID, subjectType, subjectID *string, success bool, r *http.Request, errMsg *string) {
	if s == nil || s.auditRegistry == nil {
		return
	}

	entry := models.AuditLog{
		Action:         action,
		UserID:         actorID,
		TenantID:       tenantID,
		EntityType:     subjectType,
		EntityID:       subjectID,
		Success:        success,
		ErrorMessage:   errMsg,
		ImpersonatedBy: impersonatorFromContext(ctx),
	}

	if r != nil {
		entry.IPAddress = clientIPFromRequest(r)
		entry.UserAgent = r.UserAgent()
	}

	if _, err := s.auditRegistry.Create(ctx, entry); err != nil {
		slog.Error("Failed to write admin audit log entry", "action", action, "error", err)
	}
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
