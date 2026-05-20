package apiserver

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// adminSubtreePathPrefix is the request-path prefix of the platform-admin
// subtree. The router mounts Route("/admin", ...) inside the "/api/v1"
// group (see APIServer in apiserver.go), so every admin request path
// begins with exactly this string. The trailing slash is deliberate: it
// makes the prefix match an actual sub-route and not an unrelated sibling
// path such as "/api/v1/administrate".
const adminSubtreePathPrefix = "/api/v1/admin/"

// isAdminSubtreePath reports whether the request targets the platform-admin
// subtree (/api/v1/admin/*). Used by ValidateNoUserProvidedTenantID to
// exempt that subtree — and only that subtree — from the query-parameter
// "tenant" check.
func isAdminSubtreePath(path string) bool {
	return strings.HasPrefix(path, adminSubtreePathPrefix)
}

// tenantSecurityViolationMsg is the response body returned to the client
// whenever ValidateNoUserProvidedTenantID rejects a request.
const tenantSecurityViolationMsg = "Security violation: tenant information cannot be provided by user"

// rejectTenantHeader checks request headers for any name containing
// "tenant" (case-insensitive). Returns true — having written a 403 — when a
// violation was found; false when the headers are clean.
func rejectTenantHeader(w http.ResponseWriter, r *http.Request) bool {
	for headerName, headerValues := range r.Header {
		if !strings.Contains(strings.ToLower(headerName), "tenant") {
			continue
		}
		slog.Error("Security violation: user-provided tenant ID in header",
			"header", headerName,
			"value", strings.Join(headerValues, ","),
			"user_agent", r.UserAgent(),
			"remote_addr", r.RemoteAddr,
			"method", r.Method,
			"path", r.URL.Path,
		)
		http.Error(w, tenantSecurityViolationMsg, http.StatusForbidden)
		return true
	}
	return false
}

// rejectTenantQueryParam checks query parameters for any name containing
// "tenant" (case-insensitive). Returns true — having written a 403 — when a
// violation was found; false when the query string is clean.
//
// This check is exempted for the platform-admin subtree (/api/v1/admin/*),
// which is cross-tenant BY DESIGN and gated by RequireSystemAdmin. A system
// admin supplying ?tenantID=<id> there is using a documented listing
// *filter*, not injecting a tenant context: admin handlers resolve data
// through the FactorySet directly (not the RLS-context-derived per-request
// tenant), so a tenantID query value is never consumed as a tenant-context
// override and cannot escalate access beyond what RequireSystemAdmin
// already grants. This middleware runs before JWT/RequireSystemAdmin, so a
// non-admin hitting /api/v1/admin/groups?tenantID=x passes this check but
// is still rejected downstream by RequireSystemAdmin — no security
// regression. The exemption is query-parameter only; rejectTenantHeader
// and rejectTenantBody stay fully in force for admin paths.
func rejectTenantQueryParam(w http.ResponseWriter, r *http.Request) bool {
	if isAdminSubtreePath(r.URL.Path) {
		return false
	}
	for paramName, paramValues := range r.URL.Query() {
		if !strings.Contains(strings.ToLower(paramName), "tenant") {
			continue
		}
		slog.Error("Security violation: user-provided tenant ID in query parameter",
			"param", paramName,
			"value", strings.Join(paramValues, ","),
			"user_agent", r.UserAgent(),
			"remote_addr", r.RemoteAddr,
			"method", r.Method,
			"path", r.URL.Path,
		)
		http.Error(w, tenantSecurityViolationMsg, http.StatusForbidden)
		return true
	}
	return false
}

// rejectTenantBody checks the request body of mutating requests for
// tenant_id fields. It reads and restores the body so downstream handlers
// still see it. Returns true — having written a 4xx — when a violation (or
// a body-read failure) was found; false when the body is clean or the
// method is not body-bearing.
func rejectTenantBody(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
		return false
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		slog.Error("Failed to read request body for security validation", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return true
	}
	// Restore body for downstream handlers.
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	bodyLower := strings.ToLower(string(body))
	if !strings.Contains(bodyLower, "tenant_id") &&
		!strings.Contains(bodyLower, "\"tenant\"") &&
		!strings.Contains(bodyLower, "'tenant'") {
		return false
	}
	slog.Error("Security violation: user-provided tenant ID in request body",
		"content_type", r.Header.Get("Content-Type"),
		"body_preview", truncateString(string(body), 200),
		"user_agent", r.UserAgent(),
		"remote_addr", r.RemoteAddr,
		"method", r.Method,
		"path", r.URL.Path,
	)
	http.Error(w, tenantSecurityViolationMsg, http.StatusForbidden)
	return true
}

// ValidateNoUserProvidedTenantID creates middleware that rejects any user-provided tenant information
// This is critical for preventing cross-tenant data access attacks
func ValidateNoUserProvidedTenantID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if rejectTenantHeader(w, r) ||
				rejectTenantQueryParam(w, r) ||
				rejectTenantBody(w, r) {
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RejectSpecificTenantHeaders rejects requests with specific tenant-related headers
// This provides an additional layer of protection against known attack vectors
func RejectSpecificTenantHeaders() func(http.Handler) http.Handler {
	forbiddenHeaders := []string{
		"X-Tenant-ID",
		"X-Tenant",
		"Tenant-ID",
		"Tenant",
		"X-Tenant-Context",
		"Tenant-Context",
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, header := range forbiddenHeaders {
				if value := r.Header.Get(header); value != "" {
					slog.Error("Security violation: forbidden tenant header detected",
						"header", header,
						"value", value,
						"user_agent", r.UserAgent(),
						"remote_addr", r.RemoteAddr,
						"method", r.Method,
						"path", r.URL.Path,
					)
					http.Error(w, fmt.Sprintf("Security violation: %s header not allowed", header), http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LogSecurityAttempts logs all requests for security monitoring
func LogSecurityAttempts() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Log all requests for security monitoring
			slog.Debug("API request",
				"method", r.Method,
				"path", r.URL.Path,
				"user_agent", r.UserAgent(),
				"remote_addr", r.RemoteAddr,
				"content_type", r.Header.Get("Content-Type"),
			)

			next.ServeHTTP(w, r)
		})
	}
}

// truncateString truncates a string to maxLength characters
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	return s[:maxLength] + "..."
}
