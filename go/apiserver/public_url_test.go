package apiserver

import (
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestBuildPublicURL_UsesConfiguredBaseWhenSchemeIsAllowed(t *testing.T) {
	c := qt.New(t)

	values := url.Values{"token": {"abc"}}
	got, err := buildPublicURL("https://inventario.example.com", "/verify-email", values)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.Equals, "https://inventario.example.com/verify-email?token=abc")
}

func TestBuildPublicURL_ReturnsErrorWhenConfiguredBaseSchemeIsUnsupported(t *testing.T) {
	c := qt.New(t)

	values := url.Values{"token": {"abc"}}
	got, err := buildPublicURL("ftp://inventario.example.com", "/verify-email", values)
	c.Assert(err, qt.IsNotNil)
	c.Assert(got, qt.Equals, "")
}

func TestBuildPublicURL_ReturnsErrorWhenPublicBaseURLMissing(t *testing.T) {
	c := qt.New(t)

	values := url.Values{"token": {"abc"}}
	got, err := buildPublicURL("", "/verify-email", values)
	c.Assert(err, qt.IsNotNil)
	c.Assert(got, qt.Equals, "")
}

func TestBuildPublicURL_ReturnsErrorWhenPublicBaseURLMissingSchemeOrHost(t *testing.T) {
	c := qt.New(t)

	values := url.Values{"token": {"abc"}}
	got, err := buildPublicURL("inventario.example.com", "/verify-email", values)
	c.Assert(err, qt.IsNotNil)
	c.Assert(got, qt.Equals, "")
}
