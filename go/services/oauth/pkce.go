package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"

	errxtrace "github.com/go-extras/errx/stacktrace"
)

// pkceVerifierBytes is the number of raw random bytes used to generate a
// PKCE code verifier. 32 bytes → 43 base64-url chars, comfortably inside
// the [43, 128] range RFC 7636 allows for the `code_verifier` parameter
// and well above the 32-octet minimum the spec recommends.
const pkceVerifierBytes = 32

// PKCE bundles a freshly generated (verifier, challenge) pair for the
// authorization-code flow per RFC 7636. The Verifier is the secret the
// caller stores server-side (signed into the state cookie); the Challenge
// is the value that goes on the authorize URL alongside
// `code_challenge_method=S256`.
type PKCE struct {
	Verifier  string
	Challenge string
}

// NewPKCE generates a fresh PKCE pair using S256.
//
// Why S256 and not "plain": Inventario only supports modern providers
// (Google, GitHub) and the spec recommends S256 unless the platform cannot
// compute SHA-256; we always can. Hard-coding S256 also means the
// implementation never needs to remember the chosen method across the
// authorize → callback roundtrip.
func NewPKCE() (PKCE, error) {
	buf := make([]byte, pkceVerifierBytes)
	if _, err := rand.Read(buf); err != nil {
		return PKCE{}, errxtrace.Wrap("oauth: generate PKCE verifier", err)
	}
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return PKCE{Verifier: verifier, Challenge: challenge}, nil
}
