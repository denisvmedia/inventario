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

// A "/", "?" or "#" in the password ends the host part early, and the rest of
// the password, with the "@" of the userinfo, lands after the host.
func TestCheckDatabaseURL(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantErr string
	}{
		{name: "a / in the password", dsn: "postgres://app:12/tail@host/x", wantErr: urlerr.ErrAtAfterHost.Error()},
		{name: "a ? in the password", dsn: "postgres://app:12?tail@host/x", wantErr: urlerr.ErrAtAfterHost.Error()},
		{name: "a # in the password", dsn: "postgres://app:12#tail@host/x", wantErr: urlerr.ErrAtAfterHost.Error()},
		{name: "the postgresql scheme", dsn: "postgresql://app:12/tail@host/x", wantErr: urlerr.ErrAtAfterHost.Error()},
		{name: "an @ in a query value", dsn: "postgres://app:pw@host/x?application_name=a@b", wantErr: urlerr.ErrAtAfterHost.Error()},
		{name: "a DSN that does not parse", dsn: "postgres://app:p#ss@db/x", wantErr: "invalid port after host" + hint},
		{name: "an @ in the password", dsn: "postgres://app:p@ss@host/x"},
		{name: "a percent-encoded @ in the password", dsn: "postgres://app:p%40ss@host/x"},
		{name: "a percent-encoded @ in the database name", dsn: "postgres://app:pw@host/db%40x"},
		{name: "a percent-encoded @ in a query value", dsn: "postgres://app:pw@host/x?application_name=a%40b"},
		{name: "a keyword DSN", dsn: "host=db user=app password=12/tail@host dbname=x"},
		{name: "the memory registry", dsn: "memory://"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			err := urlerr.CheckDatabaseURL(tt.dsn)

			if tt.wantErr == "" {
				c.Assert(err, qt.IsNil)
				return
			}
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Equals, tt.wantErr)
		})
	}
}

func TestErrAtAfterHost(t *testing.T) {
	c := qt.New(t)

	c.Assert(urlerr.CheckDatabaseURL("postgres://app:12/tail@host/x"), qt.ErrorIs, urlerr.ErrAtAfterHost)
	c.Assert(urlerr.ErrAtAfterHost.Error(), qt.Equals,
		"the database URL has an @ after the host; percent-encode @ in the password or the database name as %40")
}
