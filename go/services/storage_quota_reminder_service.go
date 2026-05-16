package services

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// StorageQuotaReminderService scans every group, computes its
// used/quota ratio, and:
//
//  1. emits one email per group per quota tier crossed via
//     idempotency rows (write-after-send; same race ordering as
//     WarrantyReminderService);
//  2. resets a previously-emitted row when a group's usage falls
//     back below the threshold, so a future re-crossing fires a
//     fresh email.
//
// The service is stateless — RemindOnce takes the clock as an
// argument so unit tests can pin "now" to a fixed value and assert
// the cadence boundaries without time-injection scaffolding.
//
// Per-user notification preferences are NOT consulted in v1: the
// reminder targets group admins and is operational rather than
// product-promotional. Hooking into the notifications.Service
// surface is tracked as a follow-up (per the issue body).
type StorageQuotaReminderService struct {
	factorySet *registry.FactorySet
	emailSvc   EmailService

	// quotaBytesFor returns the per-group quota in bytes. Defaults to
	// DefaultGroupStorageQuotaBytes when nil; tests inject a tiny
	// quota so the threshold-crossing cases are deterministic without
	// staging hundreds of MiB of test files.
	quotaBytesFor func(groupID string) int64

	// urlBuilders compose the deep links printed in the reminder
	// email. Both are optional — when nil, the template suppresses the
	// matching link block. publicURL ownership lives in the bootstrap
	// layer, like the warranty reminder.
	filesURLBuilder    func(groupSlug string) string
	settingsURLBuilder func(groupSlug string) string
}

// NewStorageQuotaReminderService constructs the service. emailSvc may
// be nil in tests that only assert the idempotency-row side; in
// production callers always pass a non-nil EmailService.
func NewStorageQuotaReminderService(
	factorySet *registry.FactorySet,
	emailSvc EmailService,
	filesURLBuilder func(groupSlug string) string,
	settingsURLBuilder func(groupSlug string) string,
) *StorageQuotaReminderService {
	return &StorageQuotaReminderService{
		factorySet:         factorySet,
		emailSvc:           emailSvc,
		filesURLBuilder:    filesURLBuilder,
		settingsURLBuilder: settingsURLBuilder,
	}
}

// WithQuotaBytesFor lets callers (tests + a future plans-aware
// runtime) override the per-group quota lookup. Returns the same
// service for fluent chaining.
func (s *StorageQuotaReminderService) WithQuotaBytesFor(fn func(groupID string) int64) *StorageQuotaReminderService {
	s.quotaBytesFor = fn
	return s
}

// StorageQuotaReminderStats summarises the outcome of one sweep.
// SentByThreshold counts new idempotency rows partitioned by tier;
// ResetByThreshold counts rows wiped because their group dropped
// back below the threshold. Failed is the cross-tier count of groups
// whose threshold processing failed (typically transient queue
// outages or DB errors); the next sweep retries.
type StorageQuotaReminderStats struct {
	SentByThreshold  map[models.StorageQuotaThreshold]int
	ResetByThreshold map[models.StorageQuotaThreshold]int
	Failed           int
}

// Sent returns the total number of newly-inserted idempotency rows
// across every threshold. Convenience for callers that don't need
// the per-threshold breakdown.
func (s StorageQuotaReminderStats) Sent() int {
	total := 0
	for _, v := range s.SentByThreshold {
		total += v
	}
	return total
}

// Reset returns the total number of reminder rows wiped this sweep
// because their group fell back below the matched threshold.
func (s StorageQuotaReminderStats) Reset() int {
	total := 0
	for _, v := range s.ResetByThreshold {
		total += v
	}
	return total
}

// RemindOnce runs one sweep pinned to `now`. Returns the per-tier
// counters; a non-nil error is only returned when the initial group
// listing itself fails.
func (s *StorageQuotaReminderService) RemindOnce(ctx context.Context, now time.Time) (StorageQuotaReminderStats, error) {
	stats := StorageQuotaReminderStats{
		SentByThreshold:  map[models.StorageQuotaThreshold]int{},
		ResetByThreshold: map[models.StorageQuotaThreshold]int{},
	}
	if s.factorySet == nil {
		return stats, errxtrace.Wrap("storage quota reminder service: factorySet is required", registry.ErrFieldRequired)
	}
	if s.factorySet.LocationGroupRegistry == nil {
		return stats, errxtrace.Wrap("storage quota reminder service: LocationGroupRegistry is required", registry.ErrFieldRequired)
	}
	if s.factorySet.StorageQuotaReminderRegistry == nil {
		return stats, errxtrace.Wrap("storage quota reminder service: StorageQuotaReminderRegistry is required", registry.ErrFieldRequired)
	}
	if s.factorySet.FileRegistryFactory == nil {
		return stats, errxtrace.Wrap("storage quota reminder service: FileRegistryFactory is required", registry.ErrFieldRequired)
	}

	groups, err := s.factorySet.LocationGroupRegistry.List(ctx)
	if err != nil {
		return stats, errxtrace.Wrap("storage quota reminder: list groups", err)
	}

	fileReg := s.factorySet.FileRegistryFactory.CreateServiceRegistry()
	for _, g := range groups {
		if g == nil {
			continue
		}
		s.processGroup(ctx, fileReg, g, &stats, now)
	}
	return stats, nil
}

