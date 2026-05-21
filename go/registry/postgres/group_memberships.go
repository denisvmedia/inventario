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

// CountAdminsByGroup counts memberships with role >= admin (admin or
// owner) for the given group. After the #1533 role-taxonomy expansion
// the call site (last-admin guard) was always asking "is anyone left
// who can act as an admin?", and owners by definition can. Use
// CountOwnersByGroup for the stricter ≥1-owner invariant.
//
// The query runs as a single `SELECT COUNT(*)` so the DB does the
// counting; large groups no longer pay an O(N) row-scan on every
// last-admin / last-owner check.
func (r *GroupMembershipRegistry) CountAdminsByGroup(ctx context.Context, groupID string) (int, error) {
	if groupID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	var count int
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s WHERE group_id = $1 AND role IN ($2, $3)`,
			r.tableNames.GroupMemberships(),
		)
		return tx.QueryRowContext(ctx, query, groupID, string(models.GroupRoleAdmin), string(models.GroupRoleOwner)).Scan(&count)
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to count admins by group", err)
	}
	return count, nil
}

// CountOwnersByGroup counts memberships with role = 'owner'. The
// last-owner guard uses this to enforce that every group keeps at
// least one user who can delete it. Same SQL-level COUNT(*) as
// CountAdminsByGroup — no Go-side row scan.
func (r *GroupMembershipRegistry) CountOwnersByGroup(ctx context.Context, groupID string) (int, error) {
	if groupID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	var count int
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s WHERE group_id = $1 AND role = $2`,
			r.tableNames.GroupMemberships(),
		)
		return tx.QueryRowContext(ctx, query, groupID, string(models.GroupRoleOwner)).Scan(&count)
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to count owners by group", err)
	}
	return count, nil
}

// CountByGroup returns the total number of memberships in a group.
// Used to surface members_count on the LocationGroup resource (#1650)
// when only one group needs counting. The query is a plain SQL-level
// COUNT(*) so the DB does the aggregation.
func (r *GroupMembershipRegistry) CountByGroup(ctx context.Context, groupID string) (int, error) {
	if groupID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	var count int
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT COUNT(*) FROM %s WHERE group_id = $1`,
			r.tableNames.GroupMemberships(),
		)
		return tx.QueryRowContext(ctx, query, groupID).Scan(&count)
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to count memberships by group", err)
	}
	return count, nil
}

// CountByGroups batches the per-group membership count for the
// /groups list handler so it pays one extra round-trip instead of N.
// The result map is pre-seeded with zeros for every requested ID so
// groups with zero memberships are still represented (a defensive
// invariant — every active group has its creator/owner row, but the
// API contract should not assume that).
func (r *GroupMembershipRegistry) CountByGroups(ctx context.Context, groupIDs []string) (map[string]int, error) {
	out := make(map[string]int, len(groupIDs))
	for _, id := range groupIDs {
		out[id] = 0
	}
	if len(groupIDs) == 0 {
		return out, nil
	}

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT group_id, COUNT(*)::int
			 FROM %s
			 WHERE group_id = ANY($1)
			 GROUP BY group_id`,
			r.tableNames.GroupMemberships(),
		)
		rows, err := tx.QueryxContext(ctx, query, groupIDs)
		if err != nil {
			return errxtrace.Wrap("failed to query group membership counts", err)
		}
		defer rows.Close()
		for rows.Next() {
			var (
				groupID string
				cnt     int
			)
			if err := rows.Scan(&groupID, &cnt); err != nil {
				return errxtrace.Wrap("failed to scan group membership count", err)
			}
			out[groupID] = cnt
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count memberships by groups", err)
	}
	return out, nil
}

