package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.BackofficeUserRegistry = (*BackofficeUserRegistry)(nil)

// BackofficeUserRegistry is the postgres-backed implementation of the
// platform-operator identity store (issue #1785). The underlying table
// has NO row-level security enabled — same reasoning as `tenants`: a
// back-office identity IS the boundary, so the table is wrapped by a
// NonRLSRepository and access is gated entirely at the application
// layer (Phase 2 / #1785).
//
// Email is unique platform-wide and is lowercased at the registry layer
// before INSERT and before SELECT, so case variants collapse to one row.
// The schema annotations don't express functional indexes, so the
// regular UNIQUE INDEX on `email` only catches duplicates when callers
// route through this registry.
type BackofficeUserRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewBackofficeUserRegistry returns a postgres-backed BackofficeUserRegistry
// using the default table-name set.
func NewBackofficeUserRegistry(dbx *sqlx.DB) *BackofficeUserRegistry {
	return NewBackofficeUserRegistryWithTableNames(dbx, store.DefaultTableNames)
}

// NewBackofficeUserRegistryWithTableNames lets tests override the table
// names (per the same pattern used by every other postgres registry).
func NewBackofficeUserRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *BackofficeUserRegistry {
	return &BackofficeUserRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *BackofficeUserRegistry) newSQLRegistry() *store.NonRLSRepository[models.BackofficeUser, *models.BackofficeUser] {
	return store.NewSQLRegistry[models.BackofficeUser, *models.BackofficeUser](r.dbx, r.tableNames.BackofficeUsers())
}

// Create validates required fields, lowercases the email, and rejects
// duplicates platform-wide inside the same transaction as the INSERT so
// two concurrent calls with the same email both fail-closed (one
// commits, the second rolls back on the unique-index violation; the
// pre-check turns "lost insert" into the friendlier
// ErrBackofficeEmailAlreadyExists sentinel).
//
// CreatedAt / UpdatedAt are stamped here (when zero) — the store layer's
// Insert (txexec.go) uses typekit.ExtractDBFields which includes every
// db-tagged column, so leaving them at zero would override the column
// DEFAULT CURRENT_TIMESTAMP and persist year-0001 timestamps. Mirrors
// the pattern in email_verifications / commodity_loans / login_events /
// user_mfa_secrets. The IsZero guard lets tests that pin a specific
// timestamp keep their override.
func (r *BackofficeUserRegistry) Create(ctx context.Context, user models.BackofficeUser) (*models.BackofficeUser, error) {
	if err := r.validateForCreate(user); err != nil {
		return nil, err
	}
	// Defence-in-depth: re-run the model's full validation so format/
	// length constraints (EmailPattern, max lengths, closed-set role)
	// fail closed even if a future caller bypasses Service.Bootstrap.
	// The registry's bespoke validateForCreate runs first so the existing
	// ErrFieldRequired / ErrInvalidBackofficeRole sentinels keep their
	// identity for callers that branch on them.
	if err := user.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("backoffice user failed model validation", err)
	}
	user.Email = normaliseBackofficeEmail(user.Email)
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	if user.UpdatedAt.IsZero() {
		user.UpdatedAt = now
	}
	if user.LastLoginAt != nil && user.LastLoginAt.IsZero() {
		user.LastLoginAt = nil
	}

	reg := r.newSQLRegistry()
	created, err := reg.Create(ctx, user, func(ctx context.Context, tx *sqlx.Tx) error {
		var existing models.BackofficeUser
		txReg := store.NewTxRegistry[models.BackofficeUser](tx, r.tableNames.BackofficeUsers())
		err := txReg.ScanOneByField(ctx, store.Pair("email", user.Email), &existing)
		if err == nil {
			return errxtrace.Classify(registry.ErrBackofficeEmailAlreadyExists, errx.Attrs("email", user.Email))
		}
		if !errors.Is(err, store.ErrNotFound) {
			return errxtrace.Wrap("failed to check for existing backoffice user", err)
		}
		return nil
	})
	if err != nil {
		// Close the race window between the pre-SELECT and the INSERT
		// by re-classifying a Postgres unique-violation on the email
		// index as ErrBackofficeEmailAlreadyExists so callers (and
		// Service.Bootstrap's idempotent-rerun branch) can branch on
		// the canonical sentinel even when the loser of a parallel
		// Create surfaces the SQLSTATE rather than the application-
		// level pre-check.
		if isBackofficeEmailUniqueViolation(err) {
			return nil, errxtrace.Classify(registry.ErrBackofficeEmailAlreadyExists, errx.Attrs("email", user.Email))
		}
		return nil, errxtrace.Wrap("failed to create backoffice user", err)
	}
	return &created, nil
}

