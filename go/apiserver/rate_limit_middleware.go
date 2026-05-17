package apiserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/services"
)

// ParseTrustedProxyCIDRs parses a comma-separated list of trusted proxy CIDRs or IPs.
// Bare IPs are converted to /32 or /128 prefixes.
func ParseTrustedProxyCIDRs(raw string) ([]*net.IPNet, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	nets := make([]*net.IPNet, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		if strings.Contains(value, "/") {
			_, ipNet, err := net.ParseCIDR(value)
			if err != nil {
				return nil, fmt.Errorf("invalid trusted proxy CIDR %q: %w", value, err)
			}
			key := ipNet.String()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			nets = append(nets, ipNet)
			continue
		}

		ip := net.ParseIP(value)
		if ip == nil {
			return nil, fmt.Errorf("invalid trusted proxy IP %q", value)
		}
		var mask net.IPMask
		if ip.To4() != nil {
			ip = ip.To4()
			mask = net.CIDRMask(32, 32)
		} else {
			mask = net.CIDRMask(128, 128)
		}
		ipNet := &net.IPNet{IP: ip, Mask: mask}
		key := ipNet.String()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		nets = append(nets, ipNet)
	}

	return nets, nil
}

// GlobalRateLimitMiddleware enforces API-wide per-IP rate limiting.
// It sets X-RateLimit-* headers on all responses for observability.
func GlobalRateLimitMiddleware(limiter services.GlobalRateLimiter, trustedProxyNets []*net.IPNet) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Prefer real client IP headers only when the request comes from a
			// configured trusted proxy; otherwise fall back to RemoteAddr.
			ip := clientIPForGlobalRateLimit(r, trustedProxyNets)
			res, err := limiter.Check(r.Context(), ip)
			if err != nil {
				// Fail-open: do not make API unavailable due to limiter backend outages.
				slog.Error("Global rate limiter error", "error", err, "ip", ip)
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", res.Limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", res.ResetAt.Unix()))

			if !res.Allowed {
				retryAfter := max(int(time.Until(res.ResetAt).Seconds()), 0)
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				slog.Warn("Global rate limit exceeded", "ip", ip, "path", r.URL.Path, "method", r.Method, "retry_after_seconds", retryAfter)
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func clientIPForGlobalRateLimit(r *http.Request, trustedProxyNets []*net.IPNet) string {
	remoteIP := remoteAddrIP(r)
	if !isTrustedProxyIP(remoteIP, trustedProxyNets) {
		return remoteIP
	}

	// Trusted proxy: walk X-Forwarded-For right-to-left, skipping known proxy
	// hops, and return the first non-trusted IP as the real client IP.
	// Right-to-left is required because the edge proxy appends the peer address;
	// the leftmost entry can be spoofed by the client.
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		for _, part := range slices.Backward(parts) {
			candidate := strings.TrimSpace(part)
			if candidate == "" {
				continue
			}
			ip := net.ParseIP(candidate)
			if ip == nil {
				continue
			}
			if !isTrustedProxyIP(ip.String(), trustedProxyNets) {
				return ip.String()
			}
		}
	}

	// Fallback to X-Real-IP when set by trusted proxy.
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		if ip := net.ParseIP(xri); ip != nil {
			return ip.String()
		}
	}

	return remoteIP
}

func isTrustedProxyIP(ipStr string, trustedProxyNets []*net.IPNet) bool {
	if len(trustedProxyNets) == 0 {
		return false
	}
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return false
	}
	for _, ipNet := range trustedProxyNets {
		if ipNet != nil && ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// AuthLoginRateLimitMiddleware enforces per-IP rate limiting on the login endpoint.
// It sets X-RateLimit-* headers on all responses for observability.
func AuthLoginRateLimitMiddleware(limiter services.AuthRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}
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
				slog.Warn("Login rate limit exceeded", "ip", ip, "retry_after_seconds", retryAfter)
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
				slog.Warn("Registration rate limit exceeded", "ip", ip, "retry_after_seconds", retryAfter)
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// FeedbackRateLimitMiddleware enforces per-user rate limiting on the
// POST /feedback endpoint (#1387). The user is read from the request
// context — this middleware must therefore sit AFTER JWTMiddleware /
// RLSContextMiddleware in the chain so appctx.UserFromContext returns
// a non-nil principal.
//
// On rate-limiter backend error the middleware fails open (matches the
// other rate limiters in this package); the operator still sees the
// error in the structured log.
func FeedbackRateLimitMiddleware(limiter services.AuthRateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}
			user := appctx.UserFromContext(r.Context())
			if user == nil || user.ID == "" {
				// No user context — JWT middleware should have already
				// rejected the request with 401; if we get here, the
				// safe default is to let the handler enforce auth and
				// return its own 401.
				next.ServeHTTP(w, r)
				return
			}
			res, err := limiter.CheckFeedbackAttempt(r.Context(), user.ID)
			if err != nil {
				slog.Error("Feedback rate limiter error", "error", err, "user_id", user.ID)
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", res.Limit))
			w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", res.Remaining))
			w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", res.ResetAt.Unix()))
			if !res.Allowed {
				retryAfter := max(int(time.Until(res.ResetAt).Seconds()), 0)
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				slog.Warn("Feedback rate limit exceeded", "user_id", user.ID, "retry_after_seconds", retryAfter)
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
			// io.LimitReader is used instead of http.MaxBytesReader so the middleware
			// has sole control over the response: LimitReader never writes to the
			// ResponseWriter, eliminating any risk of a double-write on oversized payloads.
			const maxBodyBytes = 4096
			bodyBytes, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes+1))
			_ = r.Body.Close()
			if err != nil {
				// Read error: let the handler deal with it.
				r.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				next.ServeHTTP(w, r)
				return
			}
			if int64(len(bodyBytes)) > maxBodyBytes {
				// Body exceeded the limit; we control the 413 here exclusively.
				http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			if len(bodyBytes) == 0 {
				// Empty body: let the handler return 400.
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
				slog.Warn("Password-reset rate limit exceeded", "ip", remoteAddrIP(r), "retry_after_seconds", retryAfter)
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