// processGroup runs one (group) leg of a sweep. Errors are folded
// into stats.Failed + slog rather than bubbled out so a single bad
// group never short-circuits the whole sweep.
func (s *StorageQuotaReminderService) processGroup(
	ctx context.Context,
	fileReg registry.FileRegistry,
	g *models.LocationGroup,
	stats *StorageQuotaReminderStats,
	now time.Time,
) {
	usedBytes, breakdown, ratioErr := s.computeGroupUsage(ctx, fileReg, g)
	if ratioErr != nil {
		stats.Failed++
		slog.Error("storage quota reminder: usage lookup failed",
			"group_id", g.ID,
			"tenant_id", g.TenantID,
			"error", ratioErr,
		)
		return
	}
	quota := s.quotaBytes(g.ID)
	if quota <= 0 {
		// No quota configured for this group — nothing to warn
		// about. Treat as "below every threshold" so any stale
		// reminder rows get reset.
		s.resetEveryThreshold(ctx, stats, g)
		return
	}
	ratio := float64(usedBytes) / float64(quota)
	for _, threshold := range models.StorageQuotaThresholds {
		if ratio >= threshold.Ratio() {
			s.processThresholdCrossed(ctx, g, threshold, usedBytes, quota, breakdown, now, stats)
			continue
		}
		s.processThresholdBelow(ctx, g, threshold, stats)
	}
}

// processThresholdCrossed handles the ratio >= threshold branch:
// HasSent → enqueue → CreateOnce, counting stats accordingly.
func (s *StorageQuotaReminderService) processThresholdCrossed(
	ctx context.Context,
	g *models.LocationGroup,
	threshold models.StorageQuotaThreshold,
	usedBytes, quotaBytes int64,
	breakdown registry.StorageBreakdown,
	now time.Time,
	stats *StorageQuotaReminderStats,
) {
	ok, processErr := s.processCrossed(ctx, g, threshold, usedBytes, quotaBytes, breakdown, now)
	if processErr != nil {
		stats.Failed++
		slog.Error("storage quota reminder failed",
			"group_id", g.ID,
			"tenant_id", g.TenantID,
			"threshold_percent", int(threshold),
			"error", processErr,
		)
		return
	}
	if ok {
		stats.SentByThreshold[threshold]++
	}
}

// processThresholdBelow handles the ratio < threshold branch:
// delete any prior reminder row so a future re-crossing fires fresh.
func (s *StorageQuotaReminderService) processThresholdBelow(
	ctx context.Context,
	g *models.LocationGroup,
	threshold models.StorageQuotaThreshold,
	stats *StorageQuotaReminderStats,
) {
	reset, resetErr := s.factorySet.StorageQuotaReminderRegistry.DeleteByGroupThreshold(ctx, g.ID, int(threshold))
	if resetErr != nil {
		stats.Failed++
		slog.Error("storage quota reminder: reset failed",
			"group_id", g.ID,
			"threshold_percent", int(threshold),
			"error", resetErr,
		)
		return
	}
	if reset {
		stats.ResetByThreshold[threshold]++
		slog.Info("storage quota reminder reset",
			"group_id", g.ID,
			"threshold_percent", int(threshold),
		)
	}
}

// resetEveryThreshold wipes every reminder tier for the given group.
// Called when a group has no configured quota — equivalent to
// "definitely below every threshold". Errors per threshold are
// counted into stats.Failed but never bubbled up: the next sweep
// retries.
func (s *StorageQuotaReminderService) resetEveryThreshold(ctx context.Context, stats *StorageQuotaReminderStats, g *models.LocationGroup) {
	for _, threshold := range models.StorageQuotaThresholds {
		reset, resetErr := s.factorySet.StorageQuotaReminderRegistry.DeleteByGroupThreshold(ctx, g.ID, int(threshold))
		if resetErr != nil {
			stats.Failed++
			slog.Error("storage quota reminder: reset failed (no-quota path)",
				"group_id", g.ID,
				"threshold_percent", int(threshold),
				"error", resetErr,
			)
			continue
		}
		if reset {
			stats.ResetByThreshold[threshold]++
		}
	}
}

