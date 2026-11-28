package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

var _ registry.UserPurger = (*UserPurger)(nil)

// UserPurger clears what names a single user before the users row can be
// deleted, all in one background-worker transaction (#2116). A bare
// DELETE FROM users is rejected by the NO ACTION foreign keys that point at
// users(id), so every row that references the user is either deleted, handed
// to another member of its group, or left for the account-deletion pre-check
// to refuse:
//
//   - Rows keyed by user_id (sessions, tokens, MFA, OAuth identities,
//     settings, notification preferences, digest claims, saved view
//     placements, personal data exports and the like) are deleted, see
//     userDeleteByUserID.
//   - The userOwnedTables entries marked deletionRemoves (memberships,
//     invites, item reminders, drafts, intake sessions, saved views) are
//     deleted in the user's tenant.
//   - The entries marked deletionHandsOver (item history, loans, service
//     records, supply links, maintenance, reminder send records, item types,
//     custom fields, delivered documents, restores, currency changes) pass to
//     another member of their group.
//   - The entries marked deletionBlocks (items, files, places, exports, tags,
//     sellers, groups) are not touched. The account-deletion pre-check refuses
//     while the user has one in a group that stays, and the final users delete
//     fails on any that remain.
//   - group_invites_audit and the invites the user accepted are deleted in
//     every tenant, and system_admin_grants is cleared by user.
//
// It does NOT touch the users row itself — the orchestration layer
// (services.* user-delete flow) issues the final DELETE FROM users after this
// returns clean.
type UserPurger struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewUserPurger returns a UserPurger bound to the default table names.
func NewUserPurger(dbx *sqlx.DB) *UserPurger {
	return NewUserPurgerWithTableNames(dbx, store.DefaultTableNames)
}

// NewUserPurgerWithTableNames returns a UserPurger using a custom TableNames
// (used by tests that want to sandbox against renamed tables).
func NewUserPurgerWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *UserPurger {
	return &UserPurger{dbx: dbx, tableNames: tableNames}
}

// userDeleteByUserID is the FK-safe DELETE sequence for tables that carry a
// `user_id` column AND a `tenant_id` column. Each entry resolves to the
// fully-qualified table name; PurgeUserDependents issues
// `DELETE FROM <table> WHERE tenant_id = $1 AND user_id = $2` against each one
// inside a single background-worker transaction. tenant_id is added as
// defense-in-depth so the worker role can't wipe another tenant's rows on a
// bad userID.
//
// Order: deepest children first. None of these tables is a parent of another
// in this slice (they are all leaf auth/identity/per-user rows), so the order
// is cosmetic, but kept stable for readability.
var userDeleteByUserID = []func(t store.TableNames) string{
	// Auth / session.
	func(t store.TableNames) string { return string(t.RefreshTokens()) },
	func(t store.TableNames) string { return string(t.APITokens()) },
	func(t store.TableNames) string { return string(t.LoginEvents()) },
	func(t store.TableNames) string { return string(t.EmailVerifications()) },
	func(t store.TableNames) string { return string(t.PasswordResets()) },
	func(t store.TableNames) string { return string(t.MagicLinkTokens()) },
	func(t store.TableNames) string { return string(t.UserMFASecrets()) },
	func(t store.TableNames) string { return string(t.UserOAuthIdentities()) },

	// Per-user concurrency / operation bookkeeping.
	func(t store.TableNames) string { return string(t.OperationSlots()) },

	// Per-user settings.
	func(t store.TableNames) string { return string(t.Settings()) },

	// AI vision per-user audit + rate-limit rows.
	func(t store.TableNames) string { return string(t.CommodityScanAudits()) },

	// NB: group_memberships is intentionally NOT here — it is keyed solely by
	// member_user_id (no plain user_id column), so it can't ride the
	// user_id template. It is a deletionRemoves entry of userOwnedTables.

	// Per-user/per-group notification overrides.
	func(t store.TableNames) string { return string(t.GroupNotificationPrefs()) },

	// Weekly digest idempotency claims (#1391).
	func(t store.TableNames) string { return string(t.WeeklyDigestSends()) },

	// Where the member shows saved views, their own and the shared views of
	// others.
	func(t store.TableNames) string { return string(t.SavedViewPlacements()) },

	// Personal data exports (#2177). The archives are deleted by the caller.
	func(t store.TableNames) string { return string(t.PersonalDataExports()) },
}

