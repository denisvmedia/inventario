// Package backoffice exposes a thin service layer over
// BackofficeUserRegistry for the Phase 1 CLI (issue #1785). Kept
// intentionally narrow: only what the bootstrap subcommand needs. Phase
// 2 (HTTP login) and Phase 3 (admin surface) will grow this package
// rather than retrofitting more behaviour onto the registry itself.
package backoffice

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// Service wraps BackofficeUserRegistry with the business-rule logic
// the bootstrap CLI needs (password hashing, idempotent "already
// exists" detection, fresh-deployment refusal-unless-force).
type Service struct {
	factorySet *registry.FactorySet
	cleanup    func() error
}

// NewService constructs a Service backed by the postgres registry.
// dbConfig.Validate already rejects non-postgres DSNs (memory://,
// file://, etc.), so by the time control reaches the registry lookup
// the DSN is guaranteed to be postgres. Mirrors services/admin.NewService.
func NewService(dbConfig *shared.DatabaseConfig) (*Service, error) {
	if err := dbConfig.Validate(); err != nil {
		return nil, fmt.Errorf("database configuration error: %w", err)
	}

	registryFunc, ok := registry.GetRegistry(dbConfig.DBDSN)
	if !ok {
		return nil, fmt.Errorf("unsupported database type in DSN: %s", dbConfig.DBDSN)
	}

	factorySet, err := registryFunc(registry.Config(dbConfig.DBDSN))
	if err != nil {
		return nil, fmt.Errorf("failed to create registry factory set: %w", err)
	}

	return &Service{
		factorySet: factorySet,
		cleanup:    nil,
	}, nil
}

// Close releases the underlying database connections, if any. Safe to
// call on a service constructed without a cleanup hook.
func (s *Service) Close() error {
	if s.cleanup != nil {
		return s.cleanup()
	}
	return nil
}

// BootstrapRequest carries the inputs to Service.Bootstrap.
type BootstrapRequest struct {
	Email    string
	Name     string
	Role     models.BackofficeRole
	Password string // optional; auto-generated when empty
	Force    bool   // allow creating an additional user when the table is non-empty
}

// BootstrapResult captures the outcome of a Bootstrap call.
type BootstrapResult struct {
	User              *models.BackofficeUser
	GeneratedPassword string // populated only when the caller did NOT supply Password
	AlreadyExisted    bool   // true when an existing user with the same email was found (idempotent re-run)
}

// Bootstrap creates the first (or, with --force, an additional) back-
// office user. The contract is:
//
//   - If a row already exists with the supplied email, return
//     AlreadyExisted=true with no error and no mutation. The CLI maps
//     this to "ℹ️  user already exists" + exit 0 so re-running the
//     command is safe.
//   - Else, if at least one back-office user already exists AND the
//     caller did not pass Force, refuse with a fail-closed error. The
//     CLI surfaces this with a hint to pass --force.
//   - Else, generate (or accept) a password, bcrypt it at DefaultCost
//     (matching models.User.SetPassword), and insert the row.
//
// Pre-existence is checked by email FIRST (idempotency) and only then
// by global count (force gate). Order matters: re-running the same
// invocation should never trip the force gate.
func (s *Service) Bootstrap(ctx context.Context, req BootstrapRequest) (*BootstrapResult, error) {
	if s.factorySet == nil || s.factorySet.BackofficeUserRegistry == nil {
		return nil, errors.New("backoffice user registry not configured")
	}

	email := strings.TrimSpace(req.Email)
	if email == "" {
		return nil, errors.New("email is required")
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, errors.New("name is required")
	}
	role := req.Role
	if role == "" {
		role = models.BackofficeRolePlatformAdmin
	}
	if !role.IsValid() {
		return nil, fmt.Errorf("invalid role %q (must be one of: support_agent, platform_admin)", string(role))
	}

	// 1. Idempotency check by email. GetByEmail is case-insensitive at
	// the registry layer, so re-running with the same email in a
	// different case still hits the existing row.
	existing, lookupErr := s.factorySet.BackofficeUserRegistry.GetByEmail(ctx, email)
	switch {
	case lookupErr == nil:
		return &BootstrapResult{User: existing, AlreadyExisted: true}, nil
	case errors.Is(lookupErr, registry.ErrBackofficeUserNotFound):
		// expected — fall through to count + insert
	default:
		return nil, errxtrace.Wrap("failed to check existing backoffice user", lookupErr)
	}

	// 2. Force gate. Counting is cheap on a tiny table; doing it under a
	// transaction is overkill for a CLI run that has no concurrent writer.
	if !req.Force {
		count, countErr := s.factorySet.BackofficeUserRegistry.Count(ctx)
		if countErr != nil {
			return nil, errxtrace.Wrap("failed to count existing backoffice users", countErr)
		}
		if count > 0 {
			return nil, fmt.Errorf("a backoffice user already exists; pass --force to add another (current count: %d)", count)
		}
	}

	// 3. Password: explicit or generated. Generated passwords are
	// returned to the CLI so the operator can copy them once; the
	// service never stores or logs the plaintext.
	password := req.Password
	var generated string
	if password == "" {
		gp, genErr := generatePassword()
		if genErr != nil {
			return nil, errxtrace.Wrap("failed to generate password", genErr)
		}
		password = gp
		generated = gp
	}

	if err := models.ValidatePassword(password); err != nil {
		return nil, fmt.Errorf("password validation failed: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errxtrace.Wrap("failed to bcrypt password", err)
	}

	user := models.BackofficeUser{
		Email:        email,
		Name:         name,
		PasswordHash: string(hash),
		Role:         role,
		IsActive:     true,
		MFAEnforced:  true,
	}

	// Run the model's full validation (length caps, EmailPattern match)
	// before handing the row to the registry. The registry's
	// validateCommonBackofficeFields only checks "not empty" + role
	// membership, so without this an invocation like
	// `--email "not-an-email"` would round-trip cleanly. Mirrors
	// services/admin.Service.CreateUser.
	if err := user.ValidateWithContext(ctx); err != nil {
		return nil, fmt.Errorf("user validation failed: %w", err)
	}

	created, err := s.factorySet.BackofficeUserRegistry.Create(ctx, user)
	if err != nil {
		// A concurrent caller could have inserted the same email
		// between our idempotency check and this Create. Surface
		// the canonical "already exists" sentinel so the CLI can
		// render the same friendly message either way.
		if errors.Is(err, registry.ErrBackofficeEmailAlreadyExists) {
			refetched, refErr := s.factorySet.BackofficeUserRegistry.GetByEmail(ctx, email)
			if refErr == nil {
				return &BootstrapResult{User: refetched, AlreadyExisted: true}, nil
			}
		}
		return nil, errxtrace.Wrap("failed to create backoffice user", err)
	}

	return &BootstrapResult{
		User:              created,
		GeneratedPassword: generated,
	}, nil
}

// generatePassword produces a strong random password the CLI prints
// once to stdout. 18 bytes of base64 = 24 ASCII chars with mixed-case
// + digits, comfortably above models.ValidatePassword's length
// requirement.
//
// The "Aa1" suffix is load-bearing: a 24-char random base64 string has
// a non-trivial probability of missing at least one of the upper /
// lower / digit character classes models.ValidatePassword requires.
// The suffix is what guarantees the validation check passes — not a
// safety net for an unlikely draw.
func generatePassword() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf) + "Aa1", nil
}
