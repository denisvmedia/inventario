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
