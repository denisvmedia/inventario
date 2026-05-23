package appctx

import (
	"context"

	"github.com/denisvmedia/inventario/models"
)

// BackofficeUserFromContext returns the back-office (platform-operator)
// identity attached by RequireBackofficeAuth. Returns nil when the
// context carries no back-office identity. Distinct from
// UserFromContext on purpose: a request inside the back-office plane
// MUST never resolve to a tenant user, and the two universes live on
// separate context keys so the call site reads obvious. Issue #1785,
// Phase 2.
func BackofficeUserFromContext(ctx context.Context) *models.BackofficeUser {
	user, ok := ctx.Value(backofficeUserCtxKey).(*models.BackofficeUser)
	if !ok {
		return nil
	}
	return user
}

// WithBackofficeUser attaches a back-office identity to the request
// context. Should only be called by RequireBackofficeAuth (and by tests
// that bypass the middleware). Does NOT touch the tenant userCtxKey or
// userIDKey — those slots are reserved for tenant users.
func WithBackofficeUser(ctx context.Context, user *models.BackofficeUser) context.Context {
	return context.WithValue(ctx, backofficeUserCtxKey, user)
}
