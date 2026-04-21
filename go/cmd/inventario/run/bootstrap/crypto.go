package bootstrap

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
)

// getJWTSecret retrieves the JWT secret from config/environment or generates a
// secure random one. Accepts both hex-encoded and plain-string secrets of at
// least 32 bytes; otherwise falls back to a randomly generated 32-byte secret
// whose hex value is logged so operators can persist it for future restarts.
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

	slog.Info("Generated random JWT secret (hex)", "secret", hex.EncodeToString(secret))
	slog.Info("Save this secret to INVENTARIO_RUN_JWT_SECRET environment variable or config file for consistent authentication across restarts")

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

	slog.Info("Generated random file signing key (hex)", "key", hex.EncodeToString(key))
	slog.Info("Save this key to INVENTARIO_RUN_FILE_SIGNING_KEY environment variable or config file for consistent file URL signing across restarts")

	return key, nil
}
