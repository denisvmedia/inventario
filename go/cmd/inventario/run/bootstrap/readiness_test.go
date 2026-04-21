package bootstrap_test

import (
	"sort"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/run/bootstrap"
)

func TestNewReadinessRedisPinger_NoRedisConfigured(t *testing.T) {
	c := qt.New(t)

	pinger := bootstrap.NewReadinessRedisPinger(&bootstrap.Config{})

	c.Assert(pinger, qt.IsNil)
}

func TestNewReadinessRedisPinger_MultipleDependenciesWithDedupedURLs(t *testing.T) {
	c := qt.New(t)

	pinger := bootstrap.NewReadinessRedisPinger(&bootstrap.Config{
		TokenBlacklistRedisURL:  "redis://localhost:6379/0",
		AuthRateLimitRedisURL:   "redis://localhost:6379/1",
		GlobalRateLimitRedisURL: "redis://localhost:6379/1",
		CSRFRedisURL:            "redis://localhost:6379/2",
	})
	c.Assert(pinger, qt.IsNotNil)
	defer func() { c.Assert(pinger.Close(), qt.IsNil) }()

	names := pinger.TargetNames()
	sort.Strings(names)
	c.Assert(names, qt.DeepEquals, []string{
		"auth_rate_limit,global_rate_limit",
		"csrf",
		"token_blacklist",
	})
}

func TestNewReadinessRedisPinger_DisabledLimitersAreExcluded(t *testing.T) {
	c := qt.New(t)

	pinger := bootstrap.NewReadinessRedisPinger(&bootstrap.Config{
		TokenBlacklistRedisURL:  "redis://localhost:6379/0",
		AuthRateLimitRedisURL:   "redis://localhost:6379/1",
		AuthRateLimitDisabled:   true,
		GlobalRateLimitRedisURL: "redis://localhost:6379/2",
		GlobalRateLimitDisabled: true,
		CSRFRedisURL:            "redis://localhost:6379/3",
	})
	c.Assert(pinger, qt.IsNotNil)
	defer func() { c.Assert(pinger.Close(), qt.IsNil) }()

	names := pinger.TargetNames()
	sort.Strings(names)
	c.Assert(names, qt.DeepEquals, []string{"csrf", "token_blacklist"})
}

func TestNewReadinessRedisPinger_AllInvalidURLsReturnsNil(t *testing.T) {
	c := qt.New(t)

	pinger := bootstrap.NewReadinessRedisPinger(&bootstrap.Config{
		TokenBlacklistRedisURL: "://invalid",
		CSRFRedisURL:           "://also-invalid",
	})

	c.Assert(pinger, qt.IsNil)
}