// PurgeUserDependents clears every row that names the user, as the type doc
// describes, inside one background-worker transaction. All operations succeed
// or none do.
//
// Idempotent: a second call after a clean purge is a no-op (every statement
// affects zero rows).
//
// PRECONDITION (not enforced here): the user no longer has a row in a
// deletionBlocks table, other than in groups that are deleted before the
// users row. The purger does not touch those rows, so the final DELETE FROM
// users fails with an FK violation if any remain, which is the intended loud
// failure rather than a silent content wipe.
func (r *UserPurger) PurgeUserDependents(ctx context.Context, tenantID, userID string) error {
	if tenantID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	return store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		handed := handOvers{}
		inTenant := []func(context.Context, *sqlx.Tx, string, string) error{
			// Thumbnail chain (#2117 parity): user_concurrency_slots.job_id
			// -> thumbnail_generation_jobs (NO ACTION) and
			// thumbnail_generation_jobs.user_id -> users (NO ACTION).
			r.purgeThumbnailChain,
			r.purgeByUserID,
			// The hand-over runs while the memberships it picks heirs from
			// are still there.
			func(ctx context.Context, tx *sqlx.Tx, tenantID, userID string) error {
				return r.handOverGroupRows(ctx, tx, handed, tenantID, userID)
			},
			func(ctx context.Context, tx *sqlx.Tx, tenantID, userID string) error {
				return r.reattributeIntakeDocuments(ctx, tx, handed, tenantID, userID)
			},
			func(ctx context.Context, tx *sqlx.Tx, tenantID, userID string) error {
				return r.recordHandOvers(ctx, tx, handed, tenantID, userID)
			},
			// The file links to the drafts and intake sources about to go.
			r.purgeLinksToRemovedRows,
			r.purgeRemovedRows,
		}
		for _, step := range inTenant {
			if err := step(ctx, tx, tenantID, userID); err != nil {
				return err
			}
		}

		// group_invites_audit immortalises who created and who used each
		// invite, and an accepted invite records who used it. The columns are
		// FK -> users(id) with NO ACTION and match the user in every tenant: a
		// user moved to another tenant (#2179) leaves them in the tenant they
		// left.
		if err := r.purgeGroupInvitesAudit(ctx, tx, userID); err != nil {
			return err
		}
		if err := r.purgeAcceptedInvites(ctx, tx, userID); err != nil {
			return err
		}

		// system_admin_grants is NON-RLS and carries NO tenant_id column, so it
		// can't ride the tenant_id template. Two FKs point at users(id):
		//   user_id    -> ON DELETE CASCADE (the granted user) — DELETE the row.
		//   granted_by -> ON DELETE SET NULL (the operator who granted it,
		//                 NULLABLE) — null it so deleting this user as an
		//                 *operator* doesn't drop someone else's grant.
		return r.purgeSystemAdminGrants(ctx, tx, userID)
	})
}

// purgeByUserID deletes the user's rows from every userDeleteByUserID table.
func (r *UserPurger) purgeByUserID(ctx context.Context, tx *sqlx.Tx, tenantID, userID string) error {
	for _, nameFn := range userDeleteByUserID {
		table := nameFn(r.tableNames)
		query := fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1 AND user_id = $2", table)
		if _, err := tx.ExecContext(ctx, query, tenantID, userID); err != nil {
			return errxtrace.Wrap(
				"failed to purge user dependents",
				err,
				errx.Attrs("table", table, "tenant_id", tenantID, "user_id", userID),
			)
		}
	}
	return nil
}

// handOverGroupRows gives the rows of every deletionHandsOver table the user
// created to another member of the row's group, see reattributeRows, and
// counts them in handed.
func (r *UserPurger) handOverGroupRows(ctx context.Context, tx *sqlx.Tx, handed handOvers, tenantID, userID string) error {
	for _, t := range userOwnedTables {
		if t.onDeletion != deletionHandsOver {
			continue
		}
		if err := r.reattributeRows(ctx, tx, t, handed, "", tenantID, userID); err != nil {
			return err
		}
	}
	return nil
}