// Get returns the row by id. Maps store.ErrNotFound to the back-office-
// specific sentinel so callers don't have to discriminate at the call
// site.
func (r *BackofficeUserRegistry) Get(ctx context.Context, id string) (*models.BackofficeUser, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var user models.BackofficeUser
	reg := r.newSQLRegistry()
	if err := reg.ScanOneByField(ctx, store.Pair("id", id), &user); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", id))
		}
		return nil, errxtrace.Wrap("failed to get backoffice user", err)
	}
	return &user, nil
}

// GetByEmail performs a case-insensitive lookup by lowercased email.
// The registry layer normalises the email both on write and read, so a
// regular `email = $1` predicate is enough — no `lower(email)` index
// required. Whitespace-only input is treated as empty so a stray "   "
// from a caller doesn't fall through to a no-rows lookup.
func (r *BackofficeUserRegistry) GetByEmail(ctx context.Context, email string) (*models.BackofficeUser, error) {
	if strings.TrimSpace(email) == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}
	normalised := normaliseBackofficeEmail(email)

	var user models.BackofficeUser
	reg := r.newSQLRegistry()
	if err := reg.ScanOneByField(ctx, store.Pair("email", normalised), &user); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("email", normalised))
		}
		return nil, errxtrace.Wrap("failed to get backoffice user by email", err)
	}
	return &user, nil
}

// List returns every back-office row in insertion order. Tiny table (a
// handful of platform operators), so no pagination is needed yet.
func (r *BackofficeUserRegistry) List(ctx context.Context) ([]*models.BackofficeUser, error) {
	var users []*models.BackofficeUser
	reg := r.newSQLRegistry()
	for user, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list backoffice users", err)
		}
		users = append(users, &user)
	}
	return users, nil
}

// Count returns the total number of back-office rows. Used by the
// bootstrap CLI to refuse a fresh create unless --force is passed.
func (r *BackofficeUserRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()
	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count backoffice users", err)
	}
	return count, nil
}

