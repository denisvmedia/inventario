package bootstrap

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"

	"github.com/go-extras/errx"

	"github.com/denisvmedia/inventario/internal/backupsign"
)

// ErrInvalidBackupSigningKey is returned when INVENTARIO_RUN_BACKUP_SIGNING_KEY
// (or the config value) is set but is not a valid 32-byte Ed25519 seed (64 hex
// chars or exactly 32 raw bytes). Unlike the HMAC secrets, a malformed backup
// signing key is a hard error rather than a fall-back-to-random, because
// silently rotating the long-lived key would break verification of every
// existing `.inb` archive.
var ErrInvalidBackupSigningKey = errx.NewSentinel("configured backup signing key must be exactly 32 bytes (or 64 hex characters)")

// getJWTSecret retrieves the JWT secret from config/environment or generates a
// secure random one. Accepts both hex-encoded and plain-string secrets of at
// least 32 bytes; otherwise falls back to a randomly generated 32-byte secret
// whose hex value is written once to stderr (outside the structured logger) so
// operators can capture it on first boot without leaking the signing key into
// log aggregators.
func getJWTSecret(configSecret string) ([]byte, error) {
	// Use the secret from config (which includes environment variables via cleanenv)
	if configSecret != "" {
		// If provided as hex string, decode it
		if decoded, err := hex.DecodeString(configSecret); err == nil && len(decoded) >= 32 {
			slog.Info("Using JWT secret from configuration (hex decoded)")
			return decoded, nil
		}
		// If provided as plain string and long enough, use it directly
		if len(configSecret) >= 32 {
			slog.Info("Using JWT secret from configuration")
			return []byte(configSecret), nil
		}
		slog.Warn("Configured JWT secret is too short (minimum 32 characters), generating random secret")
	}

	// Generate a secure random secret
	slog.Warn("No JWT secret configured, generating random secret")
	slog.Warn("For production use, set INVENTARIO_RUN_JWT_SECRET environment variable or jwt-secret in config file with a secure 32+ byte secret")

	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	if err != nil {
		return nil, err
	}

	slog.Warn("Generated random JWT secret; persist it via INVENTARIO_RUN_JWT_SECRET to keep tokens valid across restarts")
	// Print the generated secret to stderr (not the structured log) so operators
	// can capture it on first boot without leaking the signing key into any log
	// aggregator that collects application logs.
	fmt.Fprintf(os.Stderr, "INVENTARIO_RUN_JWT_SECRET=%s\n", hex.EncodeToString(secret))

	return secret, nil
}

// getFileSigningKey retrieves the file signing key from config/environment or
// generates a secure random one with the same semantics as getJWTSecret.
func getFileSigningKey(configKey string) ([]byte, error) {
	// Use the key from config (which includes environment variables via cleanenv)
	if configKey != "" {
		// If provided as hex string, decode it
		if decoded, err := hex.DecodeString(configKey); err == nil && len(decoded) >= 32 {
			slog.Info("Using file signing key from configuration (hex decoded)")
			return decoded, nil
		}
		// If provided as plain string and long enough, use it directly
		if len(configKey) >= 32 {
			slog.Info("Using file signing key from configuration")
			return []byte(configKey), nil
		}
		slog.Warn("Configured file signing key is too short (minimum 32 characters), generating random key")
	}

	// Generate a secure random key
	slog.Warn("No file signing key configured, generating random key")
	slog.Warn("For production use, set INVENTARIO_RUN_FILE_SIGNING_KEY environment variable or file-signing-key in config file with a secure 32+ byte key")

	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}

	slog.Warn("Generated random file signing key; persist it via INVENTARIO_RUN_FILE_SIGNING_KEY to keep signed URLs valid across restarts")
	// Print the generated key to stderr (not the structured log) so operators
	// can capture it on first boot without leaking the signing key into any log
	// aggregator that collects application logs.
	fmt.Fprintf(os.Stderr, "INVENTARIO_RUN_FILE_SIGNING_KEY=%s\n", hex.EncodeToString(key))

	return key, nil
}

