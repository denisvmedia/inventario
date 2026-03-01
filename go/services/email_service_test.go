package services

import (
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestRedactTokenFromURLForLogs_RemovesTokenQuery(t *testing.T) {
	t.Run("absolute url", func(t *testing.T) {
		c := qt.New(t)

		redacted := redactTokenFromURLForLogs("https://example.com/verify-email?token=secret&lang=en#frag")
		parsed, err := url.Parse(redacted)
		c.Assert(err, qt.IsNil)
		c.Assert(parsed.Fragment, qt.Equals, "")
		c.Assert(parsed.Query().Has("token"), qt.IsFalse)
		c.Assert(parsed.Query().Get("lang"), qt.Equals, "en")
	})

	t.Run("relative url", func(t *testing.T) {
		c := qt.New(t)

		redacted := redactTokenFromURLForLogs("/reset-password?token=secret")
		parsed, err := url.Parse(redacted)
		c.Assert(err, qt.IsNil)
		c.Assert(parsed.Query().Has("token"), qt.IsFalse)
		c.Assert(parsed.Path, qt.Equals, "/reset-password")
	})
}
