package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/redis/go-redis/v9"
)

type redisReadinessTarget struct {
	name   string
	client *redis.Client
}

// ReadinessRedisPinger implements apiserver.RedisPinger for the deduplicated
// set of Redis clients configured across the token blacklist, auth/global rate
// limiters and CSRF storage.
type ReadinessRedisPinger struct {
	targets []redisReadinessTarget
}

// Ping pings every configured Redis dependency in sequence and returns the
// first error encountered (if any).
func (p *ReadinessRedisPinger) Ping(ctx context.Context) error {
	for _, target := range p.targets {
		if err := target.client.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("%s dependency ping failed: %w", target.name, err)
		}
	}
	return nil
}

// Close closes every configured Redis client and returns a joined error if any
// individual client fails to close.
func (p *ReadinessRedisPinger) Close() error {
	closeErrs := make([]error, 0)
	for _, target := range p.targets {
		if err := target.client.Close(); err != nil {
			closeErrs = append(closeErrs, fmt.Errorf("%s dependency close failed: %w", target.name, err))
		}
	}
	if len(closeErrs) == 0 {
		return nil
	}
	return errors.Join(closeErrs...)
}

// TargetNames returns the human-readable dependency name for every Redis target
// this pinger owns, in registration order. It is primarily intended for tests
// and diagnostic logging; production code should rely on Ping/Close instead of
// inspecting the target list directly.
func (p *ReadinessRedisPinger) TargetNames() []string {
	names := make([]string, len(p.targets))
	for i, t := range p.targets {
		names[i] = t.name
	}
	return names
}

// NewReadinessRedisPinger constructs a ReadinessRedisPinger that pings every
// Redis dependency configured on cfg, collapsing duplicate URLs onto a single
// shared client. Returns nil when no Redis dependency is configured or every
// configured URL is malformed. The returned value satisfies
// apiserver.RedisPinger and should usually be assigned to that interface type
// at the call site.
func NewReadinessRedisPinger(cfg *Config) *ReadinessRedisPinger {
	type redisDependency struct {
		name string
		url  string
	}

	deps := make([]redisDependency, 0, 4)
	if redisURL := strings.TrimSpace(cfg.TokenBlacklistRedisURL); redisURL != "" {
		deps = append(deps, redisDependency{name: "token_blacklist", url: redisURL})
	}
	if !cfg.AuthRateLimitDisabled {
		if redisURL := strings.TrimSpace(cfg.AuthRateLimitRedisURL); redisURL != "" {
			deps = append(deps, redisDependency{name: "auth_rate_limit", url: redisURL})
		}
	}
	if !cfg.GlobalRateLimitDisabled {
		if redisURL := strings.TrimSpace(cfg.GlobalRateLimitRedisURL); redisURL != "" {
			deps = append(deps, redisDependency{name: "global_rate_limit", url: redisURL})
		}
	}
	if redisURL := strings.TrimSpace(cfg.CSRFRedisURL); redisURL != "" {
		deps = append(deps, redisDependency{name: "csrf", url: redisURL})
	}
	if len(deps) == 0 {
		return nil
	}
	groupedNamesByURL := make(map[string][]string, len(deps))
	orderedURLs := make([]string, 0, len(deps))
	for _, dep := range deps {
		if _, exists := groupedNamesByURL[dep.url]; !exists {
			orderedURLs = append(orderedURLs, dep.url)
		}
		groupedNamesByURL[dep.url] = append(groupedNamesByURL[dep.url], dep.name)
	}

	targets := make([]redisReadinessTarget, 0, len(orderedURLs))
	for _, redisURL := range orderedURLs {
		dependencyNames := groupedNamesByURL[redisURL]
		opts, err := redis.ParseURL(redisURL)
		if err != nil {
			slog.Warn(
				"Invalid Redis URL for readiness check; Redis dependency checks will be skipped",
				"dependencies",
				strings.Join(dependencyNames, ","),
				"error",
				err,
			)
			continue
		}
		targets = append(targets, redisReadinessTarget{
			name:   strings.Join(dependencyNames, ","),
			client: redis.NewClient(opts),
		})
	}

	if len(targets) == 0 {
		return nil
	}

	return &ReadinessRedisPinger{targets: targets}
}
