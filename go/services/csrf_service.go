package services

import (
	"log/slog"

	"go.5x5.cz/inventario/csrf"
	csrfinmemory "go.5x5.cz/inventario/csrf/inmemory"
	csrfredis "go.5x5.cz/inventario/csrf/redis"
)

// NewCSRFService selects the appropriate csrf.Service implementation based on
// configuration. When redisURL is non-empty a Redis-backed service is used
// (recommended for production and multi-instance deployments). Otherwise it
// falls back to an in-memory service with a warning.
func NewCSRFService(redisURL string) csrf.Service {
	if redisURL != "" {
		svc, err := csrfredis.NewFromURL(redisURL)
		if err != nil {
			slog.Error("Failed to create Redis CSRF service, falling back to in-memory", "error", err)
			return newInMemoryCSRFServiceWithWarning()
		}
		slog.Info("Using Redis CSRF service")
		return svc
	}
	return newInMemoryCSRFServiceWithWarning()
}

func newInMemoryCSRFServiceWithWarning() *csrfinmemory.Service {
	slog.Warn("Using in-memory CSRF service — not suitable for multi-instance deployments; set --csrf-redis-url for production")
	return csrfinmemory.New()
}
