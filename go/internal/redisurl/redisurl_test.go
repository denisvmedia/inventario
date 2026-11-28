package redisurl_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/redisurl"
)

func TestParse_ErrorLeavesTheURLOut(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantCause string
	}{
		{
			name:      "url.Parse error",
			url:       "redis://:s3cret-password@localhost:notaport/0",
			wantCause: `invalid port ":notaport" after host`,
		},
		{
			name:      "go-redis error",
			url:       "http://:s3cret-password@localhost:6379/0",
			wantCause: "redis: invalid URL scheme: http",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			opts, err := redisurl.Parse(tt.url)

			c.Assert(opts, qt.IsNil)
			c.Assert(err, qt.ErrorMatches, tt.wantCause)
			c.Assert(err.Error(), qt.Not(qt.Contains), "s3cret-password")
		})
	}
}

func TestParse_ValidURL(t *testing.T) {
	c := qt.New(t)

	opts, err := redisurl.Parse("redis://user:s3cret-password@localhost:6380/2")

	c.Assert(err, qt.IsNil)
	c.Assert(opts.Addr, qt.Equals, "localhost:6380")
	c.Assert(opts.Username, qt.Equals, "user")
	c.Assert(opts.Password, qt.Equals, "s3cret-password")
	c.Assert(opts.DB, qt.Equals, 2)
}