// Update revalidates required fields, lowercases the email, and writes
// the row back. The bcrypt hash + created_at + last_login_at are NOT
// part of the public Update surface — callers must go through
// SetPasswordHash / UpdateLastLogin / Create respectively, so we
// preserve those columns from the persisted row inside the same tx.
func (r *BackofficeUserRegistry) Update(ctx context.Context, user models.BackofficeUser) (*models.BackofficeUser, error) {
	if user.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := r.validateForUpdate(user); err != nil {
		return nil, err
	}
	// Defence-in-depth: re-run the model's full validation so format/
	// length constraints (EmailPattern, max lengths, closed-set role)
	// fail closed even when callers bypass the service layer. The
	// registry's bespoke validateForUpdate runs first so the existing
	// ErrFieldRequired / ErrInvalidBackofficeRole sentinels keep their
	// identity. PasswordHash isn't part of the public Update surface,
	// so model validation runs on a copy with an opaque non-empty hash
	// substituted in — the on-disk hash is restored further down.
	validateUser := user
	if validateUser.PasswordHash == "" {
		validateUser.PasswordHash = "validation-placeholder"
	}
	if err := validateUser.ValidateWithContext(ctx); err != nil {
		return nil, errxtrace.Wrap("backoffice user failed model validation", err)
	}
	user.Email = normaliseBackofficeEmail(user.Email)

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Load existing row so write-path-isolated columns survive a
		// partial-struct update from the future HTTP layer.
		var existing models.BackofficeUser
		txReg := store.NewTxRegistry[models.BackofficeUser](tx, r.tableNames.BackofficeUsers())
		if scanErr := txReg.ScanOneByField(ctx, store.Pair("id", user.GetID()), &existing); scanErr != nil {
			if errors.Is(scanErr, store.ErrNotFound) {
				return errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", user.GetID()))
			}
			return errxtrace.Wrap("failed to load backoffice user for update", scanErr)
		}

		// Cross-row email uniqueness — under the tx so a concurrent
		// Create can't slip in between the lookup and the UPDATE.
		var collision models.BackofficeUser
		lookupErr := txReg.ScanOneByField(ctx, store.Pair("email", user.Email), &collision)
		if lookupErr == nil && collision.ID != user.GetID() {
			return errxtrace.Classify(registry.ErrBackofficeEmailAlreadyExists, errx.Attrs("email", user.Email))
		}
		if lookupErr != nil && !errors.Is(lookupErr, store.ErrNotFound) {
			return errxtrace.Wrap("failed to check for backoffice email collision", lookupErr)
		}

		updateQuery := fmt.Sprintf(`
			UPDATE %s
			   SET email = $1,
			       name = $2,
			       role = $3,
			       is_active = $4,
			       mfa_enforced = $5,
			       updated_at = now()
			 WHERE id = $6`,
			r.tableNames.BackofficeUsers(),
		)
		res, execErr := tx.ExecContext(ctx, updateQuery,
			user.Email, user.Name, string(user.Role), user.IsActive, user.MFAEnforced, user.GetID(),
		)
		if execErr != nil {
			// Re-classify a Postgres unique-violation on the email
			// index as ErrBackofficeEmailAlreadyExists — mirrors the
			// Create path so callers get the canonical sentinel even
			// when a concurrent insert wins the race between this
			// transaction's pre-SELECT and its UPDATE.
			if isBackofficeEmailUniqueViolation(execErr) {
				return errxtrace.Classify(registry.ErrBackofficeEmailAlreadyExists, errx.Attrs("email", user.Email))
			}
			return errxtrace.Wrap("failed to update backoffice user", execErr)
		}
		// Defence-in-depth: even though the pre-SELECT above runs in the
		// same READ COMMITTED transaction, it doesn't take a row lock,
		// so a concurrent Delete could remove the row between the
		// ScanOneByField and this UPDATE. Checking RowsAffected closes
		// the window — without it, the call would return success on a
		// no-op write and the caller would see a stale view of the row.
		affected, raErr := res.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read rows affected for backoffice update", raErr)
		}
		if affected == 0 {
			return errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", user.GetID()))
		}

		// Replay preserved columns on the caller's struct so the
		// returned pointer is consistent with what's on disk.
		user.CreatedAt = existing.CreatedAt
		user.PasswordHash = existing.PasswordHash
		user.LastLoginAt = existing.LastLoginAt
		user.UpdatedAt = time.Now().UTC()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Delete removes the row by id. Idempotent — a Delete on a missing id
// is a no-op rather than an error, matching the memory backend and
// keeping the cross-backend contract uniform. NonRLSRepository.Delete
// returns a wrapped store.ErrNotFound when the row doesn't exist; we
// swallow it here so callers can re-run Delete safely.
func (r *BackofficeUserRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	reg := r.newSQLRegistry()
	if err := reg.Delete(ctx, id, nil); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil
		}
		return errxtrace.Wrap("failed to delete backoffice user", err)
	}
	return nil
}

// SetPasswordHash overwrites only the password_hash column on the
// target row. Kept off the generic Update path so the bcrypt hash never
// gets exposed through a full-row write.
func (r *BackofficeUserRegistry) SetPasswordHash(ctx context.Context, id, hash string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if hash == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "PasswordHash"))
	}

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`UPDATE %s SET password_hash = $1, updated_at = now() WHERE id = $2`, r.tableNames.BackofficeUsers())
		res, execErr := tx.ExecContext(ctx, query, hash, id)
		if execErr != nil {
			return errxtrace.Wrap("failed to set backoffice password hash", execErr)
		}
		affected, raErr := res.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read rows affected for backoffice password hash update", raErr)
		}
		if affected == 0 {
			return errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", id))
		}
		return nil
	})
	return err
}

