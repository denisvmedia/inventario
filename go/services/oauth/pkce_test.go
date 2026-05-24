package oauth_test

import (
	"crypto/sha256"
	"encoding/base64"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/services/oauth"
)

func TestNewPKCE_S256Roundtrip(t *testing.T) {
	c := qt.New(t)

	pkce, err := oauth.NewPKCE()
	c.Assert(err, qt.IsNil)
	c.Assert(pkce.Verifier, qt.Not(qt.Equals), "")
	c.Assert(pkce.Challenge, qt.Not(qt.Equals), "")

	// Recompute the challenge ourselves and compare. This is what the
	// provider does on its side to verify the code_verifier matches.
	sum := sha256.Sum256([]byte(pkce.Verifier))
	expected := base64.RawURLEncoding.EncodeToString(sum[:])
	c.Assert(pkce.Challenge, qt.Equals, expected)
}

func TestNewPKCE_NonRepeating(t *testing.T) {
	c := qt.New(t)
	a, err := oauth.NewPKCE()
	c.Assert(err, qt.IsNil)
	b, err := oauth.NewPKCE()
	c.Assert(err, qt.IsNil)
	c.Assert(a.Verifier, qt.Not(qt.Equals), b.Verifier)
	c.Assert(a.Challenge, qt.Not(qt.Equals), b.Challenge)
}
