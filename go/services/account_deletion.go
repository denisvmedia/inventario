package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// pgForeignKeyViolationCode is the Postgres SQLSTATE returned when a DELETE is
// rejected by a NO ACTION / RESTRICT foreign-key constraint (a child row still
// references the parent). It is the stable wire value across both drivers this
// project links (pgx's *pgconn.PgError and lib/pq's *pq.Error), so the predicate
// below matches it via the shared SQLState() interface rather than importing a
// driver-specific concrete type.
const pgForeignKeyViolationCode = "23503"

var (
	// ErrAccountSoleOwnerOfSharedGroup is returned by DeleteAccount when the
	// user is the only OWNER of a group that still has OTHER members. Erasing
	// the account would leave that shared group ownerless (and therefore
	// undeletable by anyone), so deletion is refused until the user promotes
	// another owner or transfers ownership. Surfaces as 422 auth.delete.last_owner.
	ErrAccountSoleOwnerOfSharedGroup = errx.NewSentinel("cannot delete account: you are the sole owner of a shared group — transfer ownership first")

	// ErrAccountStillOwnsContent is returned by DeleteAccount when the user still
	// owns content (commodities/files/areas/locations/exports/tags) authored in a
	// RETAINED group, or created a location_group that is not being purged. Those
	// rows carry a NOT NULL created_by_user_id (or created_by) FK to users(id)
	// that cannot be orphaned without a schema change, so the account cannot be
	// erased unilaterally. It is raised up-front by the read-only pre-check
	// (abort-before-mutate) and, as a TOCTOU backstop, by the final users-row
	// delete's foreign-key violation. Surfaces as 422 auth.delete.owns_content.
	//
	// The message is intentionally STATIC: it is rendered into the 422 response
	// body, so it must never carry the raw DB error text (which would leak the
	// schema / constraint name). The raw cause is logged, never wrapped in.
	ErrAccountStillOwnsContent = errx.NewSentinel("cannot delete account: you still own content in a shared group that cannot be erased unilaterally")
)

// AccountDeletionService orchestrates the immediate, self-service hard-deletion
// of a single user's account (GDPR right-to-erasure, #2147). It reuses the
// existing per-group purge machinery (#2158/#2095/#2115) rather than
// re-implementing the FK-safe delete order:
//
//  1. CLASSIFY every group the user belongs to:
//     - PRIVATE  (the user is the only member)            -> collect for purge
//     - SOLE-OWNER-OF-SHARED (owner, other members exist,
//     and the user is the only owner)                   -> abort with
//     ErrAccountSoleOwnerOfSharedGroup (delete nothing)
//     - otherwise (co-owner ≥2 owners, or non-owner)      -> leave untouched;
//     the membership row is removed by UserPurger in step 3.
//     1.5. PRE-CHECK content ownership (read-only). If the user authored content in
//     a RETAINED group, or created a location_group that is NOT being purged,
//     abort with ErrAccountStillOwnsContent — mutating nothing.
//  2. For each PRIVATE group, run GroupPurgeService.purgeGroup (blobs ->
//     GroupPurger -> invites -> location_groups row).
//  3. UserPurger.PurgeUserDependents (auth/identity + memberships + per-user rows
//     + the user's group_invites_audit references).
//  4. UserRegistry.Delete. A foreign-key violation here (SQLSTATE 23503) means a
//     concurrent request re-created user-owned content after the pre-check (a
//     TOCTOU race) -> ErrAccountStillOwnsContent. Any other error -> 500.
//
// The classify-first / pre-check-first / abort-before-mutating ordering is the
// safety contract: both the sole-owner-of-shared and the still-owns-content
// rejections touch nothing.
type AccountDeletionService struct {
	factorySet *registry.FactorySet
	purger     *GroupPurgeService
}

// NewAccountDeletionService constructs the orchestrator. The GroupPurgeService
// is reused for the per-group purge sequence; both dependencies are required.
func NewAccountDeletionService(factorySet *registry.FactorySet, purger *GroupPurgeService) *AccountDeletionService {
	return &AccountDeletionService{factorySet: factorySet, purger: purger}
}

