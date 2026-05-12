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

var _ registry.GroupMembershipRegistry = (*GroupMembershipRegistry)(nil)

type GroupMembershipRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewGroupMembershipRegistry(dbx *sqlx.DB) *GroupMembershipRegistry {
	return &GroupMembershipRegistry{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
	}
}

// newSQLRegistry returns an RLSRepository in service mode. group_memberships
// has RLS enabled with a tenant-isolation policy on inventario_app; service
// mode runs as the background-worker role (bypass policy). Tenant scoping is
// performed in application code via the ListByUser(tenantID, …) contract.
func (r *GroupMembershipRegistry) newSQLRegistry() *store.RLSRepository[models.GroupMembership, *models.GroupMembership] {
	return store.NewServiceSQLRegistry[models.GroupMembership, *models.GroupMembership](r.dbx, r.tableNames.GroupMemberships())
}

func (r *GroupMembershipRegistry) Get(ctx context.Context, id string) (*models.GroupMembership, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var membership models.GroupMembership
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &membership)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "GroupMembership",
				"entity_id", id,
			))
		}
		return nil, errxtrace.Wrap("failed to get group membership", err)
	}

	return &membership, nil
}

func (r *GroupMembershipRegistry) List(ctx context.Context) ([]*models.GroupMembership, error) {
	var memberships []*models.GroupMembership

	reg := r.newSQLRegistry()

	for membership, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list group memberships", err)
		}
		memberships = append(memberships, &membership)
	}

	return memberships, nil
}

func (r *GroupMembershipRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count group memberships", err)
	}

	return count, nil
}

func (r *GroupMembershipRegistry) Create(ctx context.Context, membership models.GroupMembership) (*models.GroupMembership, error) {
	if membership.GroupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	if membership.MemberUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "MemberUserID"))
	}

	if membership.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	// Role is a NOT NULL column and the zero value ("") would silently land
	// in the DB. Validate it matches one of the defined roles here so the
	// caller gets a clear field error rather than a downstream CHECK / enum
	// violation.
	if err := membership.Role.Validate(); err != nil {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Role"))
	}

	reg := r.newSQLRegistry()

	createdMembership, err := reg.Create(ctx, membership, func(ctx context.Context, tx *sqlx.Tx) error {
		// Check for duplicate membership
		txReg := store.NewTxRegistry[models.GroupMembership](tx, r.tableNames.GroupMemberships())
		for m, scanErr := range txReg.ScanByField(ctx, store.Pair("group_id", membership.GroupID)) {
			if scanErr != nil {
				return errxtrace.Wrap("failed to check for existing membership", scanErr)
			}
			if m.MemberUserID == membership.MemberUserID {
				return errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs(
					"group_id", membership.GroupID,
					"member_user_id", membership.MemberUserID,
				))
			}
		}
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to create group membership", err)
	}

	return &createdMembership, nil
}

func (r *GroupMembershipRegistry) Update(ctx context.Context, membership models.GroupMembership) (*models.GroupMembership, error) {
	if membership.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, membership, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update group membership", err)
	}

	return &membership, nil
}

func (r *GroupMembershipRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errxtrace.Wrap("failed to delete group membership", err)
	}

	return nil
}

func (r *GroupMembershipRegistry) GetByGroupAndUser(ctx context.Context, groupID, userID string) (*models.GroupMembership, error) {
	if groupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	reg := r.newSQLRegistry()

	for membership, err := range reg.ScanByField(ctx, store.Pair("group_id", groupID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to get membership by group and user", err)
		}
		if membership.MemberUserID == userID {
			return &membership, nil
		}
	}

	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
		"entity_type", "GroupMembership",
		"group_id", groupID,
		"user_id", userID,
	))
}

func (r *GroupMembershipRegistry) ListByGroup(ctx context.Context, groupID string) ([]*models.GroupMembership, error) {
	if groupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	var memberships []*models.GroupMembership
	reg := r.newSQLRegistry()

	for membership, err := range reg.ScanByField(ctx, store.Pair("group_id", groupID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list memberships by group", err)
		}
		memberships = append(memberships, &membership)
	}

	return memberships, nil
}