// purgeRemovedRows deletes the user's rows from every deletionRemoves table
// in the tenant, in list order.
func (r *UserPurger) purgeRemovedRows(ctx context.Context, tx *sqlx.Tx, tenantID, userID string) error {
	for _, t := range userOwnedTables {
		if t.onDeletion != deletionRemoves {
			continue
		}
		table := string(t.table(r.tableNames))
		query := fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1 AND %s = $2", table, t.userColumn)
		if _, err := tx.ExecContext(ctx, query, tenantID, userID); err != nil {
			return errxtrace.Wrap(
				"failed to purge the user's rows",
				err,
				errx.Attrs("table", table, "tenant_id", tenantID, "user_id", userID),
			)
		}
	}
	return nil
}

// purgeLinksToRemovedRows deletes the file links to the drafts (#2812) and
// intake sources (#2810) the user created, which purgeRemovedRows deletes
// next. The files stay: the ones another member uploaded become standalone
// files, and the user's own are content the pre-check counts.
func (r *UserPurger) purgeLinksToRemovedRows(ctx context.Context, tx *sqlx.Tx, tenantID, userID string) error {
	for _, owner := range []struct {
		entityType models.FileLinkEntityType
		table      store.TableName
	}{
		{models.FileLinkDraft, r.tableNames.CommodityDrafts()},
		{models.FileLinkIntake, r.tableNames.IntakeSources()},
	} {
		query := fmt.Sprintf(`
			DELETE FROM %s
			WHERE entity_type = $1 AND entity_id IN (
				SELECT id FROM %s WHERE tenant_id = $2 AND created_by_user_id = $3
			)`, r.tableNames.FileLinks(), owner.table)
		if _, err := tx.ExecContext(ctx, query, owner.entityType, tenantID, userID); err != nil {
			return errxtrace.Wrap("failed to purge the links to the user's rows", err,
				errx.Attrs("table", string(owner.table), "tenant_id", tenantID, "user_id", userID))
		}
	}
	return nil
}

// purgeThumbnailChain removes the thumbnail generation jobs the user created
// (and their concurrency slots). Neither user_concurrency_slots nor (in this
// chain) the slots carry the user_id directly usable for the slot delete —
// slots are scoped through the jobs they reference. Both DELETEs run BEFORE the
// jobs are removed, in FK-safe order (slots -> jobs).
//
// thumbnail_generation_jobs carries tenant_id + user_id, so the jobs DELETE is
// tenant-scoped as defense-in-depth.
//
// Idempotent: a user with no jobs matches zero rows on every statement.
func (r *UserPurger) purgeThumbnailChain(ctx context.Context, tx *sqlx.Tx, tenantID, userID string) error {
	jobs := string(r.tableNames.ThumbnailGenerationJobs())
	slots := string(r.tableNames.UserConcurrencySlots())

	jobSubquery := fmt.Sprintf(
		"SELECT id FROM %s WHERE tenant_id = $1 AND user_id = $2", jobs,
	)

	slotQuery := fmt.Sprintf(
		"DELETE FROM %s WHERE job_id IN (%s)", slots, jobSubquery,
	)
	if _, err := tx.ExecContext(ctx, slotQuery, tenantID, userID); err != nil {
		return errxtrace.Wrap(
			"failed to purge user thumbnail concurrency slots",
			err,
			errx.Attrs("table", slots, "tenant_id", tenantID, "user_id", userID),
		)
	}

	jobQuery := fmt.Sprintf(
		"DELETE FROM %s WHERE tenant_id = $1 AND user_id = $2", jobs,
	)
	if _, err := tx.ExecContext(ctx, jobQuery, tenantID, userID); err != nil {
		return errxtrace.Wrap(
			"failed to purge user thumbnail generation jobs",
			err,
			errx.Attrs("table", jobs, "tenant_id", tenantID, "user_id", userID),
		)
	}

	return nil
}

