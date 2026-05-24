package oauth_test

import (
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/services/oauth"
)

func newKey(t *testing.T) []byte {
	t.Helper()
	// Deterministic 32-byte key for reproducibility — the tests don't
	// need entropy, they need predictable inputs.
	return []byte("0123456789abcdef0123456789abcdef")
}

func TestNewStateSigner_RejectsShortKey(t *testing.T) {
	c := qt.New(t)
	_, err := oauth.NewStateSigner([]byte("too short"))
	c.Assert(err, qt.IsNotNil)
}

func TestStateSigner_RoundTrip(t *testing.T) {
	c := qt.New(t)

	signer, err := oauth.NewStateSigner(newKey(t))
	c.Assert(err, qt.IsNil)

	nonce, err := oauth.NewNonce()
	c.Assert(err, qt.IsNil)

	original := oauth.State{
		Provider:      "google",
		Nonce:         nonce,
		Verifier:      "verifier-abc-123-some-random-string-padding",
		RedirectAfter: "/dashboard",
	}

	token, err := signer.Sign(original)
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")

	decoded, err := signer.Verify(token)
	c.Assert(err, qt.IsNil)
	c.Assert(decoded.Provider, qt.Equals, original.Provider)
	c.Assert(decoded.Nonce, qt.Equals, original.Nonce)
	c.Assert(decoded.Verifier, qt.Equals, original.Verifier)
	c.Assert(decoded.RedirectAfter, qt.Equals, original.RedirectAfter)
	c.Assert(decoded.IssuedAt, qt.Not(qt.Equals), int64(0))
	c.Assert(decoded.ExpiresAt, qt.Not(qt.Equals), int64(0))
}

func TestStateSigner_RejectsMissingNonceOrVerifier(t *testing.T) {
	c := qt.New(t)
	signer, err := oauth.NewStateSigner(newKey(t))
	c.Assert(err, qt.IsNil)

	_, err = signer.Sign(oauth.State{Verifier: "v"})
	c.Assert(err, qt.IsNotNil)
	_, err = signer.Sign(oauth.State{Nonce: "n"})
	c.Assert(err, qt.IsNotNil)
}

// TestStateSigner_TamperDetection pins that a single bit-flip on either
// the payload or the signature surfaces ErrStateInvalid.
func TestStateSigner_TamperDetection(t *testing.T) {
	c := qt.New(t)

	signer, err := oauth.NewStateSigner(newKey(t))
	c.Assert(err, qt.IsNil)
	nonce, err := oauth.NewNonce()
	c.Assert(err, qt.IsNil)

	token, err := signer.Sign(oauth.State{Nonce: nonce, Verifier: "v-value-padding"})
	c.Assert(err, qt.IsNil)

	parts := strings.Split(token, ".")
	c.Assert(parts, qt.HasLen, 2)

	// Flip a middle base64 char rather than the last. The trailing chars
	// of a RawURLEncoding string often only carry a partial byte of the
	// encoded payload, so flipping them can be a no-op after decoding
	// (the classic base64 "unused trailing bits" gotcha). A mid-string
	// flip guarantees the decoded byte sequence changes.
	tampered := flipMiddleChar(parts[0]) + "." + parts[1]
	_, err = signer.Verify(tampered)
	c.Assert(err, qt.Equals, oauth.ErrStateInvalid)

	tamperedSig := parts[0] + "." + flipMiddleChar(parts[1])
	_, err = signer.Verify(tamperedSig)
	c.Assert(err, qt.Equals, oauth.ErrStateInvalid)
}

func flipMiddleChar(s string) string {
	if len(s) < 2 {
		return s
	}
	b := []byte(s)
	mid := len(b) / 2
	switch b[mid] {
	case 'A':
		b[mid] = 'B'
	default:
		b[mid] = 'A'
	}
	return string(b)
}

// TestStateSigner_RejectsExpiredToken pins that a token whose ExpiresAt
// is in the past surfaces ErrStateInvalid.
func TestStateSigner_RejectsExpiredToken(t *testing.T) {
	c := qt.New(t)

	signer, err := oauth.NewStateSigner(newKey(t))
	c.Assert(err, qt.IsNil)

	nonce, err := oauth.NewNonce()
	c.Assert(err, qt.IsNil)

	expired := oauth.State{
		Nonce:     nonce,
		Verifier:  "verifier-padded-out-to-look-realistic",
		ExpiresAt: time.Now().Add(-time.Minute).Unix(),
	}
	token, err := signer.Sign(expired)
	c.Assert(err, qt.IsNil)

	_, err = signer.Verify(token)
	c.Assert(err, qt.Equals, oauth.ErrStateInvalid)
}

// TestStateSigner_RejectsBadEncoding pins that a token with garbled
// structure (wrong number of parts, non-base64 chars) is rejected.
func TestStateSigner_RejectsBadEncoding(t *testing.T) {
	c := qt.New(t)

	signer, err := oauth.NewStateSigner(newKey(t))
	c.Assert(err, qt.IsNil)

	cases := []string{
		"",
		"not-base64-payload.not-base64-sig",
		"missing-separator",
		"too.many.dots",
	}
	for _, raw := range cases {
		_, err := signer.Verify(raw)
		c.Assert(err, qt.Equals, oauth.ErrStateInvalid, qt.Commentf("input: %q", raw))
	}
}

func TestStateSigner_RejectsForeignKey(t *testing.T) {
	c := qt.New(t)

	signerA, err := oauth.NewStateSigner(newKey(t))
	c.Assert(err, qt.IsNil)
	signerB, err := oauth.NewStateSigner([]byte("ffffffffffffffffffffffffffffffff"))
	c.Assert(err, qt.IsNil)

	nonce, err := oauth.NewNonce()
	c.Assert(err, qt.IsNil)
	token, err := signerA.Sign(oauth.State{Nonce: nonce, Verifier: "v-padded-value"})
	c.Assert(err, qt.IsNil)

	_, err = signerB.Verify(token)
	c.Assert(err, qt.Equals, oauth.ErrStateInvalid)
}

func TestSanitizeRedirect(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"", ""},
		{"/", "/"},
		{"/dashboard", "/dashboard"},
		{"/settings?tab=security", "/settings?tab=security"},
		// Protocol-relative URL — must be rejected.
		{"//evil.com/path", ""},
		// Absolute URL — must be rejected.
		{"https://evil.com/", ""},
		// Missing leading slash — rejected to avoid relative-redirect ambiguity.
		{"dashboard", ""},
		// Path traversal — collapses to a safe absolute path.
		{"/foo/../bar", "/bar"},
	}
	c := qt.New(t)
	for _, tc := range cases {
		got := oauth.SanitizeRedirect(tc.raw)
		c.Assert(got, qt.Equals, tc.want, qt.Commentf("input: %q", tc.raw))
	}
}
