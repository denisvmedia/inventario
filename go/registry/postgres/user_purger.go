package postgres

import (
	"context"
	"fmt"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.UserPurger = (*UserPurger)(nil)

// UserPurger hard-deletes a single user's auth / identity rows and orphans the
// few user-authorship columns that are nullable, all in one background-worker
// transaction (#2116). It exists because the bare `DELETE FROM users` that the
// GDPR "delete my data" path used to issue is rejected by ~43 NO ACTION child
// FKs pointing at users(id) — the worker must clear (or null) every dependent
// before the parent users row can be removed by the orchestration layer.
//
// IMPORTANT classification (see the migration audit in the PR description):
//   - DELETE rows the user OWNS (auth/identity/per-user idempotency): refresh
//     tokens, login events, email verifications, password resets, magic-link
//     tokens, MFA secret, OAuth identities, system-admin grant (user_id),
//     settings, group memberships, operation slots, commodity scan audits,
//     the user_concurrency_slots / thumbnail_generation_jobs the user created,
//     and the group_invites_audit rows that reference the user as the invite
//     creator (created_by) or accepter (used_by) — both NOT NULL FK -> users(id)
//     NO ACTION, so they must be cleared here or the final users delete is
//     rejected (#2147).
//   - SET NULL the authorship back-refs that are NULLABLE: login_events.user_id
//     (already nulled by the DELETE-by-user above — see note), and
//     system_admin_grants.granted_by (the *operator* who granted someone else's
//     grant; nullable + already ON DELETE SET NULL at the schema level, but we
//     null it explicitly so the purge transaction owns the scoping).
//
// CONTENT AUTHORSHIP THAT CANNOT BE ORPHANED (flagged, NOT handled here):
// every commodities/files/areas/locations/exports/location_groups/tags/...
// row carries BOTH a NOT NULL user_id (the RLS owner) AND a NOT NULL
// created_by / created_by_user_id. Neither column is nullable, so a user's
// shared content CANNOT be orphaned without a schema migration. Deleting a
// user who still owns content is therefore impossible by design today — the
// orchestration layer must first re-own (transfer) or purge that content (e.g.
// the GroupPurger path) before calling PurgeUserDependents. See the PR summary
// table for the full list of NOT NULL created_by columns.
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
	// user_id template. It is handled by its own DELETE in PurgeUserDependents.

	// Per-user/per-group notification overrides.
	func(t store.TableNames) string { return string(t.GroupNotificationPrefs()) },
}

// PurgeUserDependents clears every user-scoped auth/identity row (and nulls the
// nullable authorship back-refs) for a single user, inside one background-worker
// transaction. All operations succeed or none do.
//
// Idempotent: a second call after a clean purge is a no-op (every statement
// affects zero rows).
//
// PRECONDITION (not enforced here, see the type doc): the user must no longer
// OWN any shared content (commodities/files/areas/locations/exports/groups/
// tags/...). Those rows carry NOT NULL user_id + NOT NULL created_by columns
// that cannot be orphaned, so this purger deliberately does not touch them; the
// final DELETE FROM users will fail with an FK violation if any remain, which
// is the intended loud failure rather than a silent content wipe.
func (r *UserPurger) PurgeUserDependents(ctx context.Context, tenantID, userID string) error {
	if tenantID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	return store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Thumbnail chain (#2117 parity): user_concurrency_slots.job_id ->
		// thumbnail_generation_jobs (NO ACTION) and
		// thumbnail_generation_jobs.user_id -> users (NO ACTION). Slots carry
		// no user_id of their own, so they're reached through the user's jobs.
		// Order: slots (deepest child) -> jobs.
		if err := r.purgeThumbnailChain(ctx, tx, tenantID, userID); err != nil {
			return err
		}

		// Tables keyed by user_id (+ tenant_id defense-in-depth).
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

		// Group memberships are keyed SOLELY by member_user_id (there is no
		// plain user_id column on this table), so this single DELETE removes
		// every membership row that names the user as the member. It lives here
		// rather than in the user_id loop above for exactly that reason.
		memberships := string(r.tableNames.GroupMemberships())
		memberQuery := fmt.Sprintf(
			"DELETE FROM %s WHERE tenant_id = $1 AND member_user_id = $2", memberships,
		)
		if _, err := tx.ExecContext(ctx, memberQuery, tenantID, userID); err != nil {
			return errxtrace.Wrap(
				"failed to purge user memberships by member_user_id",
				err,
				errx.Attrs("table", memberships, "tenant_id", tenantID, "user_id", userID),
			)
		}

		// group_invites_audit immortalises who created and who used each invite.
		// Both created_by and used_by are NOT NULL FK -> users(id) with NO ACTION,
		// so nothing else clears them — a user whose group ever had a USED invite
		// would otherwise block the final DELETE FROM users with an FK violation
		// (this is also the latent break in the admin Service.DeleteUser path).
		// The user can appear as creator, as accepter, or both, so the DELETE
		// matches either column. The table has no children, so order is free; it
		// rides the tenant_id template as defense-in-depth.
		if err := r.purgeGroupInvitesAudit(ctx, tx, tenantID, userID); err != nil {
			return err
		}

		// system_admin_grants is NON-RLS and carries NO tenant_id column, so it
		// can't ride the tenant_id template. Two FKs point at users(id):
		//   user_id    -> ON DELETE CASCADE (the granted user) — DELETE the row.
		//   granted_by -> ON DELETE SET NULL (the operator who granted it,
		//                 NULLABLE) — null it so deleting this user as an
		//                 *operator* doesn't drop someone else's grant.
		if err := r.purgeSystemAdminGrants(ctx, tx, userID); err != nil {
			return err
		}

		return nil
	})
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
// orchestration layer drops the users row. tenant_id is added as
// defense-in-depth. Idempotent: a user who never touched an invite matches zero
// rows.
func (r *UserPurger) purgeGroupInvitesAudit(ctx context.Context, tx *sqlx.Tx, tenantID, userID string) error {
	audit := string(r.tableNames.GroupInvitesAudit())
	query := fmt.Sprintf(
		"DELETE FROM %s WHERE tenant_id = $1 AND (created_by = $2 OR used_by = $2)", audit,
	)
	if _, err := tx.ExecContext(ctx, query, tenantID, userID); err != nil {
		return errxtrace.Wrap(
			"failed to purge user group_invites_audit references",
			err,
			errx.Attrs("table", audit, "tenant_id", tenantID, "user_id", userID),
		)
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
