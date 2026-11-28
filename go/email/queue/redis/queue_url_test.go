package redis_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	emailqueueredis "go.5x5.cz/inventario/email/queue/redis"
)

// The caller logs this error, so it must not carry the URL and its password.
func TestNewFromConfig_InvalidURLErrorLeavesThePasswordOut(t *testing.T) {
	c := qt.New(t)

	_, err := emailqueueredis.NewFromConfig(emailqueueredis.Config{
		RedisURL: "redis://:s3cret-password@localhost:notaport/0",
	})

	c.Assert(err, qt.ErrorMatches, `invalid redis URL: invalid port ":notaport" after host`)
	c.Assert(err.Error(), qt.Not(qt.Contains), "s3cret-password")
}
