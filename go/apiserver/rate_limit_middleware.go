package apiserver

import (
	"fmt"
	"log/slog"
	"net/http"
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

			ip := getClientIP(r)
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
				retryAfter := int(time.Until(res.ResetAt).Seconds())
				if retryAfter < 0 {
					retryAfter = 0
				}
				w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
				http.Error(w, "Rate limit exceeded. Please try again later.", http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
