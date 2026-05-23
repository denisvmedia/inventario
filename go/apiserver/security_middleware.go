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

// tenantScanMaxBodyBytes is the maximum non-multipart request-body size
// that rejectTenantBody will accept and scan. It applies only to bodies
// that are actually scanned (multipart/* is skipped entirely — see
// rejectTenantBody); it is NOT a global request-body cap. The
// implementation actually reads up to tenantScanMaxBodyBytes+1 bytes via
// io.LimitReader so that a body of exactly this size still passes (it is
// buffered in full) but a body one byte larger trips the cap and
// short-circuits with 413 — the +1 is a sentinel read for overflow
// detection, not part of the contract.
//
// The value is intentionally much larger than every per-handler text-body
// cap in the codebase today (feedbackMaxRequestBodyBytes ≈ 53 KB;
// createInviteMaxBodyBytes = 4 KB; PasswordResetRateLimitMiddleware =
// 4 KB) so legitimate JSON / form payloads never trip it, while still
// being small enough that an attacker cannot force gigabyte-scale
// allocations per concurrent request.
const tenantScanMaxBodyBytes = 1 * 1024 * 1024 // 1 MiB

// rejectTenantBody checks the request body of mutating requests for
// tenant-identifying fields. Behaviour, in order:
//
//  1. Method gate: only POST/PUT/PATCH bodies are scanned. Anything else
//     passes through untouched (returns false).
//
//  2. Multipart skip: when Content-Type starts with "multipart/", the body
//     is skipped entirely without being read. Multipart payloads are
//     mostly binary file bytes that (a) can be gigabytes large and (b)
//     can randomly contain the substrings this scan looks for, producing
//     false-positive 403s. No handler in the codebase binds a tenant ID
//     from a multipart field, and the header / query / non-multipart-body
//     scans still catch the realistic injection paths.
//
//  3. Size cap: the body is read through io.LimitReader with a cap of
//     tenantScanMaxBodyBytes+1. If the read length exceeds the cap, the
//     middleware writes a 413 and short-circuits. io.LimitReader is used
//     in preference to http.MaxBytesReader to keep response-writing under
//     the middleware's sole control: MaxBytesReader writes to the
//     ResponseWriter when the limit is exceeded, which would race the
//     middleware's own http.Error call and risk a double-write. This is
//     the same idiom PasswordResetRateLimitMiddleware uses in
//     rate_limit_middleware.go for the same reason.
//
//  4. Substring scan: the read body is lowercased and checked against a
//     small mixed set of quote-anchored and unanchored substrings. This
//     is intentionally a cheap, case-insensitive scan rather than a JSON
//     parser — it is belt-and-suspenders defense-in-depth, not a contract
//     validator. The patterns matched are:
//     - snake_case "tenant_id" — UNANCHORED on purpose so it catches
//     form-encoded bodies such as "tenant_id=…", where the key is not
//     wrapped in quotes. This can over-fire if a free-text value
//     happens to contain the literal substring "tenant_id"; accepted
//     as a known belt-and-suspenders limitation because no handler
//     currently binds a "tenant_id" field anyway, so the cost of a
//     spurious 403 is bounded;
//     - the double-quoted form "\"tenant\"" — a JSON object key cannot
//     legitimately read free text without those surrounding quotes,
//     so the quote anchors make this match key-shaped occurrences
//     only;
//     - the single-quoted form "'tenant'" — pre-existing pattern that
//     covers non-JSON formats (YAML / JS-embedded) and is left in
//     place for back-compat. It can over-fire on free text that
//     contains 'tenant' in single quotes; same accepted trade-off as
//     the snake_case form;
//     - the camelCase JSON-key forms "\"tenantId\"" / "\"tenantID\""
//     (covered by the lowercased "\"tenantid\"" substring, quoted
//     to avoid flagging the bare word "tenantid" inside free text).
//     Only the double-quoted "\"tenantid\"" form is added — extending
//     the single-quoted variant would introduce a new false-positive
//     surface for description-style strings such as "...'tenantid'..."
//     without adding any coverage against JSON, which is the format
//     every current Inventario handler decodes. On a hit the middleware
//     writes a 403.
//
//  5. Body restoration: the buffered bytes are written back to r.Body
//     via io.NopCloser so downstream handlers can re-read.
//
// Returns true — having written a 4xx — when a violation, a body-read
// failure, or an oversize-body event occurred; false when the body is
// clean, the method is not body-bearing, or the content type is multipart.
func rejectTenantBody(w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
		return false
	}

	contentType := r.Header.Get("Content-Type")
	if strings.HasPrefix(strings.ToLower(contentType), "multipart/") {
		return false
	}

	// Bound the read so a malicious client cannot force unbounded
	// allocation here. See tenantScanMaxBodyBytes and the doc comment
	// above for the LimitReader-vs-MaxBytesReader rationale.
	body, err := io.ReadAll(io.LimitReader(r.Body, tenantScanMaxBodyBytes+1))
	_ = r.Body.Close() // mirrors PasswordResetRateLimitMiddleware; r.Body is replaced below
	if err != nil {
		slog.Error("Failed to read request body for security validation", "error", err)
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return true
	}
	if int64(len(body)) > tenantScanMaxBodyBytes {
		slog.Error("Security violation: request body exceeds tenant-scan size cap",
			"method", r.Method,
			"path", r.URL.Path,
			"content_type", contentType,
			"content_length", r.ContentLength,
			"user_agent", r.UserAgent(),
			"remote_addr", r.RemoteAddr,
		)
		http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		return true
	}
	// Restore body for downstream handlers. bytes.NewReader (read-only)
	// matches PasswordResetRateLimitMiddleware's restoration pattern in
	// rate_limit_middleware.go — the slice is never written to again
	// here, so the immutable reader is more accurate.
	r.Body = io.NopCloser(bytes.NewReader(body))

	// Convert []byte→string ONCE — the body can be up to
	// tenantScanMaxBodyBytes (1 MiB) on every POST/PUT/PATCH and a second
	// conversion in the violation-logging branch would double the
	// per-request allocation cost on a hot path.
	bodyStr := string(body)
	bodyLower := strings.ToLower(bodyStr)
	if !strings.Contains(bodyLower, "tenant_id") &&
		!strings.Contains(bodyLower, "\"tenant\"") &&
		!strings.Contains(bodyLower, "'tenant'") &&
		!strings.Contains(bodyLower, "\"tenantid\"") {
		return false
	}
	slog.Error("Security violation: user-provided tenant ID in request body",
		"content_type", contentType,
		"body_preview", truncateString(bodyStr, 200),
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
