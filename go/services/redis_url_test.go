package services_test

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services"
)

// A Redis URL that does not parse falls back to in-memory, and the caller
// logs the error. That error must not carry the URL: it holds the password.
func TestRedisFromURL_ErrorLeavesThePasswordOut(t *testing.T) {
	const badURL = "redis://:s3cret-password@localhost:notaport/0"

	tests := []struct {
		name string
		open func() error
	}{
		{
			name: "token blacklist",
			open: func() error {
				_, err := services.NewRedisTokenBlacklisterFromURL(badURL)
				return err
			},
		},
		{
			name: "auth rate limiter",
			open: func() error {
				_, err := services.NewRedisAuthRateLimiterFromURL(badURL)
				return err
			},
		},
		{
			name: "global rate limiter",
			open: func() error {
				_, err := services.NewRedisGlobalRateLimiterFromURL(badURL, 10, time.Minute)
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			err := tt.open()

			c.Assert(err, qt.ErrorMatches, `invalid redis URL: invalid port ":notaport" after host`)
			c.Assert(err.Error(), qt.Not(qt.Contains), "s3cret-password")
		})
	}
}
