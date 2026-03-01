package run

import (
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestNewReadinessRedisPinger_NoRedisConfigured(t *testing.T) {
	c := qt.New(t)

	cmd := &Command{}
	pinger := cmd.newReadinessRedisPinger()

	c.Assert(pinger, qt.IsNil)
}

func TestNewReadinessRedisPinger_MultipleDependenciesWithDedupedURLs(t *testing.T) {
	c := qt.New(t)

	cmd := &Command{
		config: Config{
			TokenBlacklistRedisURL:  "redis://localhost:6379/0",
			AuthRateLimitRedisURL:   "redis://localhost:6379/1",
			GlobalRateLimitRedisURL: "redis://localhost:6379/1",
			CSRFRedisURL:            "redis://localhost:6379/2",
		},
	}

	pinger := cmd.newReadinessRedisPinger()
	c.Assert(pinger, qt.IsNotNil)

	typedPinger, ok := pinger.(*readinessRedisPinger)
	c.Assert(ok, qt.IsTrue)
	c.Assert(typedPinger.targets, qt.HasLen, 3)

	targetNames := map[string]bool{}
	for _, target := range typedPinger.targets {
		targetNames[target.name] = true
	}

	c.Assert(targetNames["token_blacklist"], qt.IsTrue)
	c.Assert(targetNames["auth_rate_limit,global_rate_limit"], qt.IsTrue)
	c.Assert(targetNames["csrf"], qt.IsTrue)
	c.Assert(typedPinger.Close(), qt.IsNil)
}

func TestNewReadinessRedisPinger_DisabledLimitersAreExcluded(t *testing.T) {
	c := qt.New(t)

	cmd := &Command{
		config: Config{
			TokenBlacklistRedisURL:  "redis://localhost:6379/0",
			AuthRateLimitRedisURL:   "redis://localhost:6379/1",
			AuthRateLimitDisabled:   true,
			GlobalRateLimitRedisURL: "redis://localhost:6379/2",
			GlobalRateLimitDisabled: true,
			CSRFRedisURL:            "redis://localhost:6379/3",
		},
	}

	pinger := cmd.newReadinessRedisPinger()
	c.Assert(pinger, qt.IsNotNil)

	typedPinger, ok := pinger.(*readinessRedisPinger)
	c.Assert(ok, qt.IsTrue)
	c.Assert(typedPinger.targets, qt.HasLen, 2)

	targetNames := map[string]bool{}
	for _, target := range typedPinger.targets {
		targetNames[target.name] = true
	}

	c.Assert(targetNames["token_blacklist"], qt.IsTrue)
	c.Assert(targetNames["csrf"], qt.IsTrue)
	c.Assert(typedPinger.Close(), qt.IsNil)
}

func TestNewReadinessRedisPinger_AllInvalidURLsReturnsNil(t *testing.T) {
	c := qt.New(t)

	cmd := &Command{
		config: Config{
			TokenBlacklistRedisURL: "://invalid",
			CSRFRedisURL:           "://also-invalid",
		},
	}

	pinger := cmd.newReadinessRedisPinger()
	c.Assert(pinger, qt.IsNil)
}
