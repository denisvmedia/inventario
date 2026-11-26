package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

var _ registry.UserTenantMover = (*UserTenantMover)(nil)

// UserTenantMover is the postgres implementation of registry.UserTenantMover
// (#2179). Every move runs in one background-worker transaction that starts by
// locking the users row FOR UPDATE. Inserting a row with a foreign key to
// users(id) takes FOR KEY SHARE on that row, which conflicts with FOR UPDATE,
// so no membership or content naming the user can be written between the count
// and the move. A membership written after the move is refused by the tenant
// check in GroupMembershipRegistry.
type UserTenantMover struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewUserTenantMover returns a mover bound to the default table names.
func NewUserTenantMover(dbx *sqlx.DB) *UserTenantMover {
	return &UserTenantMover{dbx: dbx, tableNames: store.DefaultTableNames}
}

// userTenantMoveDeletes are the per-user rows a move deletes. They are
// sessions and credentials (the user signs in again), one-time tokens issued
// for the old tenant, and transient or per-group state that only makes sense in
// the tenant the user leaves. Personal data exports are among them: a copy is
// built from the groups of the old tenant, and the export worker's hourly
// sweep deletes the archive once its row is gone. Children come before their
// parents.
var userTenantMoveDeletes = []func(t store.TableNames) store.TableName{
	func(t store.TableNames) store.TableName { return t.RefreshTokens() },
	func(t store.TableNames) store.TableName { return t.APITokens() },
	func(t store.TableNames) store.TableName { return t.EmailVerifications() },
	func(t store.TableNames) store.TableName { return t.PasswordResets() },
	func(t store.TableNames) store.TableName { return t.MagicLinkTokens() },
	func(t store.TableNames) store.TableName { return t.OperationSlots() },
	func(t store.TableNames) store.TableName { return t.UserConcurrencySlots() },
	func(t store.TableNames) store.TableName { return t.ThumbnailGenerationJobs() },
	func(t store.TableNames) store.TableName { return t.GroupNotificationPrefs() },
	func(t store.TableNames) store.TableName { return t.SavedViewPlacements() },
	func(t store.TableNames) store.TableName { return t.PersonalDataExports() },
}

// userTenantMoveRetenants are the per-user rows that belong to the person and
// follow them to the new tenant: their settings (language, theme, ...), their
// second factor, their linked sign-in identities, their login history, the
// audit of their AI scans and the record of the weekly digests already sent.
var userTenantMoveRetenants = []func(t store.TableNames) store.TableName{
	func(t store.TableNames) store.TableName { return t.Settings() },
	func(t store.TableNames) store.TableName { return t.UserMFASecrets() },
	func(t store.TableNames) store.TableName { return t.UserOAuthIdentities() },
	func(t store.TableNames) store.TableName { return t.LoginEvents() },
	func(t store.TableNames) store.TableName { return t.CommodityScanAudits() },
	func(t store.TableNames) store.TableName { return t.WeeklyDigestSends() },
}

// CountOwnedContent implements registry.UserTenantMover.
func (m *UserTenantMover) CountOwnedContent(ctx context.Context, userID string) (registry.UserOwnedContent, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	var owned registry.UserOwnedContent
	err := store.DoAsBackgroundWorker(ctx, m.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		var err error
		owned, err = countUserOwnedContent(ctx, tx, m.tableNames, userID)
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count the user's content", err)
	}
	return owned, nil
}

// MoveUserToTenant implements registry.UserTenantMover.
func (m *UserTenantMover) MoveUserToTenant(ctx context.Context, move registry.UserTenantMove) (*models.User, error) {
	if err := validateUserTenantMove(move); err != nil {
		return nil, err
	}

	var moved *models.User
	err := store.DoAsBackgroundWorker(ctx, m.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		email, err := m.lockUser(ctx, tx, move)
		if err != nil {
			return err
		}
		if err := m.checkTarget(ctx, tx, move, email); err != nil {
			return err
		}
		owned, err := countUserOwnedContent(ctx, tx, m.tableNames, move.UserID)
		if err != nil {
			return err
		}
		if !owned.IsEmpty() {
			return &registry.UserTenantMoveBlockedError{Owned: owned}
		}
		if err := m.moveUserRows(ctx, tx, move); err != nil {
			return err
		}
		moved, err = m.readUser(ctx, tx, move.UserID)
		return err
	})
	if blocked, ok := errors.AsType[*registry.UserTenantMoveBlockedError](err); ok {
		return nil, blocked
	}
	if err != nil {
		return nil, errxtrace.Wrap("failed to move the user to another tenant", err)
	}
	return moved, nil
}

func validateUserTenantMove(move registry.UserTenantMove) error {
	switch {
	case move.UserID == "":
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	case move.FromTenantID == "":
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "FromTenantID"))
	case move.ToTenantID == "":
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ToTenantID"))
	case move.FromTenantID == move.ToTenantID:
		return errxtrace.Classify(registry.ErrInvalidInput, errx.Attrs("reason", "the user is already in the target tenant"))
	}
	return nil
}

