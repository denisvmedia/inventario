package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services/notifications"
)

// WarrantyReminderService runs one warranty-reminder sweep at a time:
// it scans every commodity (in service mode, across tenants), figures
// out which 60 / 30 / 7-day reminder thresholds it crosses against the
// configured `now`, and for each newly crossed threshold:
//
//  1. ensures an idempotency row exists in warranty_reminders (atomic
//     INSERT … ON CONFLICT DO NOTHING; the loser of a race silently
//     skips);
//  2. enqueues a SendWarrantyReminderEmail through the AsyncEmailService
//     to every group admin/owner who can act on the warranty.
//
// The service is deliberately stateless — `RemindOnce` takes the clock
// as an argument so unit tests can pin "now" to a fixed value and
// assert the cadence boundaries without time-injection scaffolding.
type WarrantyReminderService struct {
	factorySet *registry.FactorySet
	emailSvc   EmailService
	// commodityURLBuilder builds the deep-link printed in the reminder
	// email. Optional — when nil, the email omits the link block. The
	// signature takes (groupSlug, commodityID) so a non-default base
	// URL can be wired in via the bootstrap layer.
	commodityURLBuilder func(groupSlug, commodityID string) string
	// prefs is the per-user notification preferences service. Optional
	// — when nil, the IsEnabled check is skipped (legacy / test
	// behaviour: every resolved recipient receives the reminder). Set
	// via WithPreferences from the bootstrap layer.
	prefs *notifications.Service
}

// NewWarrantyReminderService constructs the service. emailSvc may be
// nil in tests that only assert the idempotency-row side; in production
// callers always pass a non-nil EmailService.
func NewWarrantyReminderService(factorySet *registry.FactorySet, emailSvc EmailService, urlBuilder func(groupSlug, commodityID string) string) *WarrantyReminderService {
	return &WarrantyReminderService{
		factorySet:          factorySet,
		emailSvc:            emailSvc,
		commodityURLBuilder: urlBuilder,
	}
}

// WithPreferences attaches a notifications.Service so the worker gates
// each recipient on their `notifications.warranty_expiry` toggle (×
// `notifications.channel.email`). Returns the same service for fluent
// chaining at the bootstrap site. Tests that don't care about the
// preference path keep using the bare constructor — IsEnabled gating
// is then a no-op.
func (s *WarrantyReminderService) WithPreferences(prefs *notifications.Service) *WarrantyReminderService {
	s.prefs = prefs
	return s
}

// WarrantyReminderStats summarises the outcome of one
// WarrantyReminderService.RemindOnce sweep. SentByThreshold counts
// the number of (commodity, threshold) idempotency rows newly written
// in this tick, partitioned by the WarrantyReminderThreshold value
// (60/30/7) — that is what the worker emits as the
// `inventario_warranty_reminders_sent_total{threshold=…}` Prometheus
// counter. Failed is the cross-threshold count of commodities whose
// threshold processing failed (typically transient queue outages);
// the next sweep retries.
type WarrantyReminderStats struct {
	SentByThreshold map[models.WarrantyReminderThreshold]int
	Failed          int
}

// Sent returns the total number of newly-inserted idempotency rows
// across every threshold. Convenience for callers that don't need the
// per-threshold breakdown.
func (s WarrantyReminderStats) Sent() int {
	total := 0
	for _, v := range s.SentByThreshold {
		total += v
	}
	return total
}

