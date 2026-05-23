package appctx

import (
	"context"
)

// AdminActor is the identity-only projection of the operator behind an
// admin/back-office request. It deliberately decouples the audit / op
// surface in apiserver from the concrete *models.BackofficeUser type:
// admin handlers only ever need the ID + Email + Name to fill audit-row
// columns and log lines, so this thin adapter keeps the handler call
// sites readable and the appctx → apiserver dependency direction clean.
//
// Issue #1785, Phase 3 — introduced together with the swap from the
// tenant-side `RequireSystemAdmin` middleware to the back-office
// `RequireBackofficeAuth`. A future phase that adds a second admin
// identity shape (e.g. a federated SSO platform operator) can populate
// the same struct without forcing a second pass over every handler.
type AdminActor struct {
	// ID is backoffice_users.id when the request was admitted via
	// RequireBackofficeAuth. Empty AdminActor (or a nil pointer from
	// AdminActorFromContext) signals "no admin actor on this request" —
	// callers MUST guard against that, exactly as they would for
	// UserFromContext.
	ID string
	// Email is the operator's email, populated for log lines and audit
	// breadcrumbs. Not used for authorization.
	Email string
	// Name is the operator's display name. Same use as Email.
	Name string
}

// AdminActorFromContext returns the admin actor attached to the request
// context by RequireBackofficeAuth (via WithBackofficeUser). Returns nil
// when the context carries no back-office identity — either the
// middleware chain is misconfigured, or the call site is running
// outside the admin subtree. Callers MUST handle nil; do not deref
// blindly.
//
// Deliberately uses the back-office context slot (NOT the tenant
// userCtxKey) so a tenant-user request cannot accidentally satisfy the
// admin-actor read — keeping the two universes on separate keys is the
// same invariant that drives BackofficeUserFromContext.
func AdminActorFromContext(ctx context.Context) *AdminActor {
	user := BackofficeUserFromContext(ctx)
	if user == nil {
		return nil
	}
	return &AdminActor{
		ID:    user.ID,
		Email: user.Email,
		Name:  user.Name,
	}
}