// lockGroupAndReadMembershipRole resolves the membership's group_id,
// acquires the shared per-group leave/role advisory lock, then re-reads
// the membership's role under that lock. The pre-lock group_id read is
// safe because group_id never mutates on an existing row; the role
// re-read is the one that matters: a concurrent owner-demotion could
// have flipped the role while this tx waited for the lock, and acting
// on the stale pre-lock view would let the owner invariants slip.
// Used by both DeleteWithMemberInvariants and UpdateRoleWithMemberInvariants
// so the two paths share both the lock key AND the freshly-read role
// they branch on (#1652, Copilot review on PR #1666).
func (r *GroupMembershipRegistry) lockGroupAndReadMembershipRole(ctx context.Context, tx *sqlx.Tx, membershipID string) (groupID, role string, err error) {
	lookup := fmt.Sprintf(`SELECT group_id FROM %s WHERE id = $1`, r.tableNames.GroupMemberships())
	if scanErr := tx.QueryRowContext(ctx, lookup, membershipID).Scan(&groupID); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			return "", "", errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "GroupMembership",
				"entity_id", membershipID,
			))
		}
		return "", "", errxtrace.Wrap("failed to look up membership group_id", scanErr)
	}

	if _, lockErr := tx.ExecContext(ctx,
		`SELECT pg_advisory_xact_lock(hashtext('group_membership_leave'), hashtext($1))`,
		groupID,
	); lockErr != nil {
		return "", "", errxtrace.Wrap("failed to acquire per-group leave lock", lockErr)
	}

	roleQuery := fmt.Sprintf(`SELECT role FROM %s WHERE id = $1`, r.tableNames.GroupMemberships())
	if scanErr := tx.QueryRowContext(ctx, roleQuery, membershipID).Scan(&role); scanErr != nil {
		if errors.Is(scanErr, sql.ErrNoRows) {
			// The row vanished while we waited for the lock —
			// a concurrent leave already won the race.
			return "", "", errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "GroupMembership",
				"entity_id", membershipID,
			))
		}
		return "", "", errxtrace.Wrap("failed to re-read membership role under leave lock", scanErr)
	}
	return groupID, role, nil
}