// purgeGroupInvitesAudit removes every group_invites_audit row that names the
// user as the invite creator (created_by) OR the accepter (used_by). Both are
// NOT NULL FK -> users(id) NO ACTION, so they must be cleared before the
// orchestration layer drops the users row. The rows are matched by user in
// every tenant: a user moved to another tenant (#2179) leaves this audit trail
// in the tenant they left. Idempotent: a user who never touched an invite
// matches zero rows.
func (r *UserPurger) purgeGroupInvitesAudit(ctx context.Context, tx *sqlx.Tx, userID string) error {
	audit := string(r.tableNames.GroupInvitesAudit())
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE created_by = $1 OR used_by = $1", audit,
	)
	if _, err := tx.ExecContext(ctx, query, userID); err != nil {
		return errxtrace.Wrap(
			"failed to purge user group_invites_audit references",
			err,
			errx.Attrs("table", audit, "user_id", userID),
		)
	}
	return nil
}

// purgeAcceptedInvites deletes the invites the user accepted, in every
// tenant. used_by is nullable, but an invite with no used_by can be accepted
// again, so the row goes rather than the column. The invites the user created
// are a deletionRemoves table and go with purgeRemovedRows.
func (r *UserPurger) purgeAcceptedInvites(ctx context.Context, tx *sqlx.Tx, userID string) error {
	invites := string(r.tableNames.GroupInvites())
	query := fmt.Sprintf("DELETE FROM %s WHERE used_by = $1", invites)
	if _, err := tx.ExecContext(ctx, query, userID); err != nil {
		return errxtrace.Wrap("failed to purge the invites the user accepted", err,
			errx.Attrs("table", invites, "user_id", userID))
	}
	return nil
}

// purgeSystemAdminGrants handles the non-RLS system_admin_grants table, which
// has no tenant_id column. The grant the user HAS is deleted; any grant the
// user GRANTED to someone else has its granted_by nulled (nullable column,
// schema-level ON DELETE SET NULL — done explicitly so the purge transaction
// owns the operation). Both statements are idempotent.
func (r *UserPurger) purgeSystemAdminGrants(ctx context.Context, tx *sqlx.Tx, userID string) error {
	grants := string(r.tableNames.SystemAdminGrants())

	nullQuery := fmt.Sprintf(
		"UPDATE %s SET granted_by = NULL WHERE granted_by = $1", grants,
	)
	if _, err := tx.ExecContext(ctx, nullQuery, userID); err != nil {
		return errxtrace.Wrap(
			"failed to null system_admin_grants.granted_by for user",
			err,
			errx.Attrs("table", grants, "user_id", userID),
		)
	}

	deleteQuery := fmt.Sprintf(
		"DELETE FROM %s WHERE user_id = $1", grants,
	)
	if _, err := tx.ExecContext(ctx, deleteQuery, userID); err != nil {
		return errxtrace.Wrap(
			"failed to purge system_admin_grants for user",
			err,
			errx.Attrs("table", grants, "user_id", userID),
		)
	}

	return nil
}

// reattributeIntakeDocuments finishes the hand-over of the documents
// delivered through the user's API tokens (#2809), which handOverGroupRows
// gave to another member. A document left with the user has a group with no
// other member and is deleted: its group is private and leaves with the user.
// The drafts the documents made go to the same member, so a replay finds the
// draft the record names; the ones whose group has no heir are purged with
// the user's other drafts.
func (r *UserPurger) reattributeIntakeDocuments(ctx context.Context, tx *sqlx.Tx, handed handOvers, tenantID, userID string) error {
	docs := r.tableNames.IntakeDocuments()
	rest := fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1 AND created_by_user_id = $2", docs)
	if _, err := tx.ExecContext(ctx, rest, tenantID, userID); err != nil {
		return errxtrace.Wrap("failed to purge the user's delivered documents", err,
			errx.Attrs("table", string(docs), "tenant_id", tenantID, "user_id", userID))
	}
	drafts := userOwnedTable{
		table:       func(t store.TableNames) store.TableName { return t.CommodityDrafts() },
		userColumn:  createdByUserIDColumn,
		groupColumn: "group_id",
	}
	return r.reattributeRows(ctx, tx, drafts, handed, "t.client_key LIKE $3",
		tenantID, userID, models.IntakeDeliveredDraftKeyPrefix+"%")
}

