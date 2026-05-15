package secrets_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/secrets"
)

func TestDeriveSubkey_Deterministic(t *testing.T) {
	c := qt.New(t)
	root := []byte("test-root-key-32-bytes-minimum-len")
	k1, err := secrets.DeriveSubkey(root, "mfa-secret-v1")
	c.Assert(err, qt.IsNil)
	k2, err := secrets.DeriveSubkey(root, "mfa-secret-v1")
	c.Assert(err, qt.IsNil)
	c.Assert(k1, qt.DeepEquals, k2)
	c.Assert(len(k1), qt.Equals, secrets.SubkeyLen)
}

func TestDeriveSubkey_DifferentLabelsDiffer(t *testing.T) {
	c := qt.New(t)
	root := []byte("test-root-key-32-bytes-minimum-len")
	k1, err := secrets.DeriveSubkey(root, "label-a")
	c.Assert(err, qt.IsNil)
	k2, err := secrets.DeriveSubkey(root, "label-b")
	c.Assert(err, qt.IsNil)
	c.Assert(k1, qt.Not(qt.DeepEquals), k2)
}

func TestDeriveSubkey_RejectsEmptyInputs(t *testing.T) {
	c := qt.New(t)
	_, err := secrets.DeriveSubkey(nil, "label")
	c.Assert(err, qt.Equals, secrets.ErrEmptyRootKey)
	_, err = secrets.DeriveSubkey([]byte("root"), "")
	c.Assert(err, qt.Equals, secrets.ErrEmptyDerivationLabel)
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	c := qt.New(t)
	root := []byte("test-root-key-32-bytes-minimum-len")
	key, err := secrets.DeriveSubkey(root, "round-trip")
	c.Assert(err, qt.IsNil)

	for _, plaintext := range []string{
		"",
		"a",
		"JBSWY3DPEHPK3PXP", // typical TOTP base32 secret
		strings.Repeat("x", 1024),
	} {
		enc, err := secrets.EncryptString(key, plaintext)
		c.Assert(err, qt.IsNil)
		c.Assert(enc, qt.Not(qt.Equals), plaintext)
		dec, err := secrets.DecryptString(key, enc)
		c.Assert(err, qt.IsNil)
		c.Assert(dec, qt.Equals, plaintext)
	}
}

func TestEncryptString_RandomNonce(t *testing.T) {
	c := qt.New(t)
	root := []byte("test-root-key-32-bytes-minimum-len")
	key, err := secrets.DeriveSubkey(root, "nonce-randomness")
	c.Assert(err, qt.IsNil)

	enc1, err := secrets.EncryptString(key, "plaintext")
	c.Assert(err, qt.IsNil)
	enc2, err := secrets.EncryptString(key, "plaintext")
	c.Assert(err, qt.IsNil)
	c.Assert(enc1, qt.Not(qt.Equals), enc2)
}

func TestDecryptString_RejectsTampered(t *testing.T) {
	c := qt.New(t)
	root := []byte("test-root-key-32-bytes-minimum-len")
	key, err := secrets.DeriveSubkey(root, "tamper")
	c.Assert(err, qt.IsNil)

	enc, err := secrets.EncryptString(key, "secret")
	c.Assert(err, qt.IsNil)

	// Flip the last byte — invalidates the GCM auth tag.
	tampered := []byte(enc)
	tampered[len(tampered)-1] ^= 0x01
	_, err = secrets.DecryptString(key, string(tampered))
	c.Assert(err, qt.Equals, secrets.ErrCiphertextCorrupted)
}

func TestDecryptString_RejectsWrongKey(t *testing.T) {
	c := qt.New(t)
	root := []byte("test-root-key-32-bytes-minimum-len")
	keyA, _ := secrets.DeriveSubkey(root, "key-a")
	keyB, _ := secrets.DeriveSubkey(root, "key-b")

	enc, err := secrets.EncryptString(keyA, "secret")
	c.Assert(err, qt.IsNil)
	_, err = secrets.DecryptString(keyB, enc)
	c.Assert(err, qt.Equals, secrets.ErrCiphertextCorrupted)
}

func TestEncryptString_RejectsWrongKeyLength(t *testing.T) {
	c := qt.New(t)
	_, err := secrets.EncryptString([]byte("too-short"), "x")
	c.Assert(err, qt.Equals, secrets.ErrSubkeyLength)
}
