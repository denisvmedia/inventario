package admin

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

var (
	// ErrTenantDeleteNotConfirmed is returned when the slug typed to confirm a
	// tenant deletion is not the tenant's slug.
	ErrTenantDeleteNotConfirmed = errx.NewSentinel("the typed slug does not match the tenant's slug")

	// ErrTenantDomainTaken is returned when a tenant is given a domain that
	// another tenant already has. Requests are routed to a tenant by their
	// host, so two tenants on one domain would make that host ambiguous.
	ErrTenantDomainTaken = errx.NewSentinel("another tenant already uses this domain")

	// ErrTenantSessionsNotEnded marks the error a status change returns when
	// the new status was saved but ending some of the tenant's sessions
	// failed. The tenant returned with it is the saved row, and the users are
	// still refused, because their tenant is no longer active.
	ErrTenantSessionsNotEnded = errx.NewSentinel("ending the sessions of the tenant's users failed")
)

// maxTenantSlugLength is the longest slug Tenant.ValidateWithContext accepts.
const maxTenantSlugLength = 50

var tenantSlugSeparators = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateTenantSlug derives a slug from a tenant name: lowercase, every run
// of characters other than ASCII letters and digits becomes one hyphen, with
// no hyphen at either end and at most 50 characters. A name with no ASCII
// letter or digit gives an empty slug, which validation then refuses.
func GenerateTenantSlug(name string) string {
	slug := tenantSlugSeparators.ReplaceAllString(strings.ToLower(name), "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > maxTenantSlugLength {
		// The slug is ASCII at this point, so cutting bytes cuts characters.
		slug = strings.TrimSuffix(slug[:maxTenantSlugLength], "-")
	}
	return slug
}

// ConfirmTenantDeletion checks the slug an operator typed to confirm that a
// tenant should be deleted. The comparison is exact, the same one the
// `inventario tenants delete` prompt makes.
func ConfirmTenantDeletion(tenant *models.Tenant, typed string) error {
	if typed != tenant.Slug {
		return errxtrace.Classify(ErrTenantDeleteNotConfirmed, errx.Attrs("tenant_id", tenant.ID))
	}
	return nil
}

// SetTokenBlacklist wires the access-token blacklist that ending a tenant's
// sessions stamps for each of its users, so access tokens issued before the
// stamp are refused at once. ttl is how long the stamp lasts and must outlast
// an access token. Without a blacklist those access tokens are still refused,
// because the authentication middleware checks that the user's tenant is
// active.
func (s *Service) SetTokenBlacklist(blacklist services.TokenBlacklister, ttl time.Duration) {
	s.blacklist = blacklist
	s.blacklistTTL = ttl
}

// SuspendTenant sets the tenant's status to suspended and ends the sessions
// of every user in it (see EndTenantSessions). A suspended tenant's users
// cannot sign in. Suspending a tenant that is already suspended ends the
// sessions again and changes nothing else.
//
// When the error wraps ErrTenantSessionsNotEnded the status was saved and the
// returned tenant is the saved row.
func (s *Service) SuspendTenant(ctx context.Context, idOrSlug string) (*models.Tenant, error) {
	return s.setTenantStatus(ctx, idOrSlug, models.TenantStatusSuspended)
}

// ReactivateTenant sets the tenant's status back to active. Sessions that a
// suspension ended stay ended: the users sign in again.
func (s *Service) ReactivateTenant(ctx context.Context, idOrSlug string) (*models.Tenant, error) {
	return s.setTenantStatus(ctx, idOrSlug, models.TenantStatusActive)
}

func (s *Service) setTenantStatus(ctx context.Context, idOrSlug string, status models.TenantStatus) (*models.Tenant, error) {
	tenant, err := s.GetTenant(ctx, idOrSlug)
	if err != nil {
		return nil, err
	}
	if tenant.Status != status {
		tenant.Status = status
		tenant, err = s.factorySet.TenantRegistry.Update(ctx, *tenant)
		if err != nil {
			return nil, errxtrace.Wrap("failed to update the tenant status", err)
		}
	}
	if status == models.TenantStatusActive {
		return tenant, nil
	}
	return tenant, s.EndTenantSessions(ctx, tenant.ID)
}

