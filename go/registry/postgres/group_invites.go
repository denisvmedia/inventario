package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.GroupInviteRegistry = (*GroupInviteRegistry)(nil)

type GroupInviteRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewGroupInviteRegistry(dbx *sqlx.DB) *GroupInviteRegistry {
	return &GroupInviteRegistry{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
	}
}

// newSQLRegistry returns an RLSRepository in service mode. group_invites has
// RLS enabled with a tenant-isolation policy on inventario_app, but invite
// lookup by token happens before any user/tenant context exists in the
// session (the invitee may not even be authenticated yet). Service mode runs
// as the background-worker role (bypass policy) so token lookup works;
// tenant verification is enforced in AcceptInvite (see expectedTenantID).
// Same pattern as RefreshTokenRegistry.
func (r *GroupInviteRegistry) newSQLRegistry() *store.RLSRepository[models.GroupInvite, *models.GroupInvite] {
	return store.NewServiceSQLRegistry[models.GroupInvite, *models.GroupInvite](r.dbx, r.tableNames.GroupInvites())
}

func (r *GroupInviteRegistry) Get(ctx context.Context, id string) (*models.GroupInvite, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var invite models.GroupInvite
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &invite)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "GroupInvite",
				"entity_id", id,
			))
		}
		return nil, errxtrace.Wrap("failed to get group invite", err)
	}

	return &invite, nil
}

func (r *GroupInviteRegistry) List(ctx context.Context) ([]*models.GroupInvite, error) {
	var invites []*models.GroupInvite

	reg := r.newSQLRegistry()

	for invite, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list group invites", err)
		}
		invites = append(invites, &invite)
	}

	return invites, nil
}

func (r *GroupInviteRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count group invites", err)
	}

	return count, nil
}

func (r *GroupInviteRegistry) Create(ctx context.Context, invite models.GroupInvite) (*models.GroupInvite, error) {
	if invite.GroupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	if invite.Token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}

	if invite.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	// CreatedBy and ExpiresAt are NOT NULL columns. Without these guards
	// a caller that forgets either one would persist an invite with an
	// empty creator (FK violation downstream) or a zero ExpiresAt, which
	// reads as "already expired" and would make the invite invalid the
	// moment it's created.
	if invite.CreatedBy == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "CreatedBy"))
	}

	if invite.ExpiresAt.IsZero() {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ExpiresAt"))
	}

	reg := r.newSQLRegistry()

	createdInvite, err := reg.Create(ctx, invite, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create group invite", err)
	}

	return &createdInvite, nil
}

func (r *GroupInviteRegistry) Update(ctx context.Context, invite models.GroupInvite) (*models.GroupInvite, error) {
	if invite.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, invite, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update group invite", err)
	}

	return &invite, nil
}

func (r *GroupInviteRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errxtrace.Wrap("failed to delete group invite", err)
	}

	return nil
}

func (r *GroupInviteRegistry) GetByToken(ctx context.Context, token string) (*models.GroupInvite, error) {
	if token == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Token"))
	}

	var invite models.GroupInvite
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("token", token), &invite)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "GroupInvite",
				"token", "[redacted]",
			))
		}
		return nil, errxtrace.Wrap("failed to get group invite by token", err)
	}

	return &invite, nil
}

// MarkUsed atomically flips an invite row from unused to used-by-userID via
// a conditional UPDATE. The `used_by IS NULL` clause is the compare-and-swap
// predicate — at most one concurrent caller per invite mutates the row.
// Returns (true, nil) if the row was updated by this call, (false, nil) if
// the invite was already used, and (false, err) for any other failure.
func (r *GroupInviteRegistry) MarkUsed(ctx context.Context, inviteID, userID string, usedAt time.Time) (bool, error) {
	if inviteID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if userID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	// Run the CAS UPDATE inside the service-mode tx so it executes under the
	// background-worker role (which has the bypass RLS policy on
	// group_invites). Running the raw ExecContext on r.dbx would use the
	// default session role and could be blocked by the tenant-isolation
	// policy when no tenant context has been SET LOCAL on the session.
	tableName := r.tableNames.GroupInvites()
	query := fmt.Sprintf("UPDATE %s SET used_by = $1, used_at = $2 WHERE id = $3 AND used_by IS NULL", tableName)
	var rowsAffected int64
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		res, execErr := tx.ExecContext(ctx, query, userID, usedAt, inviteID)
		if execErr != nil {
			return errxtrace.Wrap("failed to mark invite as used", execErr)
		}
		n, raErr := res.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read rows affected for MarkUsed", raErr)
		}
		rowsAffected = n
		return nil
	})
	if err != nil {
		return false, err
	}
	return rowsAffected == 1, nil
}

func (r *GroupInviteRegistry) ListActiveByGroup(ctx context.Context, groupID string) ([]*models.GroupInvite, error) {
	if groupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	var invites []*models.GroupInvite
	reg := r.newSQLRegistry()
	now := time.Now()

	for invite, err := range reg.ScanByField(ctx, store.Pair("group_id", groupID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list active invites by group", err)
		}
		// Only include non-expired, unused invites
		if invite.UsedBy == nil && invite.ExpiresAt.After(now) {
			invites = append(invites, &invite)
		}
	}

	return invites, nil
}
