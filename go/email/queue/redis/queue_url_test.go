package redis_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	emailqueueredis "go.5x5.cz/inventario/email/queue/redis"
	"go.5x5.cz/inventario/internal/urlerr"
)

// The caller logs this error, so it must not carry the URL and its password.
func TestNewFromConfig_InvalidURLErrorLeavesThePasswordOut(t *testing.T) {
	c := qt.New(t)

	_, err := emailqueueredis.NewFromConfig(emailqueueredis.Config{
		RedisURL: "redis://:s3cret-password#tail@localhost:6379/0",
	})

	var urlErr *urlerr.Error
	c.Assert(err, qt.ErrorAs, &urlErr)
	c.Assert(urlErr.Reason, qt.Equals, "invalid port after host")
	c.Assert(err.Error(), qt.Not(qt.Contains), "s3cret")
}