// reattributeRows gives each row of t the user created to another member of
// its group: the owner first, then an admin, then the member who joined
// first. The user's own membership is left out of the choice. A row whose
// group has no other member keeps its creator; such a group is private and is
// purged before the user is. A filter that is not empty further limits the
// rows of t; its parameters start at $3. Each row handed over is counted in
// handed.
func (r *UserPurger) reattributeRows(ctx context.Context, tx *sqlx.Tx, t userOwnedTable, handed handOvers, filter, tenantID, userID string, filterArgs ...any) error {
	if filter != "" {
		filter = " AND " + filter
	}
	table := string(t.table(r.tableNames))
	memberships := string(r.tableNames.GroupMemberships())
	query := fmt.Sprintf(`
		UPDATE %[1]s t SET %[3]s = heir.member_user_id
		FROM (
			SELECT DISTINCT ON (group_id) group_id, member_user_id
			FROM %[2]s
			WHERE tenant_id = $1 AND member_user_id <> $2
			ORDER BY group_id,
				CASE role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,
				joined_at, member_user_id
		) heir
		WHERE t.tenant_id = $1 AND t.%[3]s = $2 AND t.%[4]s = heir.group_id%[5]s
		RETURNING heir.group_id, heir.member_user_id`,
		table, memberships, t.userColumn, t.groupColumn, filter)
	rows, err := tx.QueryContext(ctx, query, append([]any{tenantID, userID}, filterArgs...)...)
	if err != nil {
		return errxtrace.Wrap(
			"failed to hand over the user's group rows",
			err,
			errx.Attrs("table", table, "tenant_id", tenantID, "user_id", userID),
		)
	}
	defer rows.Close()
	for rows.Next() {
		var groupID, heir string
		if err := rows.Scan(&groupID, &heir); err != nil {
			return errxtrace.Wrap("failed to read a handed over row", err, errx.Attrs("table", table))
		}
		handed.add(groupID, heir, table)
	}
	if err := rows.Err(); err != nil {
		return errxtrace.Wrap("failed to read the handed over rows", err, errx.Attrs("table", table))
	}
	return nil
}

// handOvers counts, per group, the rows the purge gave to another member.
type handOvers map[string]*groupHandOver

// groupHandOver is what one group's heir received: the count of rows per
// table.
type groupHandOver struct {
	heir string
	rows map[string]int
}

func (h handOvers) add(groupID, heir, table string) {
	g, ok := h[groupID]
	if !ok {
		g = &groupHandOver{heir: heir, rows: map[string]int{}}
		h[groupID] = g
	}
	g.rows[table]++
}

// recordHandOvers writes one audit row per group whose rows the purge handed
// over, in the purge transaction: the deleted user as the actor, the group
// as the entity, and the member who received the rows with the count per
// table in the breadcrumb. The rows keep the request id and the impersonator
// of the request that deleted the account, when there is one.
func (r *UserPurger) recordHandOvers(ctx context.Context, tx *sqlx.Tx, handed handOvers, tenantID, userID string) error {
	groupIDs := slices.Sorted(maps.Keys(handed))
	audit := store.NewTxRegistry[models.AuditLog](tx, r.tableNames.AuditLogs())
	for _, groupID := range groupIDs {
		g := handed[groupID]
		breadcrumb, err := json.Marshal(map[string]any{"to_user_id": g.heir, "rows": g.rows})
		if err != nil {
			return errxtrace.Wrap("failed to encode the hand-over", err)
		}
		entry := models.AuditLog{
			ID:         uuid.NewString(),
			UUID:       uuid.NewString(),
			Timestamp:  time.Now(),
			UserID:     new(userID),
			TenantID:   new(tenantID),
			Action:     registry.AuditActionAccountDeletionHandOver,
			EntityType: new("location_group"),
			EntityID:   new(groupID),
			UserAgent:  string(breadcrumb),
			Success:    true,
		}
		if id := appctx.RequestIDFromContext(ctx); id != "" {
			entry.RequestID = new(id)
		}
		if by := appctx.ImpersonatorID(ctx); by != "" {
			entry.ImpersonatedBy = new(by)
		}
		if err := audit.Insert(ctx, entry); err != nil {
			return errxtrace.Wrap("failed to record the hand-over", err, errx.Attrs("group_id", groupID))
		}
	}
	return nil
}
