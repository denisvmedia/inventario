package urlerr_test

import (
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/urlerr"
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
			// "#" starts the fragment, and url.Parse reports the start of the
			// password as the port: `invalid port ":p" after host`.
			name:    "a # in the password",
			url:     "postgres://app:p#ss@db/x",
			wantErr: "invalid port after host" + hint,
		},
		{
			// url.Parse reports the escape: `invalid URL escape "%zz"`.
			name:    "a bare % in the password",
			url:     "postgres://app:s3cret%zz@db:5432/inventario",
			wantErr: "invalid URL escape" + hint,
		},
		{
			name:    "a character a host name cannot have",
			url:     "postgres://app:s3cret@d{b/x",
			wantErr: "invalid character in host name" + hint,
		},
		{
			name:    "an IP literal that is not an address",
			url:     "postgres://app:s3cret@[s3cret]/x",
			wantErr: "invalid host" + hint,
		},
		{
			name:    "a reason net/url gives as fixed text",
			url:     "://app:s3cret@db/x",
			wantErr: "missing protocol scheme" + hint,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			u, err := urlerr.Parse(tt.url)

			c.Assert(u, qt.IsNil)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Equals, tt.wantErr)
		})
	}
}

func TestParse_ValidURL(t *testing.T) {
	c := qt.New(t)

	u, err := urlerr.Parse("postgres://app:s3cret@db:5432/inventario")

	c.Assert(err, qt.IsNil)
	c.Assert(u.Host, qt.Equals, "db:5432")
}

func TestWithoutURL_OtherErrorPassesThrough(t *testing.T) {
	c := qt.New(t)

	other := errors.New("connection refused")

	c.Assert(urlerr.WithoutURL(other), qt.ErrorIs, other)
	c.Assert(urlerr.WithoutURL(nil), qt.IsNil)
}