// computeGroupUsage returns the group's total bytes used + the
// per-bucket breakdown. The breakdown is the same shape rendered in
// the FE storage card; reusing it in the email keeps the two
// surfaces consistent.
func (s *StorageQuotaReminderService) computeGroupUsage(
	ctx context.Context,
	files registry.FileRegistry,
	g *models.LocationGroup,
) (int64, registry.StorageBreakdown, error) {
	breakdown, err := files.SumSizeBreakdownByGroup(ctx, g.TenantID, g.ID)
	if err != nil {
		return 0, registry.StorageBreakdown{}, err
	}
	return breakdown.Total(), breakdown, nil
}

// processCrossed handles one (group, threshold) pair where the
// ratio is at or above the tier. Mirrors WarrantyReminderService's
// HasSent → enqueue → CreateOnce ordering (write-after-send) so a
// failed enqueue leaves no row and the next sweep retries.
func (s *StorageQuotaReminderService) processCrossed(
	ctx context.Context,
	g *models.LocationGroup,
	threshold models.StorageQuotaThreshold,
	usedBytes, quotaBytes int64,
	breakdown registry.StorageBreakdown,
	now time.Time,
) (bool, error) {
	already, err := s.factorySet.StorageQuotaReminderRegistry.HasSent(ctx, g.ID, int(threshold))
	if err != nil {
		return false, errxtrace.Wrap("storage quota reminder: check existing row", err)
	}
	if already {
		return false, nil
	}

	// Stub-mode short-circuit: tests / dev environments without an
	// email service still want the idempotency row written so the
	// sweep counter is meaningful.
	if s.emailSvc == nil {
		ok, commitErr := s.commitReminderRow(ctx, g, threshold, now)
		if commitErr != nil {
			return false, errxtrace.Wrap("storage quota reminder: insert idempotency row (no email service)", commitErr)
		}
		if ok {
			slog.Info("storage quota reminder row inserted (no email service configured)",
				"group_id", g.ID,
				"threshold_percent", int(threshold),
			)
		}
		return ok, nil
	}

	recipients, err := s.recipientsForGroup(ctx, g)
	if err != nil {
		return false, errxtrace.Wrap("storage quota reminder: resolve recipients", err)
	}
	if len(recipients) == 0 {
		// No admins to notify — write the row so the worker stops
		// re-evaluating this (group, threshold) on every tick. The
		// SentByThreshold counter still reflects "actually-emitted
		// emails", so we return false here even though a row was
		// committed: the unreachable-group case shouldn't bump the
		// reminders_sent counter the operator reads.
		_, commitErr := s.commitReminderRow(ctx, g, threshold, now)
		if commitErr != nil {
			return false, errxtrace.Wrap("storage quota reminder: insert idempotency row (no recipients)", commitErr)
		}
		slog.Warn("storage quota reminder: no admin recipients found for group",
			"group_id", g.ID,
			"tenant_id", g.TenantID,
		)
		return false, nil
	}

	filesURL := s.buildFilesURL(g.Slug)
	settingsURL := s.buildSettingsURL(g.Slug)
	usagePercent := int(float64(usedBytes) * 100 / float64(quotaBytes))
	usedHuman := formatBytes(usedBytes)
	quotaHuman := formatBytes(quotaBytes)
	breakdownLines := buildBreakdownLines(breakdown)

	enqueueErrs := 0
	var firstEnqueueErr error
	for _, r := range recipients {
		sendErr := s.emailSvc.SendStorageQuotaWarningEmail(
			ctx,
			r.email, r.name,
			g.Name,
			int(threshold), usagePercent,
			usedHuman, quotaHuman,
			breakdownLines,
			filesURL, settingsURL,
		)
		if sendErr != nil {
			enqueueErrs++
			if firstEnqueueErr == nil {
				firstEnqueueErr = sendErr
			}
			slog.Error("storage quota reminder: enqueue failed",
				"group_id", g.ID,
				"to", r.email,
				"error", sendErr,
			)
		}
	}
	if enqueueErrs == len(recipients) {
		// Every recipient enqueue failed. Don't commit the
		// idempotency row — let the next sweep retry.
		return false, errxtrace.Wrap("storage quota reminder: all enqueues failed", firstEnqueueErr)
	}
	ok, err := s.commitReminderRow(ctx, g, threshold, now)
	if err != nil {
		return false, errxtrace.Wrap("storage quota reminder: insert idempotency row", err)
	}
	return ok, nil
}