// lockUser locks the users row and checks it is still in the source tenant
// with the photo the caller copied. It returns the user's email for the
// collision check.
func (m *UserTenantMover) lockUser(ctx context.Context, tx *sqlx.Tx, move registry.UserTenantMove) (string, error) {
	var tenantID, email string
	var avatarPath *string
	query := fmt.Sprintf("SELECT tenant_id, email, avatar_path FROM %s WHERE id = $1 FOR UPDATE", m.tableNames.Users())
	err := tx.QueryRowContext(ctx, query, move.UserID).Scan(&tenantID, &email, &avatarPath)
	if errors.Is(err, sql.ErrNoRows) {
		return "", errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "User", "entity_id", move.UserID))
	}
	if err != nil {
		return "", errxtrace.Wrap("failed to lock the user", err)
	}
	if tenantID != move.FromTenantID {
		return "", errxtrace.Classify(registry.ErrUserTenantChanged, errx.Attrs(
			"user_id", move.UserID, "tenant_id", tenantID, "expected_tenant_id", move.FromTenantID))
	}
	if !move.PhotoUnchanged(avatarPath) {
		return "", errxtrace.Classify(registry.ErrUserChangedDuringMove, errx.Attrs("user_id", move.UserID))
	}
	return email, nil
}

// checkTarget checks that the target tenant exists, is active and has no other
// user with the same email.
func (m *UserTenantMover) checkTarget(ctx context.Context, tx *sqlx.Tx, move registry.UserTenantMove, email string) error {
	var status models.TenantStatus
	query := fmt.Sprintf("SELECT status FROM %s WHERE id = $1", m.tableNames.Tenants())
	err := tx.QueryRowContext(ctx, query, move.ToTenantID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "Tenant", "entity_id", move.ToTenantID))
	}
	if err != nil {
		return errxtrace.Wrap("failed to check the target tenant", err)
	}
	if status != models.TenantStatusActive {
		return errxtrace.Classify(registry.ErrTenantNotActive, errx.Attrs("tenant_id", move.ToTenantID, "status", status))
	}

	var emailTaken bool
	query = fmt.Sprintf("SELECT EXISTS (SELECT 1 FROM %s WHERE tenant_id = $1 AND email = $2 AND id <> $3)", m.tableNames.Users())
	if err := tx.QueryRowContext(ctx, query, move.ToTenantID, email, move.UserID).Scan(&emailTaken); err != nil {
		return errxtrace.Wrap("failed to check the email in the target tenant", err)
	}
	if emailTaken {
		return errxtrace.Classify(registry.ErrEmailAlreadyExists, errx.Attrs("email", email, "tenant_id", move.ToTenantID))
	}
	return nil
}

// moveUserRows deletes and re-tenants the per-user rows, then the users row.
func (m *UserTenantMover) moveUserRows(ctx context.Context, tx *sqlx.Tx, move registry.UserTenantMove) error {
	for _, tableFn := range userTenantMoveDeletes {
		table := tableFn(m.tableNames)
		query := fmt.Sprintf("DELETE FROM %s WHERE user_id = $1", table)
		if _, err := tx.ExecContext(ctx, query, move.UserID); err != nil {
			return errxtrace.Wrap("failed to delete the user's rows", err, errx.Attrs("table", table))
		}
	}
	for _, tableFn := range userTenantMoveRetenants {
		table := tableFn(m.tableNames)
		query := fmt.Sprintf("UPDATE %s SET tenant_id = $1 WHERE user_id = $2", table)
		if _, err := tx.ExecContext(ctx, query, move.ToTenantID, move.UserID); err != nil {
			return errxtrace.Wrap("failed to move the user's rows", err, errx.Attrs("table", table))
		}
	}

	query := fmt.Sprintf(
		"UPDATE %s SET tenant_id = $1, default_group_id = NULL, avatar_path = $2, updated_at = $3 WHERE id = $4",
		m.tableNames.Users())
	if _, err := tx.ExecContext(ctx, query, move.ToTenantID, move.AvatarPath, time.Now().UTC(), move.UserID); err != nil {
		// The email check above runs under the users row lock of this user
		// only; another user with the same email can still be created in the
		// target tenant before this commits. The unique index catches it.
		if isUniqueViolationOn(err, usersTenantEmailIndex) {
			return errxtrace.Classify(registry.ErrEmailAlreadyExists, errx.Attrs("tenant_id", move.ToTenantID))
		}
		return errxtrace.Wrap("failed to move the user", err)
	}
	return nil
}

// usersTenantEmailIndex is the unique index on users(tenant_id, email).
const usersTenantEmailIndex = "users_tenant_email_idx"

func (m *UserTenantMover) readUser(ctx context.Context, tx *sqlx.Tx, userID string) (*models.User, error) {
	var user models.User
	err := store.NewTxRegistry[models.User](tx, m.tableNames.Users()).ScanOneByField(ctx, store.Pair("id", userID), &user)
	if err != nil {
		return nil, errxtrace.Wrap("failed to read the moved user", err)
	}
	return &user, nil
}