// UpdateLastLogin stamps last_login_at + updated_at on the target row.
// Called by the Phase 2 login flow on each successful authentication.
func (r *BackofficeUserRegistry) UpdateLastLogin(ctx context.Context, id string, at time.Time) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`UPDATE %s SET last_login_at = $1, updated_at = now() WHERE id = $2`, r.tableNames.BackofficeUsers())
		res, execErr := tx.ExecContext(ctx, query, at, id)
		if execErr != nil {
			return errxtrace.Wrap("failed to update backoffice last_login_at", execErr)
		}
		affected, raErr := res.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read rows affected for backoffice last_login update", raErr)
		}
		if affected == 0 {
			return errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", id))
		}
		return nil
	})
	return err
}

// SetActive flips the is_active column. Used by the future back-office
// admin UI to suspend / restore a platform operator.
func (r *BackofficeUserRegistry) SetActive(ctx context.Context, id string, active bool) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`UPDATE %s SET is_active = $1, updated_at = now() WHERE id = $2`, r.tableNames.BackofficeUsers())
		res, execErr := tx.ExecContext(ctx, query, active, id)
		if execErr != nil {
			return errxtrace.Wrap("failed to set backoffice is_active", execErr)
		}
		affected, raErr := res.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read rows affected for backoffice is_active update", raErr)
		}
		if affected == 0 {
			return errxtrace.Classify(registry.ErrBackofficeUserNotFound, errx.Attrs("entity_id", id))
		}
		return nil
	})
	return err
}

// validateForCreate enforces the full required-field invariants for an
// insert: email, name, password_hash, and a valid role. Mirrors the
// memory backend's variant so both backends fail-closed on the same
// inputs.
func (r *BackofficeUserRegistry) validateForCreate(user models.BackofficeUser) error {
	if err := validateCommonBackofficeFields(user); err != nil {
		return err
	}
	if user.PasswordHash == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "PasswordHash"))
	}
	return nil
}

// validateForUpdate skips the PasswordHash check because Update
// restores the persisted hash from the stored row — callers must use
// SetPasswordHash to change the hash.
func (r *BackofficeUserRegistry) validateForUpdate(user models.BackofficeUser) error {
	return validateCommonBackofficeFields(user)
}

// validateCommonBackofficeFields covers the invariants shared by Create
// and Update — every column except the password hash, which only Create
// requires.
func validateCommonBackofficeFields(user models.BackofficeUser) error {
	if strings.TrimSpace(user.Email) == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}
	if strings.TrimSpace(user.Name) == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}
	if !user.Role.IsValid() {
		return errxtrace.Classify(registry.ErrInvalidBackofficeRole, errx.Attrs("role", string(user.Role)))
	}
	return nil
}

// normaliseBackofficeEmail lowercases + trims the email so case variants
// collapse to a single row. Mirrors the memory backend's identical
// helper — kept private per package to keep dependency direction clean.
func normaliseBackofficeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// backofficeEmailUniqueIndexName is the postgres unique index on the
// email column. The Create / Update paths translate violations on
// this index into ErrBackofficeEmailAlreadyExists so the loser of a
// concurrent insert race surfaces the same canonical sentinel as the
// application-level pre-check. Any OTHER unique violation (e.g. on
// the uuid index) is re-raised as-is — it would indicate a programmer
// error rather than a legitimate domain conflict.
const backofficeEmailUniqueIndexName = "idx_backoffice_users_email"

// isBackofficeEmailUniqueViolation reports whether err corresponds to a
// Postgres unique-violation (SQLSTATE 23505) on the backoffice email
// index. Mirrors the helpers in warranty_reminders / currency_migration
// — kept locally so this registry stays self-contained, and string-based
// for the SQLSTATE so we don't pull in lib/pq just for the constant.
func isBackofficeEmailUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	type sqlStater interface{ SQLState() string }
	type constrainter interface{ ConstraintName() string }

	var s sqlStater
	if !errors.As(err, &s) || s.SQLState() != "23505" {
		// Fall back to substring match against the index name when
		// the underlying error doesn't expose SQLState (defence in
		// depth — current drivers do, but the helper stays robust).
		return strings.Contains(err.Error(), backofficeEmailUniqueIndexName)
	}
	var c constrainter
	if errors.As(err, &c) && c.ConstraintName() == backofficeEmailUniqueIndexName {
		return true
	}
	return strings.Contains(err.Error(), backofficeEmailUniqueIndexName)
}
