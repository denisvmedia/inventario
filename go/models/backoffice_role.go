package models

import (
	"github.com/jellydator/validation"
)

// BackofficeRole names the closed set of platform-operator roles that
// can be assigned to a BackofficeUser. The taxonomy is intentionally
// small (two values) so the back-office auth plane can keep its
// authorisation logic explicit: support_agent has read-mostly access to
// the back-office surfaces, platform_admin can also mutate them. The
// per-endpoint authorisation matrix lands in Phase 2/3 of issue #1785;
// here we only carry the typed enum so validation and storage are
// already shape-correct.
//
// Storing the role as a TEXT column (rather than an integer) keeps the
// migration cheap (adding a value is a Go-side enum extension + a
// migration that updates Validate, no schema change), and matches the
// project's other typed-enum columns (TenantStatus, RegistrationMode,
// CommodityStatus, ...).
type BackofficeRole string

const (
	// BackofficeRoleSupportAgent is the read-mostly persona — Phase 2/3
	// will gate destructive back-office actions to platform_admin only.
	BackofficeRoleSupportAgent BackofficeRole = "support_agent"
	// BackofficeRolePlatformAdmin is the full-mutation persona for the
	// back-office surfaces. Distinct from `users.is_system_admin` —
	// system_admin lives on a regular tenant user and Phase 3 removes it
	// in favour of the back-office plane entirely.
	BackofficeRolePlatformAdmin BackofficeRole = "platform_admin"
)

// IsValid reports whether r is one of the closed-set role values.
// Useful for callers that need a boolean check without producing a
// validation.Error (e.g. registry-layer guards).
func (r BackofficeRole) IsValid() bool {
	switch r {
	case BackofficeRoleSupportAgent, BackofficeRolePlatformAdmin:
		return true
	}
	return false
}

var (
	_ validation.Validatable = (*BackofficeRole)(nil)
)

// Validate implements validation.Validatable so the role can be plugged
// into a model's ValidateWithContext chain. Unknown values produce a
// validation.NewError with a stable error code so the future Phase 2
// HTTP layer can map it to a typed JSON:API error.
func (r BackofficeRole) Validate() error {
	if r.IsValid() {
		return nil
	}
	return validation.NewError(
		"validation_invalid_backoffice_role",
		"must be one of: support_agent, platform_admin",
	)
}
