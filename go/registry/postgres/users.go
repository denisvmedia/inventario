package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.UserRegistry = (*UserRegistry)(nil)

type UserRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewUserRegistry(dbx *sqlx.DB) *UserRegistry {
	return NewUserRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewUserRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *UserRegistry {
	return &UserRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *UserRegistry) newSQLRegistry() *store.NonRLSRepository[models.User, *models.User] {
	return store.NewSQLRegistry[models.User](r.dbx, r.tableNames.Users())
}

func (r *UserRegistry) Create(ctx context.Context, user models.User) (*models.User, error) {
	if user.Email == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}

	if user.Name == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}

	if user.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	// We need to handle user creation specially because of the self-referencing foreign key
	// We'll create the user with a custom implementation that handles the UserID properly

	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errxtrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, store.RollbackOrCommit(tx, err))
	}()

	// Generate a new server-side ID for security (ignore any user-provided ID)
	generatedID := uuid.New().String()
	user.ID = generatedID
	if user.UUID == "" {
		user.UUID = uuid.New().String()
	}

	// The legacy users.user_id self-FK was removed by issue #1289 Gap B —
	// the row's own id column is authoritative, so nothing else to populate.

	// Check if a user with the same email already exists (within the same tenant)
	var existingUser models.User
	txReg := store.NewTxRegistry[models.User](tx, r.tableNames.Users())
	err = txReg.ScanOneByFields(ctx, []store.FieldValue{
		store.Pair("tenant_id", user.TenantID),
		store.Pair("email", user.Email),
	}, &existingUser)
	if err == nil {
		return nil, errxtrace.Classify(registry.ErrEmailAlreadyExists, errx.Attrs("email", user.Email))
	} else if !errors.Is(err, store.ErrNotFound) {
		return nil, errxtrace.Wrap("failed to check for existing user", err)
	}

	// Insert the user
	err = txReg.Insert(ctx, user)
	if err != nil {
		return nil, errxtrace.Wrap("failed to insert user", err)
	}

	return &user, nil
}

func (r *UserRegistry) Get(ctx context.Context, id string) (*models.User, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var user models.User
	reg := r.newSQLRegistry()
	err := reg.ScanOneByField(ctx, store.Pair("id", id), &user)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "User",
				"entity_id", id,
			))
		}
		return nil, errxtrace.Wrap("failed to get entity", err)
	}

	return &user, nil
}

func (r *UserRegistry) List(ctx context.Context) ([]*models.User, error) {
	var users []*models.User

	reg := r.newSQLRegistry()

	// Query the database for all users (atomic operation)
	for user, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list users", err)
		}
		users = append(users, &user)
	}

	return users, nil
}

func (r *UserRegistry) Update(ctx context.Context, user models.User) (*models.User, error) {
	if user.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	if user.Email == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}

	if user.Name == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Name"))
	}

	if user.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, user, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update user", err)
	}

	return &user, nil
}

func (r *UserRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errxtrace.Wrap("failed to delete user", err)
	}

	return nil
}

func (r *UserRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count users", err)
	}

	return count, nil
}