func (r *GroupMembershipRegistry) ListByUser(ctx context.Context, tenantID, userID string) ([]*models.GroupMembership, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	var memberships []*models.GroupMembership
	reg := r.newSQLRegistry()

	for membership, err := range reg.ScanByField(ctx, store.Pair("member_user_id", userID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list memberships by user", err)
		}
		if membership.TenantID == tenantID {
			memberships = append(memberships, &membership)
		}
	}

	return memberships, nil
}

// CountByUser returns the per-user membership count inside a tenant
// via SELECT COUNT(*) — used by the cap check on the hot
// CreateGroup / AddMember / AcceptInvite path so we don't materialize
// every row only to read its length.
func (r *GroupMembershipRegistry) CountByUser(ctx context.Context, tenantID, userID string) (int, error) {
	if tenantID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	var count int
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE tenant_id = $1 AND member_user_id = $2`, r.tableNames.GroupMemberships())
		return tx.QueryRowContext(ctx, query, tenantID, userID).Scan(&count)
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to count memberships by user", err)
	}
	return count, nil
}

// CreateUnderCap atomically counts the user's existing memberships
// and inserts a new one only if the count is below maxMemberships.
// The transaction takes a (tenant, user) advisory lock so two
// concurrent callers can't both pass a stale count check and exceed
// the cap. Returns (nil, true, nil) when the cap would be exceeded;
// surface code translates that into ErrTooManyGroupMemberships.
func (r *GroupMembershipRegistry) CreateUnderCap(ctx context.Context, membership models.GroupMembership, maxMemberships int) (*models.GroupMembership, bool, error) {
	if maxMemberships <= 0 {
		return nil, false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "maxMemberships"))
	}
	if membership.GroupID == "" {
		return nil, false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	if membership.MemberUserID == "" {
		return nil, false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "MemberUserID"))
	}
	if membership.TenantID == "" {
		return nil, false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if err := membership.Role.Validate(); err != nil {
		return nil, false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Role"))
	}

	var (
		overCap   bool
		createdID string
	)
	reg := r.newSQLRegistry()
	created, err := reg.Create(ctx, membership, func(ctx context.Context, tx *sqlx.Tx) error {
		// Per-(tenant, user) advisory lock serializes concurrent cap
		// checks against the same user without blocking unrelated
		// memberships. xact-scoped, released on COMMIT/ROLLBACK.
		if _, lockErr := tx.ExecContext(ctx,
			`SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`,
			membership.TenantID, membership.MemberUserID,
		); lockErr != nil {
			return errxtrace.Wrap("failed to acquire membership cap lock", lockErr)
		}

		var count int
		query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE tenant_id = $1 AND member_user_id = $2`, r.tableNames.GroupMemberships())
		if scanErr := tx.QueryRowContext(ctx, query, membership.TenantID, membership.MemberUserID).Scan(&count); scanErr != nil {
			return errxtrace.Wrap("failed to count memberships under lock", scanErr)
		}
		if count >= maxMemberships {
			overCap = true
			// Bail out of the tx so the Create wrapper rolls back the
			// would-be insert. The sentinel propagates up to the
			// service layer via the err return.
			return errMembershipCapReached
		}

		// Re-check duplicate membership while we hold the lock — same
		// invariant as the regular Create path, just inside the
		// already-open tx so we don't open a second one.
		txReg := store.NewTxRegistry[models.GroupMembership](tx, r.tableNames.GroupMemberships())
		for m, scanErr := range txReg.ScanByField(ctx, store.Pair("group_id", membership.GroupID)) {
			if scanErr != nil {
				return errxtrace.Wrap("failed to check for existing membership", scanErr)
			}
			if m.MemberUserID == membership.MemberUserID {
				return errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs(
					"group_id", membership.GroupID,
					"member_user_id", membership.MemberUserID,
				))
			}
		}
		return nil
	})
	if overCap {
		return nil, true, nil
	}
	if err != nil {
		return nil, false, errxtrace.Wrap("failed to create membership under cap", err)
	}
	createdID = created.ID
	_ = createdID
	return &created, false, nil
}

// errMembershipCapReached is the sentinel CreateUnderCap raises inside
// the tx callback when the cap would be exceeded. The outer code never
// surfaces this — it's caught and translated into the (nil, true, nil)
// return contract.
var errMembershipCapReached = errors.New("membership cap reached")

