package bootstrap

import (
	"encoding/hex"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/backupsign"
)

// validHexSeed returns a 64-hex-char string that decodes to a 32-byte seed.
func validHexSeed() string {
	seed := make([]byte, backupsign.SeedSize)
	for i := range seed {
		seed[i] = byte(i + 1)
	}
	return hex.EncodeToString(seed)
}

func TestResolveBackupSeed_EmptyGeneratesRandomSeed(t *testing.T) {
	c := qt.New(t)

	seed, err := resolveBackupSeed("")
	c.Assert(err, qt.IsNil)
	c.Assert(seed, qt.HasLen, backupsign.SeedSize)
}

func TestResolveBackupSeed_ValidHexDecodes(t *testing.T) {
	c := qt.New(t)

	want := make([]byte, backupsign.SeedSize)
	for i := range want {
		want[i] = byte(i + 1)
	}

	seed, err := resolveBackupSeed(validHexSeed())
	c.Assert(err, qt.IsNil)
	c.Assert(seed, qt.DeepEquals, want)
}

func TestResolveBackupSeed_ValidRawDecodes(t *testing.T) {
	c := qt.New(t)

	raw := make([]byte, backupsign.SeedSize)
	for i := range raw {
		raw[i] = byte(i + 9)
	}

	seed, err := resolveBackupSeed(string(raw))
	c.Assert(err, qt.IsNil)
	c.Assert(seed, qt.DeepEquals, raw)
}

func TestResolveBackupSeed_NonEmptyMalformedErrors(t *testing.T) {
	c := qt.New(t)

	// A non-empty but invalid value must be REJECTED (not silently rotated to a
	// fresh random seed), so a typo can never invalidate existing .inb archives.
	seed, err := resolveBackupSeed("not-a-valid-seed")
	c.Assert(err, qt.ErrorIs, ErrInvalidBackupSigningKey)
	c.Assert(seed, qt.IsNil)
}

func TestResolveBackupSeed_NonEmptyShortHexErrors(t *testing.T) {
	c := qt.New(t)

	// Valid hex of 20 bytes → 40 hex chars: neither the 64-hex-char seed form
	// nor a 32-byte raw string, so it must be rejected (not truncated/padded).
	short := hex.EncodeToString(make([]byte, 20))
	seed, err := resolveBackupSeed(short)
	c.Assert(err, qt.ErrorIs, ErrInvalidBackupSigningKey)
	c.Assert(seed, qt.IsNil)
}

func TestGetBackupSigningKey_NonEmptyMalformedErrors(t *testing.T) {
	c := qt.New(t)

	signer, err := getBackupSigningKey("too-short")
	c.Assert(err, qt.ErrorIs, ErrInvalidBackupSigningKey)
	c.Assert(signer, qt.IsNil)
}

func TestGetBackupSigningKey_ValidHexBuildsSigner(t *testing.T) {
	c := qt.New(t)

	signer, err := getBackupSigningKey(validHexSeed())
	c.Assert(err, qt.IsNil)
	c.Assert(signer, qt.IsNotNil)
}

func TestIsPlaceholderSecret(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{"empty", "", false},
		{"root placeholder", "please-change-this-to-a-secure-random-value-use-openssl-rand-hex-32", true},
		{"root placeholder uppercased", "PLEASE-CHANGE-THIS-TO-A-SECURE-RANDOM-VALUE-USE-OPENSSL-RAND-HEX-32", true},
		{"override jwt placeholder", "your-secure-32-byte-jwt-secret-here-replace-this-value", true},
		{"override file placeholder", "your-secure-32-byte-file-signing-key-here-replace-this-value", true},
		// Labelled CI test fixtures (.github/workflows/e2e-tests.yml) must NOT be
		// rejected — otherwise the e2e stack can't start. Regression guard for the
		// over-broad "please-change" marker that previously matched these.
		{"e2e jwt fixture", "e2e-jwt-secret-please-change-for-prod", false},
		{"e2e file fixture", "e2e-file-signing-key-please-change", false},
		{"real hex secret", "3a7bd3e2360a3d29eea436fcfb7e44c735d117c42d1c1835420b6b9942dd4f1b", false},
	}
	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			c.Assert(isPlaceholderSecret(tt.value), qt.Equals, tt.want)
		})
	}
}

func TestGetJWTSecret_RejectsRepoPlaceholder(t *testing.T) {
	c := qt.New(t)

	secret, err := getJWTSecret("please-change-this-to-a-secure-random-value-use-openssl-rand-hex-32")
	c.Assert(err, qt.ErrorIs, ErrPlaceholderSecret)
	c.Assert(secret, qt.IsNil)
}

func TestGetJWTSecret_AcceptsLabelledTestFixture(t *testing.T) {
	c := qt.New(t)

	// The e2e workflow sets this exact value; it must be accepted so the stack starts.
	secret, err := getJWTSecret("e2e-jwt-secret-please-change-for-prod")
	c.Assert(err, qt.IsNil)
	c.Assert(secret, qt.DeepEquals, []byte("e2e-jwt-secret-please-change-for-prod"))
}

func TestGetFileSigningKey_AcceptsLabelledTestFixture(t *testing.T) {
	c := qt.New(t)

	key, err := getFileSigningKey("e2e-file-signing-key-please-change")
	c.Assert(err, qt.IsNil)
	c.Assert(key, qt.DeepEquals, []byte("e2e-file-signing-key-please-change"))
}

func TestGetJWTSecret_EmptyGeneratesRandom(t *testing.T) {
	c := qt.New(t)

	secret, err := getJWTSecret("")
	c.Assert(err, qt.IsNil)
	c.Assert(secret, qt.HasLen, 32)
}
