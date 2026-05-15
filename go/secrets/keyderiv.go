// Package secrets carries the tiny app-secret abstraction the auth stack
// uses for symmetric encryption-at-rest. Today it only powers the MFA
// TOTP secret column (#1380 / PR-C #1645), but it is deliberately the
// only place AES-GCM lives so a future "app key" rotation feature has
// one file to extend.
//
// The design is intentionally narrow:
//
//   - Operators configure a single root key (the JWT signing secret) and
//     every persistence-layer subkey is HKDF-derived from it with a
//     per-purpose label. Subkeys are deterministic for a given root +
//     label, so encrypted columns stay readable across restarts.
//   - DeriveSubkey(label) is the only API that produces a usable
//     encryption key. EncryptString / DecryptString work against the
//     derived []byte and tag-version the output so a key rotation can
//     later identify which generation produced a ciphertext.
//
// Rotating the root key invalidates every derived subkey — for the MFA
// use case this means existing TOTP enrollments stop verifying, which
// is the documented behaviour (operators rotate root → users
// re-enroll MFA). A future "second root with rotation window" can be
// added without changing call sites.
package secrets

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/hkdf"
)

// SubkeyLen is the byte size of every derived subkey. 32 bytes selects
// AES-256-GCM. Callers should not depend on the value; it is exported
// only for test setup convenience.
const SubkeyLen = 32

// ciphertextVersion is prepended to every encrypted blob so a future
// rotation can keep decrypting v1 ciphertexts while writing v2. Bumping
// this is a breaking change for stored data — coordinate with a
// migration.
const ciphertextVersion = byte(0x01)

// Errors returned by Decrypt* helpers. Callers compare via errors.Is.
var (
	ErrCiphertextTooShort   = errors.New("ciphertext too short")
	ErrCiphertextVersion    = errors.New("unsupported ciphertext version")
	ErrCiphertextCorrupted  = errors.New("ciphertext failed authentication")
	ErrSubkeyLength         = errors.New("subkey must be 32 bytes")
	ErrEmptyRootKey         = errors.New("root key is empty")
	ErrEmptyDerivationLabel = errors.New("derivation label is empty")
)

// DeriveSubkey returns a 32-byte AES-GCM key derived from rootKey for
// the given label. The label namespaces the subkey so two callers
// asking for different purposes (e.g. "mfa-secret-v1" vs
// "session-cookie-v1") get different keys.
//
// HKDF with SHA-256 is used; the rootKey is the IKM, label is the
// info field, and the salt is empty (rootKey already has enough
// entropy from the operator-supplied JWT secret which validation
// enforces >= 32 bytes).
func DeriveSubkey(rootKey []byte, label string) ([]byte, error) {
	if len(rootKey) == 0 {
		return nil, ErrEmptyRootKey
	}
	if label == "" {
		return nil, ErrEmptyDerivationLabel
	}
	r := hkdf.New(sha256NewHash, rootKey, nil, []byte(label))
	out := make([]byte, SubkeyLen)
	if _, err := io.ReadFull(r, out); err != nil {
		return nil, fmt.Errorf("hkdf: %w", err)
	}
	return out, nil
}

// EncryptString encrypts plaintext with key and returns a versioned,
// base64-url-encoded ciphertext suitable for storage in a TEXT column.
// The format is `base64(version || nonce || ciphertext || tag)`.
func EncryptString(key []byte, plaintext string) (string, error) {
	if len(key) != SubkeyLen {
		return "", ErrSubkeyLength
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("nonce: %w", err)
	}
	sealed := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	buf := make([]byte, 0, 1+len(nonce)+len(sealed))
	buf = append(buf, ciphertextVersion)
	buf = append(buf, nonce...)
	buf = append(buf, sealed...)
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// DecryptString reverses EncryptString. Returns ErrCiphertextVersion
// when the stored version byte does not match the build's
// ciphertextVersion (a future rotation may add a fallback path).
func DecryptString(key []byte, encoded string) (string, error) {
	if len(key) != SubkeyLen {
		return "", ErrSubkeyLength
	}
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if len(raw) < 1 {
		return "", ErrCiphertextTooShort
	}
	if raw[0] != ciphertextVersion {
		return "", ErrCiphertextVersion
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < 1+nonceSize+gcm.Overhead() {
		return "", ErrCiphertextTooShort
	}
	nonce := raw[1 : 1+nonceSize]
	sealed := raw[1+nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, sealed, nil)
	if err != nil {
		return "", ErrCiphertextCorrupted
	}
	return string(plaintext), nil
}
