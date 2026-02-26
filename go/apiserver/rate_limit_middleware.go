package apiserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/services"
)

// AuthLoginRateLimitMiddleware enforces per-IP rate limiting on the login endpoint.
// It sets X-RateLimit-* headers on all responses for observability.
func AuthLoginRateLimitMiddleware(limiter services.AuthRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Use RemoteAddr only — never trust X-Forwarded-For/X-Real-IP for rate
			// limiting, since those headers can be spoofed by any client that is
			// not behind a verified trusted proxy.
			ip := remoteAddrIP(r)
			res, err := limiter.CheckLoginAttempt(r.Context(), ip)
			if err != nil {
				// Fail-open: do not make auth unavailable due to limiter backend outages.
				slog.Error("Auth rate limiter error", "error", err, "ip", ip)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", res.Limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", res.ResetAt.Unix()))

			if !res.Allowed {
				retryAfter := max(int(time.Until(res.ResetAt).Seconds()), 0)
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RegistrationRateLimitMiddleware enforces per-IP rate limiting on registration endpoints.
func RegistrationRateLimitMiddleware(limiter services.AuthRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}
			ip := remoteAddrIP(r)
			res, err := limiter.CheckRegistrationAttempt(r.Context(), ip)
			if err != nil {
				slog.Error("Registration rate limiter error", "error", err, "ip", ip)
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", res.Limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", res.ResetAt.Unix()))
			if !res.Allowed {
				retryAfter := max(int(time.Until(res.ResetAt).Seconds()), 0)
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// PasswordResetRateLimitMiddleware enforces per-email rate limiting on the forgot-password endpoint.
// The email is extracted from the request body and used as the rate-limit key.
// Because we must read the body to extract the email, this middleware reconstructs
// r.Body so the downstream handler can still decode it.
func PasswordResetRateLimitMiddleware(limiter services.AuthRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Limit body size before reading to prevent DoS via large payloads.
			const maxBodyBytes = 4096
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			// Read the body so we can extract the email, then restore it.
			bodyBytes, err := io.ReadAll(r.Body)
			_ = r.Body.Close()
			if err != nil || len(bodyBytes) == 0 {
				// Let the handler deal with the malformed body.
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				next.ServeHTTP(w, r)
				return
			}
			// Restore body for the downstream handler.
			r.Body = io.NopCloser(bytes.NewReader(bodyBytes))

			// Extract the email without consuming the body permanently.
			var peek struct {
				Email string `json:"email"`
			}
			_ = json.Unmarshal(bodyBytes, &peek)

			email := strings.ToLower(strings.TrimSpace(peek.Email))
			if email == "" {
				// Let the handler return 400.
				next.ServeHTTP(w, r)
				return
			}

			res, err := limiter.CheckPasswordResetAttempt(r.Context(), email)
			if err != nil {
				slog.Error("Password-reset rate limiter error", "error", err, "email", email)
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", res.Limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", res.ResetAt.Unix()))
			if !res.Allowed {
				retryAfter := max(int(time.Until(res.ResetAt).Seconds()), 0)
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// remoteAddrIP extracts the host from r.RemoteAddr, ignoring all proxy headers.
// This is intentional for rate limiting: proxy headers like X-Forwarded-For can be
// forged by the client and must not be used to determine the key to rate-limit on.
func remoteAddrIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}
