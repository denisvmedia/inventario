package services

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// AuditLogger is the interface for logging security-relevant events.
type AuditLogger interface {
	// LogAuth records an authentication or authorisation event (login, logout, password change, etc.).
	// The call is best-effort: errors are logged but do not propagate to callers.
	LogAuth(ctx context.Context, action string, userID, tenantID *string, success bool, r *http.Request, errMsg *string)
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
		ID:           uuid.New().String(),
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

// clientIPFromRequest extracts the real client IP from the request, respecting
// common proxy headers. Duplicated here to keep the services package self-contained.
func clientIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (original client) IP from the comma-separated list.
		for i, c := range xff {
			if c == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr.
	host := r.RemoteAddr
	if i := len(host) - 1; i >= 0 && host[i] != ']' {
		// Strip port: find last colon outside of IPv6 brackets.
		for j := i; j >= 0; j-- {
			if host[j] == ':' {
				host = host[:j]
				break
			}
		}
	}
	return host
}

