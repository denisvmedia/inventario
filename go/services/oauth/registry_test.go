package oauth_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/oauth"
)

// stubProvider lets tests exercise the Registry without going near
// httptest.Server / the OAuth2 dance.
type stubProvider struct {
	name models.OAuthProvider
}

func (s *stubProvider) Name() models.OAuthProvider   { return s.name }
func (*stubProvider) AuthCodeURL(_, _ string) string { return "" }
func (*stubProvider) Exchange(_ context.Context, _, _ string) (oauth.Profile, error) {
	return oauth.Profile{}, errors.New("stub")
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	c := qt.New(t)
	r := oauth.NewRegistry()
	c.Assert(r.Enabled(), qt.HasLen, 0)

	err := r.Register(&stubProvider{name: models.OAuthProviderGoogle})
	c.Assert(err, qt.IsNil)
	err = r.Register(&stubProvider{name: models.OAuthProviderGitHub})
	c.Assert(err, qt.IsNil)

	enabled := r.Enabled()
	c.Assert(enabled, qt.DeepEquals, []models.OAuthProvider{
		models.OAuthProviderGoogle,
		models.OAuthProviderGitHub,
	})

	p, ok := r.Get(models.OAuthProviderGoogle)
	c.Assert(ok, qt.IsTrue)
	c.Assert(p.Name(), qt.Equals, models.OAuthProviderGoogle)

	_, ok = r.Get(models.OAuthProvider("twitter"))
	c.Assert(ok, qt.IsFalse)

	c.Assert(r.Has(models.OAuthProviderGitHub), qt.IsTrue)
	c.Assert(r.Has(models.OAuthProvider("twitter")), qt.IsFalse)
}

func TestRegistry_RegisterRejectsNilProvider(t *testing.T) {
	c := qt.New(t)
	r := oauth.NewRegistry()
	err := r.Register(nil)
	c.Assert(err, qt.IsNotNil)
}

func TestRegistry_RegisterRejectsInvalidName(t *testing.T) {
	c := qt.New(t)
	r := oauth.NewRegistry()
	err := r.Register(&stubProvider{name: models.OAuthProvider("twitter")})
	c.Assert(err, qt.IsNotNil)
}

// TestRegistry_ReregisterReplaces pins the contract: registering twice
// under the same name overwrites without growing Enabled(). Tests use this
// to inject a stub Provider.
func TestRegistry_ReregisterReplaces(t *testing.T) {
	c := qt.New(t)
	r := oauth.NewRegistry()

	first := &stubProvider{name: models.OAuthProviderGoogle}
	c.Assert(r.Register(first), qt.IsNil)

	second := &stubProvider{name: models.OAuthProviderGoogle}
	c.Assert(r.Register(second), qt.IsNil)

	c.Assert(r.Enabled(), qt.HasLen, 1)
	p, ok := r.Get(models.OAuthProviderGoogle)
	c.Assert(ok, qt.IsTrue)
	c.Assert(p, qt.Equals, second)
}

// TestRegistry_NilSafe pins that a nil *Registry tolerates the read
// methods — handlers can pass through a nil registry when OAuth is
// entirely unconfigured in the deployment.
func TestRegistry_NilSafe(t *testing.T) {
	c := qt.New(t)
	var r *oauth.Registry
	c.Assert(r.Enabled(), qt.HasLen, 0)
	c.Assert(r.Has(models.OAuthProviderGoogle), qt.IsFalse)
	_, ok := r.Get(models.OAuthProviderGoogle)
	c.Assert(ok, qt.IsFalse)
}
