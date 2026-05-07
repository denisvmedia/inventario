package bootstrap

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
)

// getJWTSecret retrieves the JWT secret from config/environment or generates a
// secure random one. Accepts both hex-encoded and plain-string secrets of at
// least 32 bytes; otherwise falls back to a randomly generated 32-byte secret.
//
// The generated secret is never written to logs or stderr — operators must set
// INVENTARIO_RUN_JWT_SECRET (or jwt-secret in the config file) to persist a
// stable signing key across restarts.
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

	// Generate a secure random secret. The value is intentionally never logged
	// or written to stderr to avoid leaking the signing key into any log
	// aggregator. Issued tokens will become invalid on the next restart unless
	// INVENTARIO_RUN_JWT_SECRET is configured.
	slog.Warn("No JWT secret configured, generating ephemeral random secret")
	slog.Warn("Tokens issued with this secret will not survive a restart; set INVENTARIO_RUN_JWT_SECRET (or jwt-secret in config) to a 32+ byte value for persistence")

	secret := make([]byte, 32)
	_, err := rand.Read(secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

// getFileSigningKey retrieves the file signing key from config/environment or
// generates a secure random one with the same semantics as getJWTSecret. The
// generated key is never written to logs or stderr.
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

	// Generate a secure random key. The value is intentionally never logged or
	// written to stderr; signed URLs minted with this key will not survive a
	// restart unless INVENTARIO_RUN_FILE_SIGNING_KEY is configured.
	slog.Warn("No file signing key configured, generating ephemeral random key")
	slog.Warn("Signed URLs issued with this key will not survive a restart; set INVENTARIO_RUN_FILE_SIGNING_KEY (or file-signing-key in config) to a 32+ byte value for persistence")

	key := make([]byte, 32)
	_, err := rand.Read(key)
	if err != nil {
		return nil, err
	}

	return key, nil
}
