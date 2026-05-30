// Package backupsign is the single source of truth for signing and verifying
// Inventario `.inb` backup archives.
//
// Backups must be signed so that a backup cannot be forged outside the system
// (issue #534). The private key is loaded the same way as the other server
// secrets (see cmd/inventario/run/bootstrap/crypto.go) and is NEVER exposed to
// users — only the derived public key (and its fingerprint) may be surfaced, so
// external tooling can verify a backup without being able to mint one.
//
// # Signing scheme
//
// To keep memory bounded for arbitrarily large backups, the payload is NOT held
// in memory and signed as one buffer. Instead the canonical signed value is the
// streaming SHA-256 digest of the compressed payload (`payload.tar.gz`), and the
// Ed25519 signature is computed over that 32-byte digest:
//
//	signature = Ed25519_sign(privKey, SHA256(payload.tar.gz))
//
// This "hash-then-sign" construction lets both the exporter and the restorer
// stream the payload through a hasher (constant memory). External verification
// reproduces it trivially: SHA-256 the `payload.tar.gz` member, then Ed25519
// verify the signature over that digest with the published public key. The
// algorithm identifier surfaced in the manifest and the public-key endpoint is
// therefore Algorithm ("ed25519-sha256"), not bare "ed25519".
//
// Verification on restore ALWAYS uses the server's own configured key, never a
// key read from the archive; a public key embedded in a manifest is purely
// informational.
package backupsign

import (
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"hash"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
)

// SeedSize is the length of the Ed25519 seed the Signer is constructed from
// (32 bytes). Re-exported so callers (config loaders, the resign CLI) can
// validate input without importing crypto/ed25519.
const SeedSize = ed25519.SeedSize

// DigestSize is the length of the canonical payload digest (SHA-256, 32 bytes).
const DigestSize = sha256.Size

// Algorithm is the canonical algorithm identifier surfaced in the manifest and
// the public-key endpoint. It names the hash-then-sign construction so external
// verifiers know to SHA-256 the payload before the Ed25519 check.
const Algorithm = "ed25519-sha256"

var (
	// ErrInvalidSeedSize is returned by NewSigner when the seed is not exactly
	// SeedSize bytes. Ed25519 derives the key pair from a fixed-length seed, so
	// a shorter/longer value is rejected rather than truncated or padded.
	ErrInvalidSeedSize = errx.NewSentinel("backup signing seed must be exactly 32 bytes")

	// ErrBadSignature is returned by the verify methods when the signature does
	// not match the digest under the Signer's public key. It is the single
	// sentinel the restore path keys its hard-refusal on.
	ErrBadSignature = errx.NewSentinel("backup signature verification failed")
)

// Signer signs and verifies backup payload digests with an Ed25519 key pair
// derived from a 32-byte seed. It is safe for concurrent use: it holds only
// immutable key material and the crypto operations are stateless.
type Signer struct {
	priv ed25519.PrivateKey
	pub  ed25519.PublicKey
}

// NewSigner derives a Signer from a 32-byte Ed25519 seed. The seed is the
// secret; both the private and public keys are computed from it.
func NewSigner(seed []byte) (*Signer, error) {
	if len(seed) != ed25519.SeedSize {
		return nil, errx.Classify(ErrInvalidSeedSize, errx.Attrs("got", len(seed), "want", ed25519.SeedSize))
	}
	priv := ed25519.NewKeyFromSeed(seed)
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		// Unreachable: ed25519.PrivateKey.Public always returns ed25519.PublicKey.
		return nil, errx.NewSentinel("backup signing key derivation produced an unexpected public key type")
	}
	return &Signer{priv: priv, pub: pub}, nil
}

// NewDigest returns the canonical streaming hash (SHA-256) the payload is
// digested with before signing. Stream the payload through it — e.g. via
// io.MultiWriter when producing the payload, or io.TeeReader when consuming it —
// then pass Sum(nil) to SignDigest / VerifyDigest. Using this constructor keeps
// the hash choice in one place.
func NewDigest() hash.Hash {
	return sha256.New()
}

// SignDigest returns the detached Ed25519 signature over a payload digest
// produced by NewDigest. The digest is expected to be DigestSize bytes; the
// signature is what gets written as the `payload.tar.gz.sig` archive member.
func (s *Signer) SignDigest(digest []byte) []byte {
	return ed25519.Sign(s.priv, digest)
}

// VerifyDigest reports whether sig is a valid Ed25519 signature over digest
// under this Signer's public key. It returns ErrBadSignature on mismatch
// (including a wrong-length signature) and nil on success.
func (s *Signer) VerifyDigest(digest, sig []byte) error {
	if !ed25519.Verify(s.pub, digest, sig) {
		return ErrBadSignature
	}
	return nil
}

// Sign is an in-memory convenience that digests message with NewDigest and
// signs it. Prefer the streaming NewDigest + SignDigest path for large payloads.
func (s *Signer) Sign(message []byte) []byte {
	h := s.digestOf(message)
	return s.SignDigest(h)
}

// Verify is the in-memory counterpart of Sign: it digests message and verifies
// sig over the digest, returning ErrBadSignature on mismatch.
func (s *Signer) Verify(message, sig []byte) error {
	return s.VerifyDigest(s.digestOf(message), sig)
}

