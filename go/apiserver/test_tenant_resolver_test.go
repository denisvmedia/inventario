package apiserver_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
)

type stubResolver struct {
	slug string
	err  error
}

func (s *stubResolver) ResolveTenant(_ *http.Request) (string, error) {
	return s.slug, s.err
}

func TestTestHeaderTenantResolver_HeaderWins(t *testing.T) {
	c := qt.New(t)
	r := &apiserver.TestHeaderTenantResolver{Inner: &stubResolver{slug: "from-host"}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(apiserver.TestTenantHeaderName, "tenant-from-header")

	slug, err := r.ResolveTenant(req)
	c.Assert(err, qt.IsNil)
	c.Assert(slug, qt.Equals, "tenant-from-header")
}

func TestTestHeaderTenantResolver_FallsBackToInner(t *testing.T) {
	c := qt.New(t)
	r := &apiserver.TestHeaderTenantResolver{Inner: &stubResolver{slug: "from-host"}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	// No header set.

	slug, err := r.ResolveTenant(req)
	c.Assert(err, qt.IsNil)
	c.Assert(slug, qt.Equals, "from-host")
}

func TestTestHeaderTenantResolver_EmptyHeaderFallsBack(t *testing.T) {
	c := qt.New(t)
	r := &apiserver.TestHeaderTenantResolver{Inner: &stubResolver{slug: "from-host"}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(apiserver.TestTenantHeaderName, "")

	slug, err := r.ResolveTenant(req)
	c.Assert(err, qt.IsNil)
	c.Assert(slug, qt.Equals, "from-host")
}

func TestTestHeaderTenantResolver_WhitespaceOnlyHeaderFallsBack(t *testing.T) {
	c := qt.New(t)
	r := &apiserver.TestHeaderTenantResolver{Inner: &stubResolver{slug: "from-host"}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(apiserver.TestTenantHeaderName, "  \t  ")

	slug, err := r.ResolveTenant(req)
	c.Assert(err, qt.IsNil)
	c.Assert(slug, qt.Equals, "from-host",
		qt.Commentf("whitespace-only header must NOT short-circuit Inner with an empty slug"))
}

func TestTestHeaderTenantResolver_TrimsSurroundingWhitespace(t *testing.T) {
	c := qt.New(t)
	r := &apiserver.TestHeaderTenantResolver{Inner: &stubResolver{slug: "from-host"}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(apiserver.TestTenantHeaderName, "  tenant-from-header  ")

	slug, err := r.ResolveTenant(req)
	c.Assert(err, qt.IsNil)
	c.Assert(slug, qt.Equals, "tenant-from-header")
}

func TestTestHeaderTenantResolver_NilInnerNoHeaderReturnsEmpty(t *testing.T) {
	c := qt.New(t)
	r := &apiserver.TestHeaderTenantResolver{}

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	slug, err := r.ResolveTenant(req)
	c.Assert(err, qt.IsNil)
	c.Assert(slug, qt.Equals, "")
}

func TestTestHeaderTenantResolver_InnerErrorPropagates(t *testing.T) {
	c := qt.New(t)
	innerErr := errors.New("inner-failed")
	r := &apiserver.TestHeaderTenantResolver{Inner: &stubResolver{err: innerErr}}

	req := httptest.NewRequest(http.MethodGet, "/", nil)

	_, err := r.ResolveTenant(req)
	c.Assert(err, qt.ErrorIs, innerErr)
}