// GetByEmail returns a user by email within a tenant
func (r *UserRegistry) GetByEmail(ctx context.Context, tenantID, email string) (*models.User, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	if email == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Email"))
	}

	reg := r.newSQLRegistry()

	// Use Do to execute custom query logic
	var user models.User
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE tenant_id = $1 AND email = $2`, r.tableNames.Users())
		err := tx.GetContext(ctx, &user, query, tenantID, email)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
					"entity_type", "User",
					"tenant_id", tenantID,
					"email", email,
				))
			}
			return errxtrace.Wrap("failed to get user by email", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &user, nil
}

// RevokeSystemAdminAtomic clears is_system_admin on the target user
// inside a transaction that also takes a global pg_advisory_xact_lock on
// the system-admin lock space. Serialising all system-admin mutations
// through one lock is sufficient (and far simpler than per-row locking)
// because grant/revoke writes are rare CLI events — contention is not a
// concern, correctness is. When allowZero=false, the lock guarantees
// that the count check ("are there other admins?") and the UPDATE happen
// atomically: a concurrent revoke either commits first (and our count
// then returns <=1, blocking us) or blocks until our COMMIT (and then
// sees the new count). Idempotent: a non-admin user returns (false, nil)
// with no row touched.
//
//revive:disable-next-line:flag-parameter
func (r *UserRegistry) RevokeSystemAdminAtomic(ctx context.Context, userID string, allowZero bool) (hadFlag bool, err error) {
	if userID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()
	err = reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Single-keyspace advisory lock: every system-admin mutation
		// serialises through this lock so the count+update is atomic.
		if _, lockErr := tx.ExecContext(ctx,
			`SELECT pg_advisory_xact_lock(hashtext('system_admin_mutations'))`,
		); lockErr != nil {
			return errxtrace.Wrap("failed to acquire system-admin advisory lock", lockErr)
		}

		// FOR UPDATE pins the target row so any concurrent direct
		// UPDATE on the same user blocks on us — defense-in-depth in
		// case a future code path bypasses this method.
		var isAdmin bool
		query := fmt.Sprintf(`SELECT is_system_admin FROM %s WHERE id = $1 FOR UPDATE`, r.tableNames.Users())
		if scanErr := tx.QueryRowContext(ctx, query, userID).Scan(&isAdmin); scanErr != nil {
			if errors.Is(scanErr, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
					"entity_type", "User",
					"entity_id", userID,
				))
			}
			return errxtrace.Wrap("failed to lock user row for revoke", scanErr)
		}

		if !isAdmin {
			// Idempotent — already non-admin.
			return nil
		}

		hadFlag = true

		if !allowZero {
			var count int
			countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE is_system_admin = true`, r.tableNames.Users())
			if countErr := tx.QueryRowContext(ctx, countQuery).Scan(&count); countErr != nil {
				return errxtrace.Wrap("failed to count system admins under lock", countErr)
			}
			if count <= 1 {
				return errxtrace.Classify(registry.ErrLastSystemAdmin, errx.Attrs(
					"user_id", userID,
				))
			}
		}

		updateQuery := fmt.Sprintf(`UPDATE %s SET is_system_admin = false, updated_at = now() WHERE id = $1`, r.tableNames.Users())
		if _, updErr := tx.ExecContext(ctx, updateQuery, userID); updErr != nil {
			return errxtrace.Wrap("failed to clear is_system_admin", updErr)
		}
		return nil
	})
	if err != nil {
		return hadFlag, err
	}
	return hadFlag, nil
}

// ListSystemAdmins returns every user with is_system_admin = true.
// Backed by the partial index `users_system_admin_idx` so the scan is
// O(matches) regardless of the total user count.
func (r *UserRegistry) ListSystemAdmins(ctx context.Context) ([]*models.User, error) {
	reg := r.newSQLRegistry()

	var admins []*models.User
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE is_system_admin = true ORDER BY created_at ASC`, r.tableNames.Users())
		rows, err := tx.QueryxContext(ctx, query)
		if err != nil {
			return errxtrace.Wrap("failed to list system admins", err)
		}
		defer rows.Close()
		for rows.Next() {
			var u models.User
			if err := rows.StructScan(&u); err != nil {
				return errxtrace.Wrap("failed to scan system admin row", err)
			}
			admins = append(admins, &u)
		}
		if err := rows.Err(); err != nil {
			return errxtrace.Wrap("failed during system admin row iteration", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return admins, nil
}

// ListByTenant returns all users for a tenant
func (r *UserRegistry) ListByTenant(ctx context.Context, tenantID string) ([]*models.User, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	var users []*models.User
	reg := r.newSQLRegistry()

	for user, err := range reg.ScanByField(ctx, store.Pair("tenant_id", tenantID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list users by tenant", err)
		}
		users = append(users, &user)
	}

	return users, nil
}