func (s *Signer) digestOf(message []byte) []byte {
	h := NewDigest()
	_, _ = h.Write(message) // hash.Hash.Write never returns an error
	return h.Sum(nil)
}

// PublicKey returns a defensive copy of the raw Ed25519 public key. A copy is
// returned (not the internal slice) so a caller can never mutate the Signer's
// key material; the key is meant to be immutable for the Signer's lifetime.
func (s *Signer) PublicKey() ed25519.PublicKey {
	return append(ed25519.PublicKey(nil), s.pub...)
}

// PublicKeyBase64 returns the raw 32-byte public key, base64 (std) encoded.
// This is the compact form embedded in the manifest's signature block.
func (s *Signer) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(s.pub)
}

// PublicKeyPEM returns the public key as a PKIX-encoded PEM block ("PUBLIC
// KEY"), the form most external verification tooling (openssl, Go's
// x509.ParsePKIXPublicKey) expects. It is surfaced by the public-key endpoint.
func (s *Signer) PublicKeyPEM() ([]byte, error) {
	der, err := x509.MarshalPKIXPublicKey(s.pub)
	if err != nil {
		return nil, errxtrace.Wrap("failed to marshal backup public key", err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}), nil
}

// Fingerprint returns the lowercase hex SHA-256 of the raw public key. It is a
// stable, short identifier for the signing key — embedded in the manifest and
// returned by the public-key endpoint so operators can tell at a glance which
// key signed a given backup (useful around key rotation).
func (s *Signer) Fingerprint() string {
	sum := sha256.Sum256(s.pub)
	return hex.EncodeToString(sum[:])
}

// VerifyDigestWithPublicKey verifies a detached signature over a payload digest
// against a raw Ed25519 public key, WITHOUT needing the private key (issue
// #534). It exists for the `inventario backup resign --verify-key` flow, where
// an operator wants to confirm an archive's existing signature matches a
// specific published key before re-signing under the server's current key.
//
// It returns ErrBadSignature on mismatch (including a wrong-length signature or
// public key) and nil on success — the same contract as (*Signer).VerifyDigest.
func VerifyDigestWithPublicKey(pub ed25519.PublicKey, digest, sig []byte) error {
	if len(pub) != ed25519.PublicKeySize {
		return ErrBadSignature
	}
	if !ed25519.Verify(pub, digest, sig) {
		return ErrBadSignature
	}
	return nil
}

// ParsePublicKey decodes an Ed25519 public key from any of the forms the
// resign CLI accepts (issue #534): a PEM "PUBLIC KEY" block (PKIX, the form
// PublicKeyPEM emits), a standard-base64 raw key (the form PublicKeyBase64
// emits), or a hex-encoded raw key. It is the inverse of the Signer's
// public-key accessors and feeds VerifyDigestWithPublicKey.
func ParsePublicKey(data []byte) (ed25519.PublicKey, error) {
	trimmed := bytesTrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errx.NewSentinel("empty public key input")
	}

	// 1. PEM block ("PUBLIC KEY", PKIX-encoded).
	if block, _ := pem.Decode(trimmed); block != nil {
		pub, err := parsePKIXEd25519(block.Bytes)
		if err != nil {
			return nil, err
		}
		return pub, nil
	}

	s := string(trimmed)

	// 2. Hex-encoded raw 32-byte key.
	if raw, err := hex.DecodeString(s); err == nil && len(raw) == ed25519.PublicKeySize {
		return ed25519.PublicKey(raw), nil
	}

	// 3. Standard-base64 raw 32-byte key (PublicKeyBase64 form).
	if raw, err := base64.StdEncoding.DecodeString(s); err == nil && len(raw) == ed25519.PublicKeySize {
		return ed25519.PublicKey(raw), nil
	}

	// 4. Raw DER (PKIX) bytes without a PEM envelope.
	if pub, err := parsePKIXEd25519(trimmed); err == nil {
		return pub, nil
	}

	return nil, errx.NewSentinel("public key is not a recognized PEM, base64, or hex Ed25519 key")
}

// parsePKIXEd25519 parses PKIX DER bytes into an Ed25519 public key,
// rejecting any other key type.
func parsePKIXEd25519(der []byte) (ed25519.PublicKey, error) {
	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, errxtrace.Wrap("failed to parse PKIX public key", err)
	}
	pub, ok := parsed.(ed25519.PublicKey)
	if !ok {
		return nil, errx.NewSentinel("public key is not an Ed25519 key")
	}
	return pub, nil
}

// bytesTrimSpace trims leading/trailing ASCII whitespace from a byte slice
// without an extra strings round-trip (keeps the PEM bytes intact for
// pem.Decode while tolerating a trailing newline on a hex/base64 file).
func bytesTrimSpace(b []byte) []byte {
	start := 0
	for start < len(b) && asciiSpace(b[start]) {
		start++
	}
	end := len(b)
	for end > start && asciiSpace(b[end-1]) {
		end--
	}
	return b[start:end]
}

func asciiSpace(c byte) bool {
	switch c {
	case ' ', '\t', '\n', '\r', '\v', '\f':
		return true
	default:
		return false
	}
}
