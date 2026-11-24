package models_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func TestGenerateAPIToken_Format(t *testing.T) {
	c := qt.New(t)

	gen, err := models.GenerateAPIToken()
	c.Assert(err, qt.IsNil)
	token, hash, prefix := gen.Token, gen.Hash, gen.DisplayPrefix

	c.Assert(strings.HasPrefix(token, models.APITokenPrefix), qt.IsTrue)
	c.Assert(token, qt.HasLen, models.APITokenLength)
	c.Assert(token, qt.Matches, `inv_pat_[0-9A-Za-z]{43}`)
	c.Assert(models.IsWellFormedAPIToken(token), qt.IsTrue)

	// The stored digest is the hex SHA-256 of the whole token.
	sum := sha256.Sum256([]byte(token))
	c.Assert(hash, qt.Equals, hex.EncodeToString(sum[:]))
	c.Assert(models.HashAPIToken(token), qt.Equals, hash)

	// The display prefix identifies the token without revealing enough of it
	// to matter: the type marker plus four characters.
	c.Assert(prefix, qt.Equals, token[:len(models.APITokenPrefix)+4])
}

// A JWT starts with "eyJ" (base64 of `{"`); the token prefix cannot be
// mistaken for one, and a JWT is never read as a token.
func TestLooksLikeAPIToken_DoesNotMatchJWTs(t *testing.T) {
	c := qt.New(t)

	c.Assert(models.LooksLikeAPIToken("eyJhbGciOiJIUzI1NiJ9.eyJ1c2VyX2lkIjoieCJ9.sig"), qt.IsFalse)
	c.Assert(models.LooksLikeAPIToken(""), qt.IsFalse)
	c.Assert(models.LooksLikeAPIToken("inv_pat_"), qt.IsTrue)
	c.Assert(models.LooksLikeAPIToken("inv_pat_short"), qt.IsTrue)
}

func TestIsWellFormedAPIToken(t *testing.T) {
	c := qt.New(t)
	gen, err := models.GenerateAPIToken()
	c.Assert(err, qt.IsNil)
	valid := gen.Token

	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"generated token", valid, true},
		{"prefix only", models.APITokenPrefix, false},
		{"one character short", valid[:len(valid)-1], false},
		{"one character long", valid + "A", false},
		{"non-base62 character", valid[:len(valid)-1] + "-", false},
		{"wrong prefix", "inv_xxx_" + valid[len(models.APITokenPrefix):], false},
		{"jwt", "eyJhbGciOiJIUzI1NiJ9.eyJ1c2VyX2lkIjoieCJ9.sig", false},
	}
	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			c.Assert(models.IsWellFormedAPIToken(tt.token), qt.Equals, tt.want)
		})
	}
}

// Every generated token is distinct and every character position draws from
// the whole alphabet. A biased or short random source shows up here long
// before it shows up anywhere else.
func TestGenerateAPIToken_IsRandom(t *testing.T) {
	c := qt.New(t)

	const n = 2000
	seen := make(map[string]struct{}, n)
	chars := make(map[rune]struct{})
	for range n {
		gen, err := models.GenerateAPIToken()
		c.Assert(err, qt.IsNil)
		token, hash := gen.Token, gen.Hash
		_, dup := seen[hash]
		c.Assert(dup, qt.IsFalse, qt.Commentf("duplicate token generated"))
		seen[hash] = struct{}{}
		for _, r := range token[len(models.APITokenPrefix):] {
			chars[r] = struct{}{}
		}
	}
	c.Assert(chars, qt.HasLen, 62)
}

func TestAPIToken_AllowsMethod(t *testing.T) {
	c := qt.New(t)

	readOnly := models.APIToken{Scope: models.APITokenScopeReadOnly}
	readWrite := models.APIToken{Scope: models.APITokenScopeReadWrite}

	for _, m := range []string{http.MethodGet, http.MethodHead, http.MethodOptions} {
		c.Assert(readOnly.AllowsMethod(m), qt.IsTrue, qt.Commentf("read-only %s", m))
		c.Assert(readWrite.AllowsMethod(m), qt.IsTrue, qt.Commentf("read-write %s", m))
	}
	for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete} {
		c.Assert(readOnly.AllowsMethod(m), qt.IsFalse, qt.Commentf("read-only %s", m))
		c.Assert(readWrite.AllowsMethod(m), qt.IsTrue, qt.Commentf("read-write %s", m))
	}

	// An unknown scope allows nothing.
	unknown := models.APIToken{Scope: "admin"}
	c.Assert(unknown.AllowsMethod(http.MethodGet), qt.IsFalse)
}

func TestAPIToken_Usability(t *testing.T) {
	c := qt.New(t)
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	noExpiry := models.APIToken{}
	c.Assert(noExpiry.IsExpired(now), qt.IsFalse)
	c.Assert(noExpiry.IsUsable(now), qt.IsTrue)

	future := models.APIToken{ExpiresAt: new(now.Add(time.Hour))}
	c.Assert(future.IsUsable(now), qt.IsTrue)

	past := models.APIToken{ExpiresAt: new(now.Add(-time.Second))}
	c.Assert(past.IsExpired(now), qt.IsTrue)
	c.Assert(past.IsUsable(now), qt.IsFalse)

	exact := models.APIToken{ExpiresAt: new(now)}
	c.Assert(exact.IsExpired(now), qt.IsTrue, qt.Commentf("a token is expired at its expiry instant"))

	revoked := models.APIToken{RevokedAt: new(now.Add(-time.Minute))}
	c.Assert(revoked.IsRevoked(), qt.IsTrue)
	c.Assert(revoked.IsUsable(now), qt.IsFalse)
}
