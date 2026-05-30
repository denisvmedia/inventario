package backupsign_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"io"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/backupsign"
)

func seed(b byte) []byte {
	s := make([]byte, backupsign.SeedSize)
	for i := range s {
		s[i] = b
	}
	return s
}

func TestNewSigner_RejectsWrongSeedSize(t *testing.T) {
	c := qt.New(t)

	for _, n := range []int{0, 1, 16, 31, 33, 64} {
		_, err := backupsign.NewSigner(make([]byte, n))
		c.Assert(err, qt.ErrorIs, backupsign.ErrInvalidSeedSize, qt.Commentf("seed len %d", n))
	}
}

func TestSigner_SignVerify_RoundTrip(t *testing.T) {
	c := qt.New(t)

	s, err := backupsign.NewSigner(seed(0x11))
	c.Assert(err, qt.IsNil)

	payload := []byte("payload.tar.gz bytes")
	sig := s.Sign(payload)
	c.Assert(sig, qt.HasLen, ed25519.SignatureSize)
	c.Assert(s.Verify(payload, sig), qt.IsNil)
}

func TestSigner_Verify_DetectsTamperedPayload(t *testing.T) {
	c := qt.New(t)

	s, err := backupsign.NewSigner(seed(0x22))
	c.Assert(err, qt.IsNil)

	payload := []byte("the original payload")
	sig := s.Sign(payload)

	tampered := bytes.Clone(payload)
	tampered[0] ^= 0xFF
	c.Assert(s.Verify(tampered, sig), qt.ErrorIs, backupsign.ErrBadSignature)
}

func TestSigner_Verify_DetectsTamperedSignature(t *testing.T) {
	c := qt.New(t)

	s, err := backupsign.NewSigner(seed(0x33))
	c.Assert(err, qt.IsNil)

	payload := []byte("payload")
	sig := s.Sign(payload)
	sig[0] ^= 0xFF
	c.Assert(s.Verify(payload, sig), qt.ErrorIs, backupsign.ErrBadSignature)

	// A truncated / wrong-length signature must also be rejected, not panic.
	c.Assert(s.Verify(payload, sig[:10]), qt.ErrorIs, backupsign.ErrBadSignature)
	c.Assert(s.Verify(payload, nil), qt.ErrorIs, backupsign.ErrBadSignature)
}

func TestSigner_Verify_RejectsOtherKey(t *testing.T) {
	c := qt.New(t)

	a, err := backupsign.NewSigner(seed(0x44))
	c.Assert(err, qt.IsNil)
	b, err := backupsign.NewSigner(seed(0x55))
	c.Assert(err, qt.IsNil)

	payload := []byte("signed by a")
	sig := a.Sign(payload)
	// b must NOT accept a's signature — this is the forgery defence.
	c.Assert(b.Verify(payload, sig), qt.ErrorIs, backupsign.ErrBadSignature)
}

func TestSigner_DeterministicFromSeed(t *testing.T) {
	c := qt.New(t)

	a, err := backupsign.NewSigner(seed(0x66))
	c.Assert(err, qt.IsNil)
	b, err := backupsign.NewSigner(seed(0x66))
	c.Assert(err, qt.IsNil)

	c.Assert(a.Fingerprint(), qt.Equals, b.Fingerprint())
	c.Assert(a.PublicKeyBase64(), qt.Equals, b.PublicKeyBase64())
	c.Assert([]byte(a.PublicKey()), qt.DeepEquals, []byte(b.PublicKey()))

	// Cross-verify: a's signature verifies under b (same key material).
	payload := []byte("same seed")
	c.Assert(b.Verify(payload, a.Sign(payload)), qt.IsNil)
}

func TestSigner_Fingerprint_HexSHA256(t *testing.T) {
	c := qt.New(t)

	s, err := backupsign.NewSigner(seed(0x77))
	c.Assert(err, qt.IsNil)
	fp := s.Fingerprint()
	c.Assert(fp, qt.HasLen, 64) // hex of 32-byte SHA-256
}

func TestSigner_StreamingDigestMatchesInMemory(t *testing.T) {
	c := qt.New(t)

	s, err := backupsign.NewSigner(seed(0x99))
	c.Assert(err, qt.IsNil)

	payload := bytes.Repeat([]byte("streamed payload chunk "), 4096)

	// Streaming digest: feed the payload through NewDigest (constant memory).
	h := backupsign.NewDigest()
	_, err = io.Copy(h, bytes.NewReader(payload))
	c.Assert(err, qt.IsNil)
	digest := h.Sum(nil)
	c.Assert(digest, qt.HasLen, backupsign.DigestSize)

	sig := s.SignDigest(digest)
	c.Assert(s.VerifyDigest(digest, sig), qt.IsNil)

	// The in-memory convenience must agree with the streaming path.
	c.Assert(s.Verify(payload, sig), qt.IsNil)
	c.Assert(s.VerifyDigest(digest, s.Sign(payload)), qt.IsNil)
}

func TestSigner_VerifyDigest_DetectsTamper(t *testing.T) {
	c := qt.New(t)

	s, err := backupsign.NewSigner(seed(0xAB))
	c.Assert(err, qt.IsNil)

	h := backupsign.NewDigest()
	_, _ = h.Write([]byte("original"))
	digest := h.Sum(nil)
	sig := s.SignDigest(digest)

	digest[0] ^= 0xFF
	c.Assert(s.VerifyDigest(digest, sig), qt.ErrorIs, backupsign.ErrBadSignature)
}

func TestSigner_PublicKeyPEM_ParsesBackToSameKey(t *testing.T) {
	c := qt.New(t)

	s, err := backupsign.NewSigner(seed(0x88))
	c.Assert(err, qt.IsNil)

	pemBytes, err := s.PublicKeyPEM()
	c.Assert(err, qt.IsNil)

	block, _ := pem.Decode(pemBytes)
	c.Assert(block, qt.IsNotNil)
	c.Assert(block.Type, qt.Equals, "PUBLIC KEY")

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	c.Assert(err, qt.IsNil)
	edPub, ok := pub.(ed25519.PublicKey)
	c.Assert(ok, qt.IsTrue)
	c.Assert([]byte(edPub), qt.DeepEquals, []byte(s.PublicKey()))
}
