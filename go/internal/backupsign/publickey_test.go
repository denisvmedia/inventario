package backupsign_test

import (
	"encoding/hex"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/backupsign"
)

func TestVerifyDigestWithPublicKey_RoundTrip(t *testing.T) {
	c := qt.New(t)

	s, err := backupsign.NewSigner(seed(0x22))
	c.Assert(err, qt.IsNil)

	digest := backupsign.NewDigest()
	_, _ = digest.Write([]byte("payload.tar.gz bytes"))
	sum := digest.Sum(nil)
	sig := s.SignDigest(sum)

	// The same public key the Signer holds must verify the digest.
	err = backupsign.VerifyDigestWithPublicKey(s.PublicKey(), sum, sig)
	c.Assert(err, qt.IsNil)
}

func TestVerifyDigestWithPublicKey_RejectsWrongKey(t *testing.T) {
	c := qt.New(t)

	signer, err := backupsign.NewSigner(seed(0x22))
	c.Assert(err, qt.IsNil)
	other, err := backupsign.NewSigner(seed(0x33))
	c.Assert(err, qt.IsNil)

	digest := backupsign.NewDigest()
	_, _ = digest.Write([]byte("payload"))
	sum := digest.Sum(nil)
	sig := signer.SignDigest(sum)

	err = backupsign.VerifyDigestWithPublicKey(other.PublicKey(), sum, sig)
	c.Assert(err, qt.ErrorIs, backupsign.ErrBadSignature)
}

func TestVerifyDigestWithPublicKey_RejectsBadLengthKey(t *testing.T) {
	c := qt.New(t)
	err := backupsign.VerifyDigestWithPublicKey([]byte{0x01, 0x02}, []byte("digest"), []byte("sig"))
	c.Assert(err, qt.ErrorIs, backupsign.ErrBadSignature)
}

func TestParsePublicKey_AllForms(t *testing.T) {
	c := qt.New(t)

	s, err := backupsign.NewSigner(seed(0x44))
	c.Assert(err, qt.IsNil)

	pemBytes, err := s.PublicKeyPEM()
	c.Assert(err, qt.IsNil)
	b64 := s.PublicKeyBase64()
	hexStr := hex.EncodeToString(s.PublicKey())

	for name, input := range map[string][]byte{
		"pem":            pemBytes,
		"base64":         []byte(b64),
		"hex":            []byte(hexStr),
		"hex_trailingnl": []byte(hexStr + "\n"),
	} {
		t.Run(name, func(t *testing.T) {
			c := qt.New(t)
			pub, err := backupsign.ParsePublicKey(input)
			c.Assert(err, qt.IsNil)
			c.Assert([]byte(pub), qt.DeepEquals, []byte(s.PublicKey()))
		})
	}
}

func TestParsePublicKey_RejectsGarbage(t *testing.T) {
	c := qt.New(t)
	_, err := backupsign.ParsePublicKey([]byte("not a key"))
	c.Assert(err, qt.IsNotNil)

	_, err = backupsign.ParsePublicKey(nil)
	c.Assert(err, qt.IsNotNil)
}