// RemindOnce runs one sweep pinned to `now`. Returns a
// WarrantyReminderStats with the per-threshold count of newly-written
// idempotency rows + the failed counter. A non-nil error is only
// returned when the initial commodity listing itself fails.
func (s *WarrantyReminderService) RemindOnce(ctx context.Context, now time.Time) (WarrantyReminderStats, error) {
	stats := WarrantyReminderStats{SentByThreshold: map[models.WarrantyReminderThreshold]int{}}
	if s.factorySet == nil {
		return stats, errxtrace.Wrap("warranty reminder service: factorySet is required", registry.ErrFieldRequired)
	}
	commodityReg := s.factorySet.CommodityRegistryFactory.CreateServiceRegistry()
	commodities, err := commodityReg.List(ctx)
	if err != nil {
		return stats, errxtrace.Wrap("warranty reminder: list commodities", err)
	}

	for _, c := range commodities {
		if c == nil || c.WarrantyExpiresAt == nil || string(*c.WarrantyExpiresAt) == "" {
			continue
		}
		// #1554: defence-in-depth — a Count > 1 row should not have a
		// warranty in the first place (model validation rejects it
		// from the FE / API path), but legacy rows that pre-date the
		// constraint may still carry one. Skip them so we don't email
		// a "your bundle's warranty is expiring" reminder that the
		// user can't act on without splitting the row first.
		if c.Count > 1 {
			slog.Warn("warranty reminder: skipping commodity with count > 1",
				"commodity_id", c.ID,
				"count", c.Count,
			)
			continue
		}
		for _, threshold := range matchedThresholds(c.WarrantyExpiresAt, now) {
			ok, processErr := s.processOne(ctx, c, threshold, now)
			if processErr != nil {
				stats.Failed++
				slog.Error("warranty reminder failed",
					"commodity_id", c.ID,
					"threshold_days", threshold,
					"error", processErr,
				)
				continue
			}
			if ok {
				stats.SentByThreshold[threshold]++
			}
		}
	}
	return stats, nil
}

// matchedThresholds returns every WarrantyReminderThreshold whose
// "days remaining" window contains the commodity at the given clock.
// Concretely: a commodity expiring in N days matches every threshold
// T where N <= T — so a row 30 days from expiry returns [60, 30],
// and the worker emits one email per matched threshold. The
// `WarrantyReminderRegistry.CreateOnce` idempotency row keeps each
// (commodity, threshold) tuple emitting at most once across all
// future ticks; that is what prevents duplicates, NOT this function.
//
// Returning every matched threshold (rather than only the tightest
// one) is deliberate: after a worker outage we still want to send the
// 60-day reminder for an item that is now 30 days out, because the
// 60-day row was never inserted and the user has missed that signal.
//
// `now` is normalised to UTC before deriving today's date — same
// rationale as ComputeWarrantyStatus. Two callers with the same
// (expires, now) pair always get the same threshold list, in
// canonical largest → smallest order.
func matchedThresholds(expires models.PDate, now time.Time) []models.WarrantyReminderThreshold {
	if expires == nil || string(*expires) == "" {
		return nil
	}
	exp := expires.ToTime()
	if exp.IsZero() {
		return nil
	}
	n := now.UTC()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	if exp.Before(today) {
		// Expired warranties are surfaced in the FE; the email cadence
		// does not double-message after the deadline.
		return nil
	}
	daysLeft := int(exp.Sub(today).Hours() / 24)
	var out []models.WarrantyReminderThreshold
	for _, t := range models.WarrantyReminderThresholds {
		if daysLeft <= int(t) {
			out = append(out, t)
		}
	}
	return out
}

