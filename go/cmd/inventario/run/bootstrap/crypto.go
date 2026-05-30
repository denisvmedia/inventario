package bootstrap

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"

	"github.com/denisvmedia/inventario/internal/backupsign"
)

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
// On empty/invalid input a random 32-byte seed is generated and its hex is
// printed once to stderr (outside the structured logger, mirroring the JWT /
// file-signing flows) so operators can persist it via
// INVENTARIO_RUN_BACKUP_SIGNING_KEY and keep their archives verifiable across
// restarts.
func getBackupSigningKey(configKey string) (*backupsign.Signer, error) {
	seed, err := resolveBackupSeed(configKey)
	if err != nil {
		return nil, err
	}
	return backupsign.NewSigner(seed)
}

// resolveBackupSeed returns the 32-byte Ed25519 seed for the backup signer,
// generating + announcing a random one when the configured value is absent or
// not exactly 32 bytes.
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
		slog.Warn("Configured backup signing key is not exactly 32 bytes (or 64 hex chars); generating a random seed")
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
