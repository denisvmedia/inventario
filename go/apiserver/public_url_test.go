package apiserver

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestBuildPublicURL_UsesConfiguredBaseWhenSchemeIsAllowed(t *testing.T) {
	c := qt.New(t)

	req := httptest.NewRequest(http.MethodGet, "http://internal.local/ignored", nil)
	values := url.Values{"token": {"abc"}}

	got := buildPublicURL("https://inventario.example.com", req, "/verify-email", values)
	c.Assert(got, qt.Equals, "https://inventario.example.com/verify-email?token=abc")
}

func TestBuildPublicURL_FallsBackWhenConfiguredBaseSchemeIsUnsupported(t *testing.T) {
	c := qt.New(t)

	req := httptest.NewRequest(http.MethodGet, "http://app.local/ignored", nil)
	values := url.Values{"token": {"abc"}}

	got := buildPublicURL("ftp://inventario.example.com", req, "/verify-email", values)
	c.Assert(got, qt.Equals, "http://app.local/verify-email?token=abc")
}

func TestBuildPublicURL_IgnoresUnsupportedForwardedProto(t *testing.T) {
	c := qt.New(t)

	req := httptest.NewRequest(http.MethodGet, "http://app.local/ignored", nil)
	req.Header.Set("X-Forwarded-Proto", "javascript")
	values := url.Values{"token": {"abc"}}

	got := buildPublicURL("", req, "/verify-email", values)
	c.Assert(got, qt.Equals, "http://app.local/verify-email?token=abc")
}

func TestBuildPublicURL_UsesFirstForwardedProtoValue(t *testing.T) {
	c := qt.New(t)

	req := httptest.NewRequest(http.MethodGet, "http://app.local/ignored", nil)
	req.Header.Set("X-Forwarded-Proto", "https, http")
	values := url.Values{"token": {"abc"}}

	got := buildPublicURL("", req, "/verify-email", values)
	c.Assert(got, qt.Equals, "https://app.local/verify-email?token=abc")
}
