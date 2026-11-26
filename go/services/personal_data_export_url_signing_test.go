package services_test

import (
	"net/url"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/services"
)

func TestPersonalDataExportURL_RoundTrip(t *testing.T) {
	c := qt.New(t)
	signer := services.NewFileSigningService([]byte("0123456789abcdef0123456789abcdef"), time.Minute)

	signed, expiresAt, err := signer.GeneratePersonalDataExportURL("export-1", "user-1", "binding")
	c.Assert(err, qt.IsNil)
	c.Assert(expiresAt.After(time.Now()), qt.IsTrue)
	u, err := url.Parse(signed)
	c.Assert(err, qt.IsNil)
	c.Assert(u.Path, qt.Equals, services.PersonalDataExportURLPrefix+"export-1")

	claims, err := signer.ValidatePersonalDataExportURL(u.Path, u.Query(), "binding")
	c.Assert(err, qt.IsNil)
	c.Assert(claims.ExportID, qt.Equals, "export-1")
	c.Assert(claims.UserID, qt.Equals, "user-1")

	tests := []struct {
		name    string
		path    string
		query   func() url.Values
		binding services.SessionBinding
	}{
		{name: "another session", path: u.Path, query: u.Query, binding: "other"},
		{name: "another export", path: services.PersonalDataExportURLPrefix + "export-2", query: u.Query, binding: "binding"},
		{name: "another user", path: u.Path, query: func() url.Values {
			q := u.Query()
			q.Set("uid", "user-2")
			return q
		}, binding: "binding"},
		{name: "extended expiry", path: u.Path, query: func() url.Values {
			q := u.Query()
			q.Set("exp", "99999999999")
			return q
		}, binding: "binding"},
		{name: "not this route", path: "/api/v1/avatars/export-1", query: u.Query, binding: "binding"},
	}
	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			_, err := signer.ValidatePersonalDataExportURL(tt.path, tt.query(), tt.binding)
			c.Assert(err, qt.IsNotNil)
		})
	}
}

func TestPersonalDataExportURL_Expires(t *testing.T) {
	c := qt.New(t)
	signer := services.NewFileSigningService([]byte("0123456789abcdef0123456789abcdef"), -time.Minute)

	signed, _, err := signer.GeneratePersonalDataExportURL("export-1", "user-1", "")
	c.Assert(err, qt.IsNil)
	u, err := url.Parse(signed)
	c.Assert(err, qt.IsNil)
	_, err = signer.ValidatePersonalDataExportURL(u.Path, u.Query(), "")
	c.Assert(err, qt.ErrorMatches, ".*expired.*")
}
