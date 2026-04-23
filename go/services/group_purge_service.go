package services

import (
	"context"
	"log/slog"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// GroupPurgeService orchestrates hard-deletion of LocationGroups that were
// previously marked pending_deletion. A single PurgeOnce tick:
//
//  1. lists every LocationGroup in pending_deletion state (across tenants via
//     the service-mode registry, which runs as inventario_background_worker);
//  2. for each group, snapshots its used invites into group_invites_audit,
//     deletes physical blobs (fail-fast), bulk-deletes group-scoped rows via
//     GroupPurger, removes remaining invites, and finally deletes the
//     location_groups row itself.
//
// Per-group failures are logged and the tick continues with the next group —
// a failed group stays pending_deletion and will be retried on the next tick.
// That idempotency is what makes fail-fast blob deletion safe.
type GroupPurgeService struct {
	factorySet  *registry.FactorySet
	fileService *FileService
}

// NewGroupPurgeService constructs the orchestrator. Both dependencies are
// required.
func NewGroupPurgeService(factorySet *registry.FactorySet, fileService *FileService) *GroupPurgeService {
	return &GroupPurgeService{factorySet: factorySet, fileService: fileService}
}

// PurgeOnce runs one full sweep across every pending_deletion LocationGroup
// and returns the number of successfully purged groups alongside the number
// that failed. Errors for individual groups are logged but do not abort the
// sweep — a non-nil error is only returned when the initial listing itself
// fails.
func (s *GroupPurgeService) PurgeOnce(ctx context.Context) (purged, failed int, err error) {
	if s.factorySet == nil || s.factorySet.LocationGroupRegistry == nil {
		return 0, 0, errxtrace.Wrap("factorySet.LocationGroupRegistry is not configured", registry.ErrFieldRequired)
	}
	all, err := s.factorySet.LocationGroupRegistry.List(ctx)
	if err != nil {
		return 0, 0, errxtrace.Wrap("failed to list location_groups", err)
	}

	for _, g := range all {
		if g == nil || g.Status != models.LocationGroupStatusPendingDeletion {
			continue
		}
		if err := s.purgeGroup(ctx, g); err != nil {
			failed++
			slog.Error("group purge failed",
				"group_id", g.ID,
				"tenant_id", g.TenantID,
				"error", err)
			continue
		}
		purged++
		slog.Info("group purged",
			"group_id", g.ID,
			"tenant_id", g.TenantID,
			"slug", g.Slug)
	}
	return purged, failed, nil
}

// CleanExpiredInvites deletes every unused invite whose ExpiresAt is in the
// past (spec #1309 Option 2i). Used invites are never touched here — they
// are snapshotted into group_invites_audit during per-group purge.
func (s *GroupPurgeService) CleanExpiredInvites(ctx context.Context) (int, error) {
	if s.factorySet == nil || s.factorySet.GroupInviteRegistry == nil {
		return 0, errxtrace.Wrap("factorySet.GroupInviteRegistry is not configured", registry.ErrFieldRequired)
	}
	n, err := s.factorySet.GroupInviteRegistry.DeleteExpiredUnused(ctx, time.Now())
	if err != nil {
		return 0, errxtrace.Wrap("failed to delete expired unused invites", err)
	}
	return n, nil
}

// purgeGroup executes the full purge sequence for a single group. Order
// matters — see doc on GroupPurgeService.
func (s *GroupPurgeService) purgeGroup(ctx context.Context, g *models.LocationGroup) error {
	// 1) snapshot used invites into the audit table BEFORE deleting them.
	if err := s.snapshotUsedInvites(ctx, g); err != nil {
		return errxtrace.Wrap("failed to snapshot used invites", err)
	}
	// 2) physical blobs first — if object storage is unavailable we abort
	// without mutating the database, and the next tick retries.
	if err := s.fileService.DeletePhysicalFilesForGroup(ctx, g.TenantID, g.ID); err != nil {
		return errxtrace.Wrap("failed to delete physical blobs", err)
	}
	// 3) bulk-delete every group-scoped row in FK-safe order.
	if err := s.factorySet.GroupPurger.PurgeGroupDependents(ctx, g.TenantID, g.ID); err != nil {
		return errxtrace.Wrap("failed to purge group dependents", err)
	}
	// 4) delete any remaining invites (used rows are already audited).
	if _, err := s.factorySet.GroupInviteRegistry.DeleteByGroup(ctx, g.ID); err != nil {
		return errxtrace.Wrap("failed to delete group invites", err)
	}
	// 5) finally, delete the location_groups row itself. Once this succeeds
	// the purge is durable — the row won't reappear on the next tick.
	if err := s.factorySet.LocationGroupRegistry.Delete(ctx, g.ID); err != nil {
		return errxtrace.Wrap("failed to delete location_groups row", err)
	}
	return nil
}

// snapshotUsedInvites copies every accepted invite for the group into
// group_invites_audit. Uses the group-scoped ListUsedByGroup query so cost
// scales with per-group invites rather than the full invite table. The audit
// Create is idempotent (unique index on tenant_id + original_invite_id), so
// retries after a mid-purge failure never produce duplicate rows.
func (s *GroupPurgeService) snapshotUsedInvites(ctx context.Context, g *models.LocationGroup) error {
	used, err := s.factorySet.GroupInviteRegistry.ListUsedByGroup(ctx, g.ID)
	if err != nil {
		return errxtrace.Wrap("failed to list used invites", err)
	}
	for _, inv := range used {
		if inv == nil {
			continue
		}
		if inv.TenantID != g.TenantID {
			continue
		}
		if inv.UsedBy == nil || *inv.UsedBy == "" {
			continue
		}
		audit := models.NewGroupInviteAuditFromInvite(inv, g)
		if _, err := s.factorySet.GroupInviteAuditRegistry.Create(ctx, *audit); err != nil {
			return errxtrace.Wrap("failed to write invite audit row", err)
		}
	}
	return nil
}