// processOne handles one (commodity, threshold) pair across the
// commit + send pipeline. Order is:
//
//  1. Cheap HasSent check — if a previous tick already wrote the row
//     for this tuple, skip everything (no recipient lookup, no email).
//     This is the idempotency contract that callers rely on.
//  2. Resolve recipients (fallible — registry round-trip).
//  3. Try every recipient's enqueue (fallible — queue may be down).
//  4. Only commit the idempotency row if at least one enqueue
//     succeeded. The CreateOnce uniqueness guard still protects
//     against a race against another worker between (1) and (4).
//
// The reason (4) lives at the end rather than before (2)/(3) is the
// review feedback on this PR: writing the row before send permanently
// skipped retries when the queue was transiently down. With the
// row-after-send order, a failed enqueue leaves no row and the next
// sweep retries. The trade-off — a worker crash after enqueue but
// before commit — replays the reminder on the next sweep, which we
// accept (a duplicate email beats silent loss).
//
// Returns:
//   - (true, nil)  — this call won the row insert AND at least one
//     enqueue succeeded.
//   - (false, nil) — already-sent (HasSent hit) OR another worker won
//     the CreateOnce race OR no recipients matched
//     (row still committed so the worker stops
//     re-evaluating).
//   - (false, err) — recipient lookup failed OR every enqueue failed;
//     caller increments `failed` and the next sweep
//     retries.
//
// `now` is passed in (rather than re-reading the wall clock) so the
// SentAt timestamp matches the sweep clock — important for tests that
// pin time and for audit consistency across rows produced in the same
// tick.
func (s *WarrantyReminderService) processOne(ctx context.Context, c *models.Commodity, threshold models.WarrantyReminderThreshold, now time.Time) (bool, error) {
	already, err := s.factorySet.WarrantyReminderRegistry.HasSent(ctx, c.ID, int(threshold))
	if err != nil {
		return false, errxtrace.Wrap("warranty reminder: check existing row", err)
	}
	if already {
		return false, nil
	}

	// Stub-mode short-circuit: tests/dev environments without an email
	// service still want the idempotency row written so the sweep
	// counter is meaningful. Skip the recipient lookup entirely (it
	// would be a no-op) and write the row directly.
	if s.emailSvc == nil {
		ok, commitErr := s.commitReminderRow(ctx, c, threshold, now)
		if commitErr != nil {
			return false, errxtrace.Wrap("warranty reminder: insert idempotency row", commitErr)
		}
		if ok {
			slog.Info("warranty reminder row inserted (no email service configured)",
				"commodity_id", c.ID,
				"threshold_days", int(threshold),
			)
		}
		return ok, nil
	}

	recipients, err := s.recipientsForCommodity(ctx, c)
	if err != nil {
		return false, errxtrace.Wrap("warranty reminder: resolve recipients", err)
	}
	if len(recipients) == 0 {
		// No one to email — write the row anyway so the worker stops
		// re-considering this (commodity, threshold) on every tick.
		ok, commitErr := s.commitReminderRow(ctx, c, threshold, now)
		if commitErr != nil {
			return false, errxtrace.Wrap("warranty reminder: insert idempotency row (no recipients)", commitErr)
		}
		if ok {
			slog.Warn("warranty reminder: no recipients found for commodity",
				"commodity_id", c.ID,
				"group_id", c.GroupID,
			)
		}
		return ok, nil
	}

	expiry := string(*c.WarrantyExpiresAt)
	url := s.buildCommodityURL(ctx, c)
	enqueueErrs := 0
	attempted := 0
	var firstEnqueueErr error
	for _, r := range recipients {
		// Per-recipient opt-out gate. Skipped recipients still count
		// toward "this commodity/threshold was processed" — the
		// idempotency row gets written below so we don't sweep them
		// again on every tick. When prefs is nil (legacy / test path),
		// every recipient is treated as opted-in.
		if s.prefs != nil && !s.prefs.IsEnabled(ctx, r.user, notifications.CategoryWarrantyExpiry, notifications.ChannelEmail) {
			slog.Debug("warranty reminder: recipient opted out",
				"commodity_id", c.ID,
				"to", r.email,
			)
			continue
		}
		attempted++
		sendErr := s.emailSvc.SendWarrantyReminderEmail(ctx, r.email, r.name, c.Name, expiry, url, int(threshold))
		if sendErr != nil {
			enqueueErrs++
			if firstEnqueueErr == nil {
				firstEnqueueErr = sendErr
			}
			slog.Error("warranty reminder: enqueue failed",
				"commodity_id", c.ID,
				"to", r.email,
				"error", sendErr,
			)
		}
	}
	if attempted > 0 && enqueueErrs == attempted {
		// Every recipient enqueue failed. Don't commit the idempotency
		// row — let the next sweep retry. Once any enqueue succeeds the
		// async email service handles its own per-job retry inside the
		// queue worker pool.
		return false, errxtrace.Wrap("warranty reminder: all enqueues failed", firstEnqueueErr)
	}
	ok, err := s.commitReminderRow(ctx, c, threshold, now)
	if err != nil {
		return false, errxtrace.Wrap("warranty reminder: insert idempotency row", err)
	}
	return ok, nil
}

// commitReminderRow writes the (commodity, threshold) idempotency row
// stamped with the sweep's `now`. Returns (true, nil) iff this call
// was the winner of the unique-constraint race; (false, nil) for the
// loser (a concurrent sweep already wrote the row).
func (s *WarrantyReminderService) commitReminderRow(ctx context.Context, c *models.Commodity, threshold models.WarrantyReminderThreshold, now time.Time) (bool, error) {
	reminder := models.WarrantyReminder{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        c.TenantID,
			GroupID:         c.GroupID,
			CreatedByUserID: c.CreatedByUserID,
		},
		CommodityID:   c.ID,
		ThresholdDays: int(threshold),
		SentAt:        now,
	}
	return s.factorySet.WarrantyReminderRegistry.CreateOnce(ctx, reminder)
}

