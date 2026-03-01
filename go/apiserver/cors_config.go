package apiserver

import (
	"net/http"
	"slices"
	"strings"

	"github.com/rs/cors"
)

var defaultAllowedOrigins = []string{
	"http://localhost:5173",
	"http://localhost:3000",
}

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
	AllowCredentials bool
	MaxAge           int
}

// DefaultCORSConfig returns strict defaults suitable for local development
// and production-safe behavior (no wildcard origins).
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowedOrigins:   slices.Clone(defaultAllowedOrigins),
		AllowedMethods:   slices.Clone(defaultAllowedMethods),
		AllowedHeaders:   slices.Clone(defaultAllowedHeaders),
		ExposedHeaders:   slices.Clone(defaultExposedHeaders),
		AllowCredentials: true,
		MaxAge:           defaultCORSMaxAge,
	}
}

// ParseAllowedOrigins parses comma-separated origins.
// Empty input falls back to development defaults.
func ParseAllowedOrigins(originsRaw string) []string {
	originsRaw = strings.TrimSpace(originsRaw)
	if originsRaw == "" {
		return slices.Clone(defaultAllowedOrigins)
	}

	parts := strings.Split(originsRaw, ",")
	origins := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		if _, ok := seen[origin]; ok {
			continue
		}
		seen[origin] = struct{}{}
		origins = append(origins, origin)
	}

	if len(origins) == 0 {
		return slices.Clone(defaultAllowedOrigins)
	}

	return origins
}

func normalizeCORSConfig(cfg CORSConfig) CORSConfig {
	def := DefaultCORSConfig()

	if len(cfg.AllowedOrigins) == 0 {
		cfg.AllowedOrigins = def.AllowedOrigins
	}
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

	return cfg
}

// NewCORSMiddleware builds CORS middleware from config.
func NewCORSMiddleware(config CORSConfig) *cors.Cors {
	cfg := normalizeCORSConfig(config)
	return cors.New(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   cfg.AllowedMethods,
		AllowedHeaders:   cfg.AllowedHeaders,
		ExposedHeaders:   cfg.ExposedHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           cfg.MaxAge,
	})
}