// commitReminderRow writes the (group, threshold) idempotency row
// stamped with the sweep's `now`. Returns (true, nil) iff this call
// won the unique-constraint race; (false, nil) for the loser.
//
// The reminder row's CreatedByUserID is the group's CreatedByUserID
// (an admin from the group lifecycle) — this is for audit only; the
// row is never user-readable.
func (s *StorageQuotaReminderService) commitReminderRow(
	ctx context.Context,
	g *models.LocationGroup,
	threshold models.StorageQuotaThreshold,
	now time.Time,
) (bool, error) {
	reminder := models.StorageQuotaReminder{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        g.TenantID,
			GroupID:         g.ID,
			CreatedByUserID: g.CreatedBy,
		},
		ThresholdPercent: int(threshold),
		SentAt:           now,
	}
	return s.factorySet.StorageQuotaReminderRegistry.CreateOnce(ctx, reminder)
}

type storageQuotaRecipient struct {
	email string
	name  string
}

// recipientsForGroup returns every admin user that should receive a
// quota warning for the given group. Falls back to the group's
// CreatedByUserID when the memberships registry is unwired or
// returns no admins, so a single-user / pre-#1533 setup still gets a
// notification.
func (s *StorageQuotaReminderService) recipientsForGroup(ctx context.Context, g *models.LocationGroup) ([]storageQuotaRecipient, error) {
	out := make([]storageQuotaRecipient, 0, 4)
	if s.factorySet.GroupMembershipRegistry != nil {
		members, err := s.factorySet.GroupMembershipRegistry.ListByGroup(ctx, g.ID)
		if err != nil {
			return nil, err
		}
		seen := make(map[string]struct{}, len(members))
		for _, m := range members {
			if m == nil || m.Role != models.GroupRoleAdmin {
				continue
			}
			user, err := s.factorySet.UserRegistry.Get(ctx, m.MemberUserID)
			if err != nil {
				slog.Warn("storage quota reminder: skip member with missing user",
					"user_id", m.MemberUserID,
					"group_id", g.ID,
					"error", err,
				)
				continue
			}
			if user == nil || strings.TrimSpace(user.Email) == "" {
				continue
			}
			if _, dup := seen[user.Email]; dup {
				continue
			}
			seen[user.Email] = struct{}{}
			out = append(out, storageQuotaRecipient{email: user.Email, name: user.Name})
		}
	}

	if len(out) == 0 && g.CreatedBy != "" {
		user, err := s.factorySet.UserRegistry.Get(ctx, g.CreatedBy)
		if err == nil && user != nil && strings.TrimSpace(user.Email) != "" {
			out = append(out, storageQuotaRecipient{email: user.Email, name: user.Name})
		}
	}
	return out, nil
}

// quotaBytes returns the configured quota for the group, falling
// back to DefaultGroupStorageQuotaBytes when no override is wired.
// Zero or negative means "no quota / disabled" — the worker then
// treats every tier as if the group were below it.
func (s *StorageQuotaReminderService) quotaBytes(groupID string) int64 {
	if s.quotaBytesFor != nil {
		return s.quotaBytesFor(groupID)
	}
	return DefaultGroupStorageQuotaBytes
}

func (s *StorageQuotaReminderService) buildFilesURL(slug string) string {
	if s.filesURLBuilder == nil || slug == "" {
		return ""
	}
	return s.filesURLBuilder(slug)
}

func (s *StorageQuotaReminderService) buildSettingsURL(slug string) string {
	if s.settingsURLBuilder == nil || slug == "" {
		return ""
	}
	return s.settingsURLBuilder(slug)
}

// formatBytes renders a byte count as a short human-readable string
// (e.g. "135 MiB", "1.2 GiB"). Used in the email body so the
// recipient sees "135 MiB / 150 MiB" rather than raw byte counts.
// Binary units match the FE storage card.
func formatBytes(n int64) string {
	const (
		kib = 1024
		mib = 1024 * kib
		gib = 1024 * mib
	)
	switch {
	case n >= gib:
		return fmt.Sprintf("%.1f GiB", float64(n)/float64(gib))
	case n >= mib:
		return fmt.Sprintf("%.0f MiB", float64(n)/float64(mib))
	case n >= kib:
		return fmt.Sprintf("%.0f KiB", float64(n)/float64(kib))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

// buildBreakdownLines turns a StorageBreakdown into the per-bucket
// label slice rendered in the email body. Buckets that are
// zero-sized are dropped — no "Images: 0 B" rows.
func buildBreakdownLines(b registry.StorageBreakdown) []string {
	lines := make([]string, 0, 4)
	if b.Images > 0 {
		lines = append(lines, fmt.Sprintf("Images: %s", formatBytes(b.Images)))
	}
	if b.Documents > 0 {
		lines = append(lines, fmt.Sprintf("Documents: %s", formatBytes(b.Documents)))
	}
	if b.Other > 0 {
		lines = append(lines, fmt.Sprintf("Other: %s", formatBytes(b.Other)))
	}
	if b.Exports > 0 {
		lines = append(lines, fmt.Sprintf("Export bundles: %s", formatBytes(b.Exports)))
	}
	return lines
}
