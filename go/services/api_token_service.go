package services

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// MaxActiveAPITokensPerUser caps how many usable tokens one account may hold.
// It bounds what a single account can mint, and a household has no use for
// more.
const MaxActiveAPITokensPerUser = 50

// MaxAPITokenNameLength matches the api_tokens.name column.
const MaxAPITokenNameLength = 100

var (
	// ErrAPITokenInvalid is the one error every failed token authentication
	// returns, whatever the cause, so a caller cannot tell an unknown token
	// from a revoked or expired one. The cause is attached as the "reason"
	// attribute for server-side logs; see APITokenFailureReason.
	ErrAPITokenInvalid = errx.NewSentinel("invalid API token")

	// ErrAPITokenNameRequired rejects an empty or whitespace-only name.
	ErrAPITokenNameRequired = errx.NewSentinel("token name is required")
	// ErrAPITokenNameTooLong rejects a name over MaxAPITokenNameLength.
	ErrAPITokenNameTooLong = errx.NewSentinel("token name is too long")
	// ErrAPITokenInvalidScope rejects a scope other than read_only/read_write.
	ErrAPITokenInvalidScope = errx.NewSentinel("token scope must be read_only or read_write")
	// ErrAPITokenExpiryInPast rejects an expiry that is not in the future.
	ErrAPITokenExpiryInPast = errx.NewSentinel("token expiry must be in the future")
	// ErrAPITokenGroupNotAllowed rejects a group restriction the creator
	// cannot hold: a group in another tenant, one they are not a member of,
	// or one that is not active.
	ErrAPITokenGroupNotAllowed = errx.NewSentinel("you are not a member of that group")
	// ErrAPITokenLimitReached rejects creation past MaxActiveAPITokensPerUser.
	ErrAPITokenLimitReached = errx.NewSentinel("too many active API tokens")
)

// Reasons attached to ErrAPITokenInvalid. They are for logs and tests only and
// must never reach the response.
const (
	APITokenReasonMalformed      = "malformed"
	APITokenReasonUnknown        = "unknown"
	APITokenReasonRevoked        = "revoked"
	APITokenReasonExpired        = "expired"
	APITokenReasonUserNotFound   = "user_not_found"
	APITokenReasonUserInactive   = "user_inactive"
	APITokenReasonTenantMismatch = "tenant_mismatch"
)

// APITokenFailureReason returns the reason attached to an ErrAPITokenInvalid,
// or "" for any other error.
func APITokenFailureReason(err error) string {
	if !errors.Is(err, ErrAPITokenInvalid) {
		return ""
	}
	attrs := errx.ExtractAttrs(err)
	for _, attr := range attrs {
		if attr.Key == "reason" {
			if s, ok := attr.Value.(string); ok {
				return s
			}
		}
	}
	return ""
}

func invalidAPIToken(reason string) error {
	return errxtrace.Classify(ErrAPITokenInvalid, errx.Attrs("reason", reason))
}

// CreateAPITokenInput is what a user chooses when creating a token.
type CreateAPITokenInput struct {
	Name      string
	Scope     models.APITokenScope
	GroupID   *string
	ExpiresAt *time.Time
}

// APITokenService owns the lifecycle of personal API tokens: creation,
// listing, revocation and authentication. It has no HTTP dependency; the
// apiserver handlers and middleware add the request-bound parts (audit IP and
// user agent, rate limits, route rules).
type APITokenService struct {
	tokens registry.APITokenRegistry
	users  registry.UserRegistry
	groups *GroupService
	now    func() time.Time
}

// NewAPITokenService wires the service. groups may be nil only in tests that
// never create a group-restricted token.
func NewAPITokenService(tokens registry.APITokenRegistry, users registry.UserRegistry, groups *GroupService) *APITokenService {
	return &APITokenService{tokens: tokens, users: users, groups: groups, now: time.Now}
}

// SetClock replaces the time source. Tests use it to move past an expiry or
// the last-used interval without sleeping.
func (s *APITokenService) SetClock(now func() time.Time) {
	s.now = now
}

// ValidateCreate runs every check Create runs, without creating anything.
// The handler calls it before asking for a one-time second factor, so a typo
// in the name does not burn the user's code.
func (s *APITokenService) ValidateCreate(ctx context.Context, owner *models.User, in CreateAPITokenInput) error {
	_, err := s.normalize(ctx, owner, in)
	return err
}

// Create mints a token for owner and returns the stored row together with
// the token itself, which is never stored and cannot be recovered later.
// The caller is responsible for deciding that owner may create a token at all
// (a browser session, not a token or an impersonation session).
func (s *APITokenService) Create(ctx context.Context, owner *models.User, in CreateAPITokenInput) (*models.APIToken, string, error) {
	n, err := s.normalize(ctx, owner, in)
	if err != nil {
		return nil, "", err
	}

	gen, err := models.GenerateAPIToken()
	if err != nil {
		return nil, "", errxtrace.Wrap("failed to generate api token", err)
	}
	created, err := s.tokens.Create(ctx, models.APIToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: owner.TenantID,
			UserID:   owner.ID,
		},
		Name:        n.name,
		TokenHash:   gen.Hash,
		TokenPrefix: gen.DisplayPrefix,
		Scope:       n.scope,
		GroupID:     n.groupID,
		ExpiresAt:   n.expiresAt,
	})
	if err != nil {
		return nil, "", errxtrace.Wrap("failed to store api token", err)
	}
	return created, gen.Token, nil
}

type normalizedAPITokenInput struct {
	name      string
	scope     models.APITokenScope
	groupID   *string
	expiresAt *time.Time
}

