package noop

import (
	"context"

	"github.com/denisvmedia/inventario/csrf"
)

const noopToken = "noop-csrf-token"

// Service is a CSRF service that accepts every request without any validation.
// Use only in test environments where CSRF protection is deliberately disabled.
type Service struct{}

var _ csrf.Service = Service{}

// GenerateToken always returns the same static no-op token.
func (Service) GenerateToken(_ context.Context, _ string) (string, error) {
	return noopToken, nil
}

// ValidateToken always reports the token as valid.
func (Service) ValidateToken(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

// GetToken always returns the same static no-op token.
func (Service) GetToken(_ context.Context, _ string) (string, error) {
	return noopToken, nil
}

// RevokeToken is a no-op.
func (Service) RevokeToken(_ context.Context, _, _ string) error {
	return nil
}

// DeleteAllTokens is a no-op.
func (Service) DeleteAllTokens(_ context.Context, _ string) error {
	return nil
}