// DeleteAccount hard-deletes the user identified by (tenantID, userID) per the
// sequence documented on AccountDeletionService. Re-authentication is verified
// by the HTTP handler before this is called; this method owns only the
// classification + purge + delete.
func (s *AccountDeletionService) DeleteAccount(ctx context.Context, tenantID, userID string) error {
	if s.factorySet == nil || s.purger == nil {
		return errxtrace.Wrap("AccountDeletionService is not configured", registry.ErrFieldRequired)
	}
	if tenantID == "" {
		return errxtrace.Wrap("tenantID required", registry.ErrFieldRequired)
	}
	if userID == "" {
		return errxtrace.Wrap("userID required", registry.ErrFieldRequired)
	}

	// 1) Classify the user's memberships. Aborts (touching nothing) when the
	// user is the sole owner of a still-shared group.
	privateGroupIDs, err := s.classifyGroups(ctx, tenantID, userID)
	if err != nil {
		return err
	}

	// 1.5) Up-front content-ownership pre-check (abort BEFORE mutating). The
	// private groups in privateGroupIDs are purged wholesale below, so content
	// there does not block deletion. But any content the user authored in a
	// RETAINED group — or any location_group the user created that is NOT being
	// purged — carries a NOT NULL FK to users(id) that cannot be orphaned. If any
	// such row exists, refuse the delete now, mutating nothing, rather than
	// discovering it at the final DELETE FROM users (which would leave a
	// half-erased account). This mirrors the abort-before-mutate contract of the
	// sole-owner-of-shared case above. The final-delete FK→sentinel mapping in
	// step 4 remains a backstop for the TOCTOU window between here and there.
	if s.factorySet.UserContentOwnershipChecker != nil {
		owns, err := s.factorySet.UserContentOwnershipChecker.HasRetainedOwnedContent(ctx, tenantID, userID, privateGroupIDs)
		if err != nil {
			return errxtrace.Wrap("failed to pre-check retained owned content", err)
		}
		if owns {
			return errxtrace.Classify(ErrAccountStillOwnsContent)
		}
	}

	// 2) Purge each PRIVATE group via the existing per-group sequence.
	for _, gid := range privateGroupIDs {
		group, err := s.factorySet.LocationGroupRegistry.Get(ctx, gid)
		if err != nil {
			return errxtrace.Wrap("failed to load private group for purge", err)
		}
		if err := s.purger.purgeGroup(ctx, group); err != nil {
			return errxtrace.Wrap("failed to purge private group", err)
		}
	}

	// 3) Purge the user's auth/identity dependent rows (and any remaining
	// membership rows in groups left untouched in step 1).
	if err := s.factorySet.UserPurger.PurgeUserDependents(ctx, tenantID, userID); err != nil {
		return errxtrace.Wrap("failed to purge user dependents", err)
	}

	// 4) Drop the users row. The up-front pre-check in step 1.5 already refused
	// the delete if the user still owns retained content, so this DELETE should
	// succeed. It remains a TOCTOU backstop: if a concurrent request created
	// user-owned content in a retained group between the pre-check and here, the
	// NOT NULL created_by_user_id FK rejects the DELETE with SQLSTATE 23503 —
	// translate ONLY that to the typed sentinel so the handler returns 422
	// auth.delete.owns_content. Any OTHER error (a transient infra blip, a
	// connection drop) is returned as the wrapped original so the handler maps
	// it to 500 instead of a misleading permanent 422. Crucially, the sentinel's
	// message is STATIC (see ErrAccountStillOwnsContent) and the raw DB error is
	// kept only as a log line — it must never reach the client body, which would
	// leak the schema/constraint name.
	if err := s.factorySet.UserRegistry.Delete(ctx, userID); err != nil {
		if isForeignKeyViolation(err) {
			slog.Warn("account deletion: users-row delete rejected by FK (still owns content)",
				"user_id", userID, "tenant_id", tenantID, "error", err)
			return errxtrace.Classify(ErrAccountStillOwnsContent)
		}
		return errxtrace.Wrap("failed to delete user row", err)
	}

	return nil
}

// isForeignKeyViolation reports whether err is a Postgres foreign-key violation
// (SQLSTATE 23503). It reads the code through the SQLState() interface that both
// linked drivers expose (pgx *pgconn.PgError, lib/pq *pq.Error), so it stays
// driver-agnostic and pulls in neither concrete type. The memory backend never
// raises this — its Delete is a map operation — so a false here on memory is
// correct (a leftover row is a logic error surfaced as 500, not an FK 422).
func isForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	var sqlStater interface{ SQLState() string }
	if errors.As(err, &sqlStater) {
		return sqlStater.SQLState() == pgForeignKeyViolationCode
	}
	return false
}

// classifyGroups walks every group the user is a member of and returns the IDs
// of the PRIVATE groups (user is the only member) that must be purged. It
// returns ErrAccountSoleOwnerOfSharedGroup — before collecting anything — when
// the user is the only owner of a group that still has other members.
func (s *AccountDeletionService) classifyGroups(ctx context.Context, tenantID, userID string) ([]string, error) {
	memberships, err := s.factorySet.GroupMembershipRegistry.ListByUser(ctx, tenantID, userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list user memberships", err)
	}

	privateGroupIDs := make([]string, 0, len(memberships))
	for _, m := range memberships {
		if m == nil || m.GroupID == "" {
			continue
		}

		groupMembers, err := s.factorySet.GroupMembershipRegistry.ListByGroup(ctx, m.GroupID)
		if err != nil {
			return nil, errxtrace.Wrap("failed to list group members", err)
		}

		// PRIVATE group: the user is the only member -> purge the whole group.
		if len(groupMembers) <= 1 {
			privateGroupIDs = append(privateGroupIDs, m.GroupID)
			continue
		}

		// Shared group: only block when the user is its SOLE owner. A
		// co-owner (≥2 owners) or a non-owner member leaves the group intact;
		// the membership row is dropped by UserPurger. We already know the
		// current user is an owner here, so an owner count of one means the
		// user is that sole owner.
		if m.Role != models.GroupRoleOwner {
			continue
		}
		if countOwners(groupMembers) <= 1 {
			return nil, errxtrace.Classify(ErrAccountSoleOwnerOfSharedGroup)
		}
	}

	return privateGroupIDs, nil
}

// countOwners returns the number of memberships with the owner role.
func countOwners(members []*models.GroupMembership) int {
	ownerCount := 0
	for _, m := range members {
		if m == nil {
			continue
		}
		if m.Role == models.GroupRoleOwner {
			ownerCount++
		}
	}
	return ownerCount
}
