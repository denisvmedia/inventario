package redisurl_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/redisurl"
)

const hint = " (percent-encode the characters a URL reserves in the user name and password, such as @ : / ? # and %)"

// Each error is fixed text, so no character of the URL, the password
// included, can be in it.
func TestParse_ErrorQuotesNothingFromTheURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{
			// "#" starts the fragment, and url.Parse reports the password as
			// the port: `invalid port ":s3cret-password" after host`.
			name:    "a # in the password",
			url:     "redis://:s3cret-password#tail@localhost:6379/0",
			wantErr: "invalid port after host" + hint,
		},
		{
			// "/" ends the host, and go-redis quotes the path, which holds the
			// rest of the password: `redis: invalid URL path: /secret-tail@…`.
			name:    "a / in the password",
			url:     "redis://:123/secret-tail@localhost:6379/0",
			wantErr: "redis: invalid URL path, expected /<database number>" + hint,
		},
		{
			name:    "a / in the password with one path segment after it",
			url:     "redis://:123/secret-tail@localhost",
			wantErr: "redis: invalid database number" + hint,
		},
		{
			name:    "a ? in the password",
			url:     "redis://:123?secret-tail@localhost/0",
			wantErr: "redis: unexpected query option" + hint,
		},
		{
			name:    "an option value go-redis cannot read",
			url:     "redis://:s3cret@localhost/0?dial_timeout=s3cret",
			wantErr: "redis: invalid query option value" + hint,
		},
		{
			name:    "a scheme go-redis does not know",
			url:     "s3cret://:s3cret@localhost:6379/0",
			wantErr: "redis: invalid URL scheme, expected redis, rediss or unix" + hint,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			opts, err := redisurl.Parse(tt.url)

			c.Assert(opts, qt.IsNil)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Equals, tt.wantErr)
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