func (s *APITokenService) normalize(ctx context.Context, owner *models.User, in CreateAPITokenInput) (normalizedAPITokenInput, error) {
	var n normalizedAPITokenInput
	n.name = strings.TrimSpace(in.Name)
	if n.name == "" {
		return n, ErrAPITokenNameRequired
	}
	if utf8.RuneCountInString(n.name) > MaxAPITokenNameLength {
		return n, ErrAPITokenNameTooLong
	}
	if !in.Scope.IsValid() {
		return n, ErrAPITokenInvalidScope
	}
	n.scope = in.Scope
	now := s.now()
	if in.ExpiresAt != nil {
		if !in.ExpiresAt.After(now) {
			return n, ErrAPITokenExpiryInPast
		}
		n.expiresAt = new(in.ExpiresAt.UTC())
	}

	if in.GroupID != nil && strings.TrimSpace(*in.GroupID) != "" {
		id := strings.TrimSpace(*in.GroupID)
		if err := s.checkGroupRestriction(ctx, owner, id); err != nil {
			return n, err
		}
		n.groupID = new(id)
	}

	active, err := s.tokens.CountActiveByUserID(ctx, owner.ID, now)
	if err != nil {
		return n, errxtrace.Wrap("failed to count api tokens", err)
	}
	if active >= MaxActiveAPITokensPerUser {
		return n, ErrAPITokenLimitReached
	}
	return n, nil
}

func (s *APITokenService) checkGroupRestriction(ctx context.Context, owner *models.User, groupID string) error {
	if s.groups == nil {
		return ErrAPITokenGroupNotAllowed
	}
	group, err := s.groups.GetGroup(ctx, groupID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			return ErrAPITokenGroupNotAllowed
		}
		return errxtrace.Wrap("failed to load group for api token", err)
	}
	if group.TenantID != owner.TenantID || !group.IsActive() {
		return ErrAPITokenGroupNotAllowed
	}
	isMember, err := s.groups.CheckGroupMembership(ctx, group.ID, owner.ID)
	if err != nil {
		return errxtrace.Wrap("failed to check group membership for api token", err)
	}
	if !isMember {
		return ErrAPITokenGroupNotAllowed
	}
	return nil
}

// List returns the user's tokens that are not revoked, newest first.
func (s *APITokenService) List(ctx context.Context, userID string) ([]*models.APIToken, error) {
	return s.tokens.ListByUserID(ctx, userID)
}

// Revoke revokes one of the user's tokens. A token id of another user answers
// registry.ErrNotFound, the same as an id that does not exist.
func (s *APITokenService) Revoke(ctx context.Context, userID, id string) (*models.APIToken, error) {
	return s.tokens.RevokeByID(ctx, userID, id)
}

// RevokeAllForUser revokes every token of the user and returns how many were
// still live. Called on password change and reset, on disabling MFA, and when
// an account is blocked or deactivated.
func (s *APITokenService) RevokeAllForUser(ctx context.Context, userID string) (int, error) {
	return s.tokens.RevokeAllByUserID(ctx, userID)
}

// Authenticate resolves a bearer credential to its token and owner. Every
// check runs on every request: the token must exist, not be revoked or
// expired, and belong to an active user whose tenant is the token's tenant and
// the tenant the request addresses (requestTenantID; empty skips that one
// comparison). Any failure is ErrAPITokenInvalid with a reason attribute.
func (s *APITokenService) Authenticate(ctx context.Context, credential, requestTenantID string) (*models.APIToken, *models.User, error) {
	if !models.IsWellFormedAPIToken(credential) {
		return nil, nil, invalidAPIToken(APITokenReasonMalformed)
	}
	hash := models.HashAPIToken(credential)
	token, err := s.tokens.GetByTokenHash(ctx, hash)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			return nil, nil, invalidAPIToken(APITokenReasonUnknown)
		}
		return nil, nil, errxtrace.Wrap("failed to look up api token", err)
	}
	// The row was found through a unique index on this same digest, so this
	// compare cannot fail on a correct database. It is here so the final
	// decision does not rest on how the index compared strings, and it runs
	// in constant time so it adds no timing signal of its own.
	if subtle.ConstantTimeCompare([]byte(token.TokenHash), []byte(hash)) != 1 {
		return nil, nil, invalidAPIToken(APITokenReasonUnknown)
	}

	now := s.now()
	if token.IsRevoked() {
		return nil, nil, invalidAPIToken(APITokenReasonRevoked)
	}
	if token.IsExpired(now) {
		return nil, nil, invalidAPIToken(APITokenReasonExpired)
	}

	user, err := s.users.Get(ctx, token.UserID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			return nil, nil, invalidAPIToken(APITokenReasonUserNotFound)
		}
		return nil, nil, errxtrace.Wrap("failed to load api token owner", err)
	}
	if !user.IsActive {
		return nil, nil, invalidAPIToken(APITokenReasonUserInactive)
	}
	if user.TenantID != token.TenantID || (requestTenantID != "" && requestTenantID != token.TenantID) {
		return nil, nil, invalidAPIToken(APITokenReasonTenantMismatch)
	}
	return token, user, nil
}

// RecordUse writes last_used_at when the stored value is older than
// models.APITokenLastUsedInterval, and reports whether it wrote. Most requests
// write nothing. The caller audits a use only when this returns true, so the
// audit trail gets one row per token per interval.
func (s *APITokenService) RecordUse(ctx context.Context, token *models.APIToken) (bool, error) {
	now := s.now()
	staleBefore := now.Add(-models.APITokenLastUsedInterval)
	if token.LastUsedAt != nil && !token.LastUsedAt.Before(staleBefore) {
		return false, nil
	}
	return s.tokens.TouchLastUsed(ctx, token.ID, now, staleBefore)
}