// DeleteWithMemberInvariants atomically removes a membership while
// two transactional invariants are enforced under a per-group
// advisory lock (#1652). The lock serializes concurrent leaves
// against the same group so two members on a two-row group cannot
// both pass the count check and both delete (the count(*) under the
// advisory lock acts as the FOR UPDATE the AC asks for, without
// pulling every membership row into memory only to lock them). If
// the row's role is owner and removing it would drop the owner count
// to zero, ErrLastOwner is returned without touching the row; if
// removing it would drop the total membership count to zero,
// ErrLastMember is returned (defense-in-depth — catches the case
// where role data has drifted so the owner check passes vacuously).
// Returns ErrNotFound if no row with the given id exists.
func (r *GroupMembershipRegistry) DeleteWithMemberInvariants(ctx context.Context, membershipID string) error {
	if membershipID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		groupID, currentRole, lockErr := r.lockGroupAndReadMembershipRole(ctx, tx, membershipID)
		if lockErr != nil {
			return lockErr
		}

		// Invariant A — ≥1 owner. Checked first so a sole-owner self-
		// leave (which is also a sole-member leave) surfaces the more
		// specific ErrLastOwner — the FE renders a "transfer
		// ownership first" path that's directly actionable. The
		// member-count fallback only fires when the owner check
		// passes vacuously (role data drift; defense-in-depth).
		if currentRole == string(models.GroupRoleOwner) {
			var ownerCount int
			countOwners := fmt.Sprintf(
				`SELECT COUNT(*) FROM %s WHERE group_id = $1 AND role = $2`,
				r.tableNames.GroupMemberships(),
			)
			if scanErr := tx.QueryRowContext(ctx, countOwners, groupID, string(models.GroupRoleOwner)).Scan(&ownerCount); scanErr != nil {
				return errxtrace.Wrap("failed to count owners under leave lock", scanErr)
			}
			if ownerCount <= 1 {
				return errxtrace.Classify(registry.ErrLastOwner, errx.Attrs("group_id", groupID))
			}
		}

		// Invariant B — ≥1 member. Count under the lock so a
		// concurrent leave can't drop the total from 2 → 0.
		var memberCount int
		countMembers := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE group_id = $1`, r.tableNames.GroupMemberships())
		if scanErr := tx.QueryRowContext(ctx, countMembers, groupID).Scan(&memberCount); scanErr != nil {
			return errxtrace.Wrap("failed to count group memberships under leave lock", scanErr)
		}
		if memberCount <= 1 {
			return errxtrace.Classify(registry.ErrLastMember, errx.Attrs("group_id", groupID))
		}

		del := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, r.tableNames.GroupMemberships())
		res, delErr := tx.ExecContext(ctx, del, membershipID)
		if delErr != nil {
			return errxtrace.Wrap("failed to delete membership under leave lock", delErr)
		}
		affected, raErr := res.RowsAffected()
		if raErr != nil {
			return errxtrace.Wrap("failed to read rows-affected", raErr)
		}
		if affected == 0 {
			// Raced with another leave / delete; the row is gone.
			return errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "GroupMembership",
				"entity_id", membershipID,
			))
		}
		return nil
	})
	if err != nil {
		// Pass-through the classified sentinels so callers can
		// errors.Is them; wrap only genuinely-unexpected paths.
		if errors.Is(err, registry.ErrLastOwner) ||
			errors.Is(err, registry.ErrLastMember) ||
			errors.Is(err, registry.ErrNotFound) ||
			errors.Is(err, registry.ErrFieldRequired) {
			return err
		}
		return errxtrace.Wrap("failed to delete membership with invariants", err)
	}
	return nil
}

// UpdateRoleWithMemberInvariants atomically swaps the row's role
// under the SAME per-group advisory lock key DeleteWithMemberInvariants
// uses (#1652). Without sharing the key, a concurrent leave +
// owner-demotion pair could both observe ownerCount=2 before either
// committed and both commit — leaving the group with zero owners.
// With it the second op (whichever wins the lock second) sees the
// first op's effect and bails out with ErrLastOwner.
func (r *GroupMembershipRegistry) UpdateRoleWithMemberInvariants(ctx context.Context, membershipID string, newRole models.GroupRole) (*models.GroupMembership, error) {
	if membershipID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}
	if err := newRole.Validate(); err != nil {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "Role"))
	}

	var updated models.GroupMembership
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		groupID, currentRole, lockErr := r.lockGroupAndReadMembershipRole(ctx, tx, membershipID)
		if lockErr != nil {
			return lockErr
		}

		// Owner-count check only when transitioning out of owner.
		// `currentRole` is the post-lock value so a concurrent
		// demotion that already landed isn't double-counted.
		if currentRole == string(models.GroupRoleOwner) && newRole != models.GroupRoleOwner {
			var ownerCount int
			countOwners := fmt.Sprintf(
				`SELECT COUNT(*) FROM %s WHERE group_id = $1 AND role = $2`,
				r.tableNames.GroupMemberships(),
			)
			if scanErr := tx.QueryRowContext(ctx, countOwners, groupID, string(models.GroupRoleOwner)).Scan(&ownerCount); scanErr != nil {
				return errxtrace.Wrap("failed to count owners under leave lock", scanErr)
			}
			if ownerCount <= 1 {
				return errxtrace.Classify(registry.ErrLastOwner, errx.Attrs("group_id", groupID))
			}
		}

		upd := fmt.Sprintf(`UPDATE %s SET role = $1 WHERE id = $2 RETURNING *`, r.tableNames.GroupMemberships())
		if scanErr := tx.QueryRowxContext(ctx, upd, string(newRole), membershipID).StructScan(&updated); scanErr != nil {
			if errors.Is(scanErr, sql.ErrNoRows) {
				return errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
					"entity_type", "GroupMembership",
					"entity_id", membershipID,
				))
			}
			return errxtrace.Wrap("failed to update membership role under leave lock", scanErr)
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, registry.ErrLastOwner) ||
			errors.Is(err, registry.ErrNotFound) ||
			errors.Is(err, registry.ErrFieldRequired) {
			return nil, err
		}
		return nil, errxtrace.Wrap("failed to update membership role with invariants", err)
	}
	return &updated, nil
}

// ListByGroupWithUsers joins group_memberships with users so the
// members list endpoint can serve avatar/name/email in a single
// round-trip. The JOIN matches tenant_id on both sides as a
// defense-in-depth guard against cross-tenant leakage. Note the query
// rows are NOT RLS-scoped here: this registry runs in service mode on
// the background-worker role, which bypasses the tenant-isolation
// policy. The tenant gate for the non-admin /groups/{id}/members
// surface is the requireGroupMembership HTTP middleware upstream;
// pinning the user row's tenant_id to the membership's keeps the
// SQL-layer contract explicit on top of that.
func (r *GroupMembershipRegistry) ListByGroupWithUsers(ctx context.Context, groupID string) ([]*models.MembershipWithUser, error) {
	if groupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	var out []*models.MembershipWithUser
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var loadErr error
		out, loadErr = r.loadMembersWithUsersTx(ctx, tx, groupID)
		return loadErr
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list memberships with users", err)
	}
	return out, nil
}

// ListByGroupWithUsersAdmin is the cross-tenant twin of
// ListByGroupWithUsers, backing the #1756 admin membership editor. The
// system-admin caller is not tenant-scoped, so the join runs under
// `SET LOCAL row_security = off` — the same defense-in-depth RLS bypass
// LocationGroupRegistry.GetAdmin / ListAdmin use. The membership rows
// already run via the background-worker role (bypass policy on
// group_memberships); the explicit `row_security = off` additionally
// covers the JOINed users table so a group in ANY tenant lists fine.
func (r *GroupMembershipRegistry) ListByGroupWithUsersAdmin(ctx context.Context, groupID string) ([]*models.MembershipWithUser, error) {
	if groupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	var out []*models.MembershipWithUser
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if _, execErr := tx.ExecContext(ctx, "SET LOCAL row_security = off"); execErr != nil {
			return errxtrace.Wrap("failed to disable row_security for admin members listing", execErr)
		}
		var loadErr error
		out, loadErr = r.loadMembersWithUsersTx(ctx, tx, groupID)
		return loadErr
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list memberships with users", err)
	}
	return out, nil
}

// loadMembersWithUsersTx runs the group_memberships↔users join within
// the supplied tx. It is the shared query body for ListByGroupWithUsers
// and its admin twin so the two surfaces ship an identical row shape.
// The query rows are never RLS-scoped at this layer — the registry runs
// in service mode on the background-worker role, which bypasses the
// tenant-isolation policy. The only difference between the two callers
// is whether the admin twin additionally issues `SET LOCAL row_security
// = off` first as documented defense-in-depth covering the JOINed users
// table; the non-admin caller is tenant-gated upstream by the
// requireGroupMembership HTTP middleware. The JOIN matches tenant_id on
// both sides as a further defense-in-depth guard against cross-tenant
// leakage.
func (r *GroupMembershipRegistry) loadMembersWithUsersTx(ctx context.Context, tx *sqlx.Tx, groupID string) ([]*models.MembershipWithUser, error) {
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
		UTenantID  string    `db:"u_tenant_id"`
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
			u.id AS u_id, u.uuid AS u_uuid, u.tenant_id AS u_tenant_id,
			u.email AS u_email, u.name AS u_name, u.is_active AS u_is_active,
			u.created_at AS u_created_at
		FROM %s m
		JOIN %s u ON u.id = m.member_user_id AND u.tenant_id = m.tenant_id
		WHERE m.group_id = $1
		ORDER BY m.joined_at ASC
	`, r.tableNames.GroupMemberships(), r.tableNames.Users())

	rows, err := tx.QueryxContext(ctx, query, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*models.MembershipWithUser
	for rows.Next() {
		var r row
		if err := rows.StructScan(&r); err != nil {
			return nil, err
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
				TenantID: r.UTenantID,
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
	return out, rows.Err()
}
