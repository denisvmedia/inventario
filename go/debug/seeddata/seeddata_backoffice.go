package seeddata

import (
	"context"
	"errors"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"golang.org/x/crypto/bcrypt"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// Back-office operator fixtures consumed by the admin e2e suite (#2100).
// They live OUTSIDE the tenant model — `backoffice_users` has no tenant_id
// — so the emails deliberately carry no tenant suffix.
const (
	// backofficeAdminEmail is the platform_admin operator the suite signs in
	// as. platform_admin is the only role allowed to start an impersonation
	// session (RequirePlatformAdmin on POST /admin/users/{id}/impersonate).
	backofficeAdminEmail = "operator@backoffice.test"

	// backofficeSupportEmail is the support_agent operator. It exists so the
	// suite can assert the role boundary: a support agent reaches the
	// read surfaces but is refused at impersonation start.
	backofficeSupportEmail = "support@backoffice.test"

	// backofficeFixturePassword matches the tenant fixtures' password so the
	// harness has one credential to remember.
	backofficeFixturePassword = "TestPassword123"
)

// ensureBackofficeOperators idempotently provisions the two back-office
// operator fixtures the admin e2e suite authenticates as (#2100).
//
// The production path is `inventario backoffice bootstrap`, which opens its
// own postgres connection and so cannot serve the e2e harness: the local
// stack runs memory-mode, and even against postgres a separate process
// would not share the in-process registries. The seed is therefore the only
// provisioning path that works for every harness lane — the same reasoning
// that put ensureSystemAdminUser here.
//
// Both rows are created with MFAEnforced=false. The schema default is true,
// which fails login closed with 501 until an operator enrols TOTP via
// `inventario backoffice mfa setup`; a browser suite cannot do that, so the
// fixtures opt out explicitly. That is also why this is gated behind
// opts.SeedBackofficeOperators: minting a password-only platform operator
// from the unauthenticated /api/v1/seed endpoint would be a privilege-
// escalation hole in any deployment where /seed is reachable.
//
// Self-healing on re-seed: a fixture left deactivated or flipped to
// MFAEnforced=true by a previous run is reconciled back to the expected
// state rather than skipped.
func ensureBackofficeOperators(ctx context.Context, factorySet *registry.FactorySet, opts SeedOptions) error {
	if !opts.SeedBackofficeOperators {
		return nil
	}

	reg := factorySet.BackofficeUserRegistry
	if reg == nil {
		// A nil registry here is a miswired FactorySet that would produce a
		// stack the admin suite cannot sign in to. Fail loudly instead of
		// silently seeding nothing.
		return errxtrace.Wrap(
			"backoffice operator fixtures require BackofficeUserRegistry; FactorySet is miswired",
			registry.ErrInvalidConfig,
		)
	}

	fixtures := []struct {
		email string
		name  string
		role  models.BackofficeRole
	}{
		{backofficeAdminEmail, "E2E Platform Admin", models.BackofficeRolePlatformAdmin},
		{backofficeSupportEmail, "E2E Support Agent", models.BackofficeRoleSupportAgent},
	}
	for _, f := range fixtures {
		if err := ensureBackofficeOperator(ctx, reg, f.email, f.name, f.role); err != nil {
			return errxtrace.Wrap("failed to provision backoffice operator fixture "+f.email, err)
		}
	}
	return nil
}

func ensureBackofficeOperator(
	ctx context.Context,
	reg registry.BackofficeUserRegistry,
	email, name string,
	role models.BackofficeRole,
) error {
	existing, err := reg.GetByEmail(ctx, email)
	switch {
	case err == nil:
		return reconcileBackofficeOperator(ctx, reg, existing, role)
	case errors.Is(err, registry.ErrBackofficeUserNotFound):
		// expected on a fresh database — fall through to create
	default:
		return err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(backofficeFixturePassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user := models.BackofficeUser{
		Email:        email,
		Name:         name,
		PasswordHash: string(hash),
		Role:         role,
		IsActive:     true,
		MFAEnforced:  false,
	}
	if err := user.ValidateWithContext(ctx); err != nil {
		return err
	}
	if _, err := reg.Create(ctx, user); err != nil {
		// A concurrent seed may have inserted the row between the lookup
		// and here; that is a success for our purposes.
		if errors.Is(err, registry.ErrBackofficeEmailAlreadyExists) {
			return nil
		}
		return err
	}
	return nil
}

// reconcileBackofficeOperator drags a drifted fixture back to the state the
// suite needs. The password hash is deliberately left alone — a re-seed must
// not silently reset a credential an operator changed on a stack that is not
// the e2e one.
func reconcileBackofficeOperator(
	ctx context.Context,
	reg registry.BackofficeUserRegistry,
	user *models.BackofficeUser,
	role models.BackofficeRole,
) error {
	if user.IsActive && !user.MFAEnforced && user.Role == role {
		return nil
	}
	if !user.IsActive {
		if err := reg.SetActive(ctx, user.ID, true); err != nil {
			return err
		}
		user.IsActive = true
	}
	user.MFAEnforced = false
	user.Role = role
	if _, err := reg.Update(ctx, *user); err != nil {
		return err
	}
	return nil
}
