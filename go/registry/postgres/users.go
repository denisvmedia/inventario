package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

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
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "User", "entity_id", user.GetID()))
		}
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

// ListAdminByTenant returns paginated, filtered, sorted users belonging
// to the given tenant for the `/api/v1/admin/tenants/{tenantID}/users`
// listing (#1746) along with the per-row group_membership_count
// computed from a correlated subquery on group_memberships.
//
// The endpoint crosses tenants by design — the admin caller may not be
// a member of the target tenant. The query runs inside store.DoAsAdmin,
// under the inventario_admin role, whose BYPASSRLS attribute skips the
// tenant-isolation policy on `users` and `group_memberships` for this
// transaction only. inventario_app traffic never assumes that role and
// stays RLS-enforced (see TenantRegistry.ListAdmin for the same model).
//
// Total is post-filter, pre-pagination, matching TenantRegistry.ListAdmin.
func (r *UserRegistry) ListAdminByTenant(ctx context.Context, tenantID string, opts registry.AdminUserListOptions) ([]*registry.AdminUserListItem, int, error) {
	if tenantID == "" {
		return nil, 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	page := opts.Page
	if page <= 0 {
		page = 1
	}
	perPage := opts.PerPage
	if perPage <= 0 {
		perPage = 50
	}

	sortField := opts.SortField
	if !sortField.IsValid() {
		sortField = registry.AdminUserSortEmail
	}
	direction := "ASC"
	if opts.SortDesc {
		direction = "DESC"
	}

	usersTable := r.tableNames.Users()
	membershipsTable := r.tableNames.GroupMemberships()

	args := []any{tenantID}
	whereClauses := []string{"u.tenant_id = $1"}
	if q := strings.TrimSpace(opts.Query); q != "" {
		args = append(args, "%"+q+"%")
		// $2 reused across email + name.
		whereClauses = append(whereClauses, fmt.Sprintf("(u.email ILIKE $%d OR u.name ILIKE $%d)", len(args), len(args)))
	}
	if opts.IsActive != nil {
		args = append(args, *opts.IsActive)
		whereClauses = append(whereClauses, fmt.Sprintf("u.is_active = $%d", len(args)))
	}
	where := "WHERE " + strings.Join(whereClauses, " AND ")

	var (
		items []*registry.AdminUserListItem
		total int
	)
	err := store.DoAsAdmin(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s AS u %s", usersTable, where)
		if scanErr := tx.QueryRowContext(ctx, countQuery, args...).Scan(&total); scanErr != nil {
			return errxtrace.Wrap("failed to count admin users", scanErr)
		}

		limitPos := len(args) + 1
		offsetPos := len(args) + 2
		offset := (page - 1) * perPage

		// SECURITY: sortField is constrained to AdminUserSortField via IsValid above,
		// direction is "ASC"/"DESC" literals, and table-names come from r.tableNames —
		// never user-supplied — so direct fmt.Sprintf interpolation is safe.
		// The membership COUNT joins on (tenant_id, member_user_id) — the
		// tenant predicate is belt-and-braces: today the user.id PK is
		// globally unique, but tenant-scoping the join keeps the count
		// honest if id-reuse-across-tenants ever becomes possible (e.g.
		// a future tenant-import flow) and matches the
		// (tenant_id, member_user_id) shape ListByUser uses elsewhere.
		pageQuery := fmt.Sprintf(`
			SELECT u.*,
				(SELECT COUNT(*) FROM %s AS m WHERE m.member_user_id = u.id AND m.tenant_id = u.tenant_id) AS _group_membership_count
			FROM %s AS u
			%s
			ORDER BY u.%s %s, u.id ASC
			LIMIT $%d OFFSET $%d`,
			membershipsTable, usersTable, where, string(sortField), direction, limitPos, offsetPos,
		)
		pageArgs := append(append([]any{}, args...), perPage, offset)

		rows, err := tx.QueryxContext(ctx, pageQuery, pageArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to list admin users", err)
		}
		defer rows.Close()

		for rows.Next() {
			var row struct {
				models.User
				GroupMembershipCount int `db:"_group_membership_count"`
			}
			if scanErr := rows.StructScan(&row); scanErr != nil {
				return errxtrace.Wrap("failed to scan admin user row", scanErr)
			}
			user := row.User
			items = append(items, &registry.AdminUserListItem{
				User:                 &user,
				GroupMembershipCount: row.GroupMembershipCount,
			})
		}
		if rowsErr := rows.Err(); rowsErr != nil {
			return errxtrace.Wrap("failed during admin user row iteration", rowsErr)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

// CountSessionsByUser returns the number of refresh_tokens rows for the
// user that are neither revoked nor expired. Backs the
// `active_session_count` field on the admin user-detail endpoint
// (#1746). The lookup crosses tenants intentionally — the admin caller
// may not be a member of the target user's tenant — and runs inside
// store.DoAsAdmin, under the inventario_admin (BYPASSRLS) role, so the
// refresh_tokens RLS policy is skipped for this transaction only (see
// TenantRegistry.ListAdmin for the same model).
//
// Note on the handler contract: admin handler degrades a CountSessionsByUser
// failure to 0 + a separate audit row (admin.get_user_sessions, success=false)
// rather than 500-ing the whole user-detail endpoint, so audit consumers
// must correlate by ActorID/timestamp to distinguish "genuine 0
// sessions" from "session-count registry hiccup".
func (r *UserRegistry) CountSessionsByUser(ctx context.Context, userID string) (int, error) {
	if userID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	var count int
	err := store.DoAsAdmin(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s WHERE user_id = $1 AND revoked_at IS NULL AND expires_at > $2`,
			r.tableNames.RefreshTokens(),
		)
		if scanErr := tx.QueryRowContext(ctx, query, userID, time.Now()).Scan(&count); scanErr != nil {
			return errxtrace.Wrap("failed to count active sessions", scanErr)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
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
