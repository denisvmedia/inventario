package apiserver

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/rs/cors"
)

var defaultAllowedMethods = []string{
	http.MethodGet,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodOptions,
}

var defaultAllowedHeaders = []string{
	"Accept",
	"Authorization",
	"Content-Type",
	"X-CSRF-Token",
	"X-Auth-Check",
	"X-Request-ID",
}

var defaultExposedHeaders = []string{
	"X-CSRF-Token",
	"X-RateLimit-Limit",
	"X-RateLimit-Remaining",
	"X-RateLimit-Reset",
	"X-Total-Count",
	"X-Page-Count",
}

const defaultCORSMaxAge = 300

// CORSConfig defines CORS middleware behavior.
type CORSConfig struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials *bool
	MaxAge           int
}

// DefaultCORSConfig returns strict defaults suitable for production-safe behavior.
// AllowedOrigins intentionally defaults to empty (fail-closed for cross-origin requests).
func DefaultCORSConfig() CORSConfig {
	allowCredentials := true
	return CORSConfig{
		AllowedMethods:   append([]string(nil), defaultAllowedMethods...),
		AllowedHeaders:   append([]string(nil), defaultAllowedHeaders...),
		ExposedHeaders:   append([]string(nil), defaultExposedHeaders...),
		AllowCredentials: &allowCredentials,
		MaxAge:           defaultCORSMaxAge,
	}
}

// DefaultDevAllowedOrigins returns the local development CORS origin allowlist.
func DefaultDevAllowedOrigins() []string {
	return []string{
		"http://localhost:5173",
		"http://localhost:3000",
	}
}

// ParseAllowedOrigins parses comma-separated origins.
// Empty input returns an empty list (fail-closed for cross-origin requests).
func ParseAllowedOrigins(originsRaw string) ([]string, error) {
	originsRaw = strings.TrimSpace(originsRaw)
	if originsRaw == "" {
		return nil, nil
	}

	parts := strings.Split(originsRaw, ",")
	origins := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		if origin == "*" || strings.EqualFold(origin, "null") {
			return nil, fmt.Errorf("unsafe CORS origin %q is not allowed", origin)
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}

	return origins, nil
}

func normalizeCORSConfig(cfg CORSConfig) CORSConfig {
	def := DefaultCORSConfig()

	if len(cfg.AllowedMethods) == 0 {
		cfg.AllowedMethods = def.AllowedMethods
	}
	if len(cfg.AllowedHeaders) == 0 {
		cfg.AllowedHeaders = def.AllowedHeaders
	}
	if len(cfg.ExposedHeaders) == 0 {
		cfg.ExposedHeaders = def.ExposedHeaders
	}
	if cfg.MaxAge <= 0 {
		cfg.MaxAge = def.MaxAge
	}
	if cfg.AllowCredentials == nil {
		cfg.AllowCredentials = def.AllowCredentials
	}

	return cfg
}

// NewCORSMiddleware builds CORS middleware from config.
func NewCORSMiddleware(config CORSConfig) *cors.Cors {
	cfg := normalizeCORSConfig(config)
	allowCredentials := false
	if cfg.AllowCredentials != nil {
		allowCredentials = *cfg.AllowCredentials
	}
	return cors.New(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   cfg.AllowedMethods,
		AllowedHeaders:   cfg.AllowedHeaders,
		ExposedHeaders:   cfg.ExposedHeaders,
		AllowCredentials: allowCredentials,
		MaxAge:           cfg.MaxAge,
	})
}