// EndTenantSessions ends the sessions of every user in the tenant, the same
// teardown blocking a user runs: their refresh tokens and personal API tokens
// are revoked, and with a blacklist wired (SetTokenBlacklist) their access
// tokens are refused from now on. Every user is handled even when an earlier
// one failed; the failures come back joined and wrapped in
// ErrTenantSessionsNotEnded.
func (s *Service) EndTenantSessions(ctx context.Context, tenantID string) error {
	users, err := s.factorySet.UserRegistry.ListByTenant(ctx, tenantID)
	if err != nil {
		return errxtrace.Wrap("failed to list the tenant's users", err, ErrTenantSessionsNotEnded)
	}
	var errs []error
	for _, user := range users {
		errs = append(errs, s.endUserSessions(ctx, user.ID))
	}
	if err := errors.Join(errs...); err != nil {
		return errxtrace.Wrap("failed to end the sessions of some of the tenant's users", err, ErrTenantSessionsNotEnded)
	}
	return nil
}

// endUserSessions revokes one user's sessions. Every step runs even when an
// earlier one failed. The blacklist stamp goes before the API tokens: a token
// being minted reads the stamp after it is stored, so a token this revocation
// misses is refused by the stamp.
func (s *Service) endUserSessions(ctx context.Context, userID string) error {
	var errs []error
	if s.factorySet.RefreshTokenRegistry != nil {
		if err := s.factorySet.RefreshTokenRegistry.RevokeByUserID(ctx, userID); err != nil {
			errs = append(errs, errxtrace.Wrap("failed to revoke refresh tokens", err, errx.Attrs("user_id", userID)))
		}
	}
	if s.blacklist != nil {
		if err := s.blacklist.BlacklistUserTokens(ctx, userID, s.blacklistTTL); err != nil {
			errs = append(errs, errxtrace.Wrap("failed to blacklist access tokens", err, errx.Attrs("user_id", userID)))
		}
	}
	if s.factorySet.APITokenRegistry != nil {
		if _, err := s.factorySet.APITokenRegistry.RevokeAllByUserID(ctx, userID); err != nil {
			errs = append(errs, errxtrace.Wrap("failed to revoke API tokens", err, errx.Attrs("user_id", userID)))
		}
	}
	return errors.Join(errs...)
}

// checkTenantDomainFree refuses a domain another tenant already has. tenantID
// is the tenant being written, empty for a new one.
func (s *Service) checkTenantDomainFree(ctx context.Context, tenantID string, domain *string) error {
	if domain == nil || *domain == "" {
		return nil
	}
	other, err := s.factorySet.TenantRegistry.GetByDomain(ctx, *domain)
	switch {
	case errors.Is(err, registry.ErrNotFound):
		return nil
	case err != nil:
		return errxtrace.Wrap("failed to check the tenant domain", err)
	case other.ID != tenantID:
		return errxtrace.Classify(ErrTenantDomainTaken, errx.Attrs("domain", *domain))
	default:
		return nil
	}
}

// tenantDomainTaken returns ErrTenantDomainTaken for a registry write that
// the unique index on tenants.domain refused, and err as it is otherwise. Two
// writes that give one domain to two tenants at the same time both pass
// checkTenantDomainFree; the second to commit gets this, the error the check
// gives.
func tenantDomainTaken(err error, domain *string) error {
	if !errors.Is(err, registry.ErrDomainAlreadyExists) || domain == nil {
		return err
	}
	return errxtrace.Classify(ErrTenantDomainTaken, errx.Attrs("domain", *domain))
}

// normalizeTenantDomain turns an empty domain into no domain, so clearing the
// domain stores NULL rather than an empty string.
func normalizeTenantDomain(domain *string) *string {
	if domain == nil || *domain == "" {
		return nil
	}
	return domain
}

// sameTenantDomain reports whether two stored domains are equal, nil and ""
// both meaning no domain.
func sameTenantDomain(a, b *string) bool {
	return strings.EqualFold(stringOrEmpty(a), stringOrEmpty(b))
}

func stringOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
