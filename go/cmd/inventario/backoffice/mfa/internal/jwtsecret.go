// Package internal is the shared helper for the back-office MFA CLI
// subcommands. Resolves the JWT secret from the supplied flag value
// (which cleanenv-binds to INVENTARIO_BACKOFFICE_MFA_JWT_SECRET) and
// rejects values too short to satisfy the secrets package's
// SubkeyLen >= 32 requirement.
//
// Kept in a sub-package so each subcommand's main package can import
// it without pulling in a circular dependency on the parent `mfa`
// package.
package internal

import (
	"encoding/hex"
	"errors"
)

// ResolveJWTSecret converts the flag-supplied JWT secret string into a
// byte slice suitable for services.NewMFAService. Accepts both
// hex-encoded (64+ hex chars) and plain-string (32+ bytes) inputs —
// mirrors the server's getJWTSecret behaviour in cmd/inventario/run/
// bootstrap/crypto.go so a config-file value works in both places
// without re-encoding.
func ResolveJWTSecret(configSecret string) ([]byte, error) {
	if configSecret == "" {
		return nil, errors.New("--jwt-secret is required (or INVENTARIO_BACKOFFICE_MFA_JWT_SECRET env)")
	}
	if decoded, err := hex.DecodeString(configSecret); err == nil && len(decoded) >= 32 {
		return decoded, nil
	}
	if len(configSecret) >= 32 {
		return []byte(configSecret), nil
	}
	return nil, errors.New("JWT secret is too short: need at least 32 bytes plaintext or 64 hex chars")
}