// CountAdminsByGroup counts memberships with role >= admin. After the
// #1533 role-taxonomy expansion that includes both admin and owner —
// the call site (last-admin guard) was always asking "is anyone left
// who can act as an admin?", and owners by definition can. Use
// CountOwnersByGroup for the stricter ≥1-owner invariant.
func (r *GroupMembershipRegistry) CountAdminsByGroup(ctx context.Context, groupID string) (int, error) {
	if groupID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	count := 0
	reg := r.newSQLRegistry()

	for membership, err := range reg.ScanByField(ctx, store.Pair("group_id", groupID)) {
		if err != nil {
			return 0, errxtrace.Wrap("failed to count admins by group", err)
		}
		if membership.IsAdmin() {
			count++
		}
	}

	return count, nil
}

// CountOwnersByGroup counts memberships with role = 'owner'. The
// last-owner guard uses this to enforce that every group keeps at
// least one user who can delete it.
func (r *GroupMembershipRegistry) CountOwnersByGroup(ctx context.Context, groupID string) (int, error) {
	if groupID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	count := 0
	reg := r.newSQLRegistry()

	for membership, err := range reg.ScanByField(ctx, store.Pair("group_id", groupID)) {
		if err != nil {
			return 0, errxtrace.Wrap("failed to count owners by group", err)
		}
		if membership.Role == models.GroupRoleOwner {
			count++
		}
	}

	return count, nil
}

// ListByGroupWithUsers joins group_memberships with users so the
// members list endpoint can serve avatar/name/email in a single
// round-trip. Tenant-scoped via the membership row's tenant_id
// (the RLS layer enforces this on top); the user join doesn't add
// a tenant predicate of its own — a membership references a user
// that already lives in the same tenant.
func (r *GroupMembershipRegistry) ListByGroupWithUsers(ctx context.Context, groupID string) ([]*models.MembershipWithUser, error) {
	if groupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	type row struct {
		// membership fields
		MID           string    `db:"m_id"`
		MUUID         string    `db:"m_uuid"`
		MTenantID     string    `db:"m_tenant_id"`
		MGroupID      string    `db:"m_group_id"`
		MMemberUserID string    `db:"m_member_user_id"`
		MRole         string    `db:"m_role"`
		MJoinedAt     time.Time `db:"m_joined_at"`
		// user fields
		UID        string    `db:"u_id"`
		UUUID      string    `db:"u_uuid"`
		UEmail     string    `db:"u_email"`
		UName      string    `db:"u_name"`
		UIsActive  bool      `db:"u_is_active"`
		UCreatedAt time.Time `db:"u_created_at"`
	}

	query := fmt.Sprintf(`
		SELECT
			m.id AS m_id, m.uuid AS m_uuid, m.tenant_id AS m_tenant_id,
			m.group_id AS m_group_id, m.member_user_id AS m_member_user_id,
			m.role AS m_role, m.joined_at AS m_joined_at,
			u.id AS u_id, u.uuid AS u_uuid, u.email AS u_email,
			u.name AS u_name, u.is_active AS u_is_active,
			u.created_at AS u_created_at
		FROM %s m
		JOIN %s u ON u.id = m.member_user_id
		WHERE m.group_id = $1
		ORDER BY m.joined_at ASC
	`, r.tableNames.GroupMemberships(), r.tableNames.Users())

	var out []*models.MembershipWithUser
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		rows, err := tx.QueryxContext(ctx, query, groupID)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var r row
			if err := rows.StructScan(&r); err != nil {
				return err
			}
			m := &models.GroupMembership{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: r.MID},
					TenantID: r.MTenantID,
				},
				GroupID:      r.MGroupID,
				MemberUserID: r.MMemberUserID,
				Role:         models.GroupRole(r.MRole),
				JoinedAt:     r.MJoinedAt,
			}
			m.UUID = r.MUUID
			u := &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: r.UID},
				},
				Email:     r.UEmail,
				Name:      r.UName,
				IsActive:  r.UIsActive,
				CreatedAt: r.UCreatedAt,
			}
			u.UUID = r.UUUID
			out = append(out, &models.MembershipWithUser{
				Membership: m,
				User:       u,
			})
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list memberships with users", err)
	}
	return out, nil
}