// getBackupSigningKey resolves the Ed25519 seed used to sign `.inb` backup
// archives (issue #534) and builds a *backupsign.Signer from it. Unlike the
// HMAC secrets above, the decoded seed must be EXACTLY 32 bytes (the Ed25519
// seed size) — a longer/shorter value is rejected, never truncated or padded,
// because the key pair is derived deterministically from the seed and a
// silently-resized seed would produce a different, unrecoverable key.
//
// Accepted input forms:
//   - 64 hex characters → decoded to 32 raw bytes,
//   - a 32-byte raw string → used directly.
//
// Key-resolution policy DIVERGES from getJWTSecret / getFileSigningKey on
// purpose: those HMAC secrets sign short-lived tokens / URLs, so silently
// rotating to a fresh random secret on a too-short value only invalidates
// in-flight requests. Backup signing keys are LONG-LIVED — `.inb` archives
// produced today must still verify months later — so silently rotating the key
// on a typo would permanently break verification of every existing archive
// with no warning. Therefore:
//   - empty/unset value  → generate a random seed (first-boot convenience),
//     print it to stderr to persist.
//   - non-empty but not a valid 32-byte seed → HARD ERROR (a malformed value
//     is a misconfiguration; failing loudly beats silently
//     rotating the signing key).
func getBackupSigningKey(configKey string) (*backupsign.Signer, error) {
	seed, err := resolveBackupSeed(configKey)
	if err != nil {
		return nil, err
	}
	return backupsign.NewSigner(seed)
}

// resolveBackupSeed returns the 32-byte Ed25519 seed for the backup signer. It
// generates + announces a random seed ONLY when the configured value is empty;
// a non-empty but malformed value is rejected rather than silently rotating the
// long-lived signing key (see getBackupSigningKey for the rationale).
func resolveBackupSeed(configKey string) ([]byte, error) {
	if configKey != "" {
		// Hex form: must decode to exactly the Ed25519 seed size.
		if decoded, decErr := hex.DecodeString(configKey); decErr == nil && len(decoded) == backupsign.SeedSize {
			slog.Info("Using backup signing key from configuration (hex decoded)")
			return decoded, nil
		}
		// Raw form: must already be exactly the Ed25519 seed size. We do
		// NOT accept "at least 32" here (unlike the HMAC keys) because the
		// seed length is fixed — a 40-byte string is a misconfiguration,
		// not a longer-is-fine secret.
		if len(configKey) == backupsign.SeedSize {
			slog.Info("Using backup signing key from configuration")
			return []byte(configKey), nil
		}
		// A non-empty but invalid value is a misconfiguration. Refuse rather
		// than generate: silently rotating a long-lived backup signing key on
		// a typo would break verification of every existing `.inb` archive.
		return nil, errx.Classify(
			ErrInvalidBackupSigningKey,
			errx.Attrs("len", len(configKey), "want", backupsign.SeedSize),
		)
	}

	slog.Warn("No backup signing key configured, generating random seed")
	slog.Warn("For production use, set INVENTARIO_RUN_BACKUP_SIGNING_KEY environment variable or backup-signing-key in config file with a 32-byte (64 hex char) Ed25519 seed")

	seed := make([]byte, backupsign.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}

	slog.Warn("Generated random backup signing key; persist it via INVENTARIO_RUN_BACKUP_SIGNING_KEY to keep .inb archives verifiable across restarts")
	// Print to stderr (not the structured log) so operators can capture the
	// seed on first boot without leaking it into a log aggregator.
	fmt.Fprintf(os.Stderr, "INVENTARIO_RUN_BACKUP_SIGNING_KEY=%s\n", hex.EncodeToString(seed))

	return seed, nil
}

// getOAuthStateKey retrieves the OAuth state-signing key from
// config/environment or generates a secure random one. Mirrors
// getJWTSecret and getFileSigningKey: accepts hex-encoded or plain
// strings of at least 32 bytes; otherwise falls back to a randomly
// generated 32-byte key whose hex is written once to stderr so operators
// can capture and persist it. Multi-replica deployments MUST supply a
// stable value or signed states won't survive a request that lands on
// a different replica after the provider redirect.
func getOAuthStateKey(configKey string) ([]byte, error) {
	if configKey != "" {
		if decoded, err := hex.DecodeString(configKey); err == nil && len(decoded) >= 32 {
			slog.Info("Using OAuth state key from configuration (hex decoded)")
			return decoded, nil
		}
		if len(configKey) >= 32 {
			slog.Info("Using OAuth state key from configuration")
			return []byte(configKey), nil
		}
		slog.Warn("Configured OAuth state key is too short (minimum 32 characters), generating random key")
	}

	slog.Warn("No OAuth state key configured, generating random key")
	slog.Warn("For production use, set INVENTARIO_RUN_OAUTH_STATE_KEY environment variable or oauth-state-key in config file with a secure 32+ byte key")

	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}

	slog.Warn("Generated random OAuth state key; persist it via INVENTARIO_RUN_OAUTH_STATE_KEY to keep in-flight OAuth flows valid across restarts and replicas")
	fmt.Fprintf(os.Stderr, "INVENTARIO_RUN_OAUTH_STATE_KEY=%s\n", hex.EncodeToString(key))

	return key, nil
}
