package backup

import (
	"encoding/hex"
	"os"

	"github.com/go-extras/errx"

	"github.com/denisvmedia/inventario/internal/backupsign"
)

// backupSigningKeyEnv is the environment variable the server reads the backup
// signing seed from (see cmd/inventario/run/bootstrap). The CLI reuses it so an
// operator can run `inventario backup resign` with the same configuration the
// server uses.
const backupSigningKeyEnv = "INVENTARIO_RUN_BACKUP_SIGNING_KEY"

// loadSigner resolves the backup signing seed from the --backup-signing-key flag
// (if set) or the INVENTARIO_RUN_BACKUP_SIGNING_KEY environment variable, and
// builds a *backupsign.Signer. The seed must decode to EXACTLY 32 bytes (64 hex
// chars or a 32-byte raw string) — the same rule the server enforces.
func loadSigner(flagValue string) (*backupsign.Signer, error) {
	raw := flagValue
	if raw == "" {
		raw = os.Getenv(backupSigningKeyEnv)
	}
	if raw == "" {
		return nil, errx.NewSentinel("no backup signing key configured; set --backup-signing-key or " + backupSigningKeyEnv)
	}

	seed, err := decodeSeed(raw)
	if err != nil {
		return nil, err
	}
	return backupsign.NewSigner(seed)
}

// decodeSeed mirrors the server's resolveBackupSeed: 64 hex chars → 32 bytes, or
// a 32-byte raw string. Any other length is rejected (never truncated/padded).
func decodeSeed(raw string) ([]byte, error) {
	if decoded, err := hex.DecodeString(raw); err == nil && len(decoded) == backupsign.SeedSize {
		return decoded, nil
	}
	if len(raw) == backupsign.SeedSize {
		return []byte(raw), nil
	}
	return nil, errx.NewSentinel("backup signing key must be exactly 32 bytes (or 64 hex characters)")
}