type warrantyRecipient struct {
	email string
	name  string
	// user is the full User the email is addressed to. It is carried
	// alongside the email/name because the per-recipient preference
	// check (see prefs.IsEnabled in processOne) needs the user_id +
	// tenant_id to materialise a user-scoped SettingsRegistry. Always
	// non-nil for recipients that came out of recipientsForCommodity.
	user *models.User
}

// recipientsForCommodity returns every group admin/owner that should
// receive a reminder for this commodity. Falls back to the
// CreatedByUserID when no membership records resolve, so a single-user
// install (memberships table empty) still gets a notification.
func (s *WarrantyReminderService) recipientsForCommodity(ctx context.Context, c *models.Commodity) ([]warrantyRecipient, error) {
	out := make([]warrantyRecipient, 0, 4)
	if s.factorySet.GroupMembershipRegistry != nil {
		members, err := s.factorySet.GroupMembershipRegistry.ListByGroup(ctx, c.GroupID)
		if err != nil {
			return nil, err
		}
		out = s.collectAdminRecipients(ctx, c.GroupID, members)
	}

	if len(out) == 0 && c.CreatedByUserID != "" {
		user, err := s.factorySet.UserRegistry.Get(ctx, c.CreatedByUserID)
		if err == nil && user != nil && strings.TrimSpace(user.Email) != "" {
			out = append(out, warrantyRecipient{email: user.Email, name: user.Name, user: user})
		}
	}
	return out, nil
}

// isWarrantyRecipient reports whether the given membership role should
// receive warranty reminders. Admins do; plain users do not — they get
// the FE pill, but the email goes to the people who can act on it.
func isWarrantyRecipient(role models.GroupRole) bool {
	return role == models.GroupRoleAdmin
}

// collectAdminRecipients filters memberships to admin-role rows,
// resolves each to a User, and returns the deduplicated recipient
// list. Lookups that 404 (member id rotted out from under a stale
// membership row) are logged and skipped — the rest of the group
// still gets the reminder. Split out of recipientsForCommodity to
// keep the outer function under the nestif threshold.
func (s *WarrantyReminderService) collectAdminRecipients(
	ctx context.Context,
	groupID string,
	members []*models.GroupMembership,
) []warrantyRecipient {
	out := make([]warrantyRecipient, 0, 4)
	seen := make(map[string]struct{}, 4)
	for _, m := range members {
		if m == nil || !isWarrantyRecipient(m.Role) {
			// Skip viewer-only memberships — owners + admins are the
			// canonical recipients for renewal-actionable reminders.
			continue
		}
		user, err := s.factorySet.UserRegistry.Get(ctx, m.MemberUserID)
		if err != nil {
			slog.Warn("warranty reminder: skip member with missing user",
				"user_id", m.MemberUserID,
				"group_id", groupID,
				"error", err,
			)
			continue
		}
		if user == nil || strings.TrimSpace(user.Email) == "" {
			continue
		}
		if _, ok := seen[user.Email]; ok {
			continue
		}
		seen[user.Email] = struct{}{}
		out = append(out, warrantyRecipient{email: user.Email, name: user.Name, user: user})
	}
	return out
}

// buildCommodityURL composes the deep-link printed in the reminder.
// Returns "" when the resolver is unset OR the commodity's parent
// group can't be resolved (e.g. because the group was hard-deleted
// between scan and tick).
func (s *WarrantyReminderService) buildCommodityURL(ctx context.Context, c *models.Commodity) string {
	if s.commodityURLBuilder == nil {
		return ""
	}
	if s.factorySet.LocationGroupRegistry == nil {
		return ""
	}
	group, err := s.factorySet.LocationGroupRegistry.Get(ctx, c.GroupID)
	if err != nil || group == nil {
		return ""
	}
	return s.commodityURLBuilder(group.Slug, c.ID)
}
