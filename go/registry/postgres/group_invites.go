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

func (r *GroupInviteRegistry) newSQLRegistry() *store.NonRLSRepository[models.GroupInvite, *models.GroupInvite] {
	return store.NewSQLRegistry[models.GroupInvite, *models.GroupInvite](r.dbx, r.tableNames.GroupInvites())
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

	tableName := r.tableNames.GroupInvites()
	query := fmt.Sprintf("UPDATE %s SET used_by = $1, used_at = $2 WHERE id = $3 AND used_by IS NULL", tableName)
	result, err := r.dbx.ExecContext(ctx, query, userID, usedAt, inviteID)
	if err != nil {
		return false, errxtrace.Wrap("failed to mark invite as used", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, errxtrace.Wrap("failed to read rows affected for MarkUsed", err)
	}
	return rows == 1, nil
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
