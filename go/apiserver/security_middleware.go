package apiserver

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// ValidateNoUserProvidedTenantID creates middleware that rejects any user-provided tenant information
// This is critical for preventing cross-tenant data access attacks
func ValidateNoUserProvidedTenantID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 1. Check headers for tenant_id (case-insensitive)
			for headerName, headerValues := range r.Header {
				headerLower := strings.ToLower(headerName)
				if strings.Contains(headerLower, "tenant") {
					slog.Error("Security violation: user-provided tenant ID in header",
						"header", headerName,
						"value", strings.Join(headerValues, ","),
						"user_agent", r.UserAgent(),
						"remote_addr", r.RemoteAddr,
						"method", r.Method,
						"path", r.URL.Path,
					)
					http.Error(w, "Security violation: tenant information cannot be provided by user", http.StatusForbidden)
					return
				}
			}

			// 2. Check query parameters for tenant_id (case-insensitive)
			for paramName, paramValues := range r.URL.Query() {
				paramLower := strings.ToLower(paramName)
				if strings.Contains(paramLower, "tenant") {
					slog.Error("Security violation: user-provided tenant ID in query parameter",
						"param", paramName,
						"value", strings.Join(paramValues, ","),
						"user_agent", r.UserAgent(),
						"remote_addr", r.RemoteAddr,
						"method", r.Method,
						"path", r.URL.Path,
					)
					http.Error(w, "Security violation: tenant information cannot be provided by user", http.StatusForbidden)
					return
				}
			}

			// 3. Check request body for tenant_id fields (for POST/PUT/PATCH requests)
			if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
				body, err := io.ReadAll(r.Body)
				if err != nil {
					slog.Error("Failed to read request body for security validation", "error", err)
					http.Error(w, "Failed to read request body", http.StatusBadRequest)
					return
				}

				// Restore body for downstream handlers
				r.Body = io.NopCloser(bytes.NewBuffer(body))

				// Check for tenant_id in request body (case-insensitive)
				bodyLower := strings.ToLower(string(body))
				if strings.Contains(bodyLower, "tenant_id") ||
					strings.Contains(bodyLower, "\"tenant\"") ||
					strings.Contains(bodyLower, "'tenant'") {
					slog.Error("Security violation: user-provided tenant ID in request body",
						"content_type", r.Header.Get("Content-Type"),
						"body_preview", truncateString(string(body), 200),
						"user_agent", r.UserAgent(),
						"remote_addr", r.RemoteAddr,
						"method", r.Method,
						"path", r.URL.Path,
					)
					http.Error(w, "Security violation: tenant information cannot be provided by user", http.StatusForbidden)
					return
				}
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
