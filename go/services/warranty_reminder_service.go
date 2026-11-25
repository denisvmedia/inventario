package services

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/notifications"
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

// prefsForSweep returns a per-sweep preferences cache when the service
// is wired with a notifications.Service, or nil otherwise (tests that
// don't care about opt-out gating leave prefs unset; every recipient
// is then treated as opted-in). The cache is intentionally per-call
// so the worker observes the user's latest toggle flips on the next
// sweep instead of stale-reading from a shared cache forever.
func (s *WarrantyReminderService) prefsForSweep() *notifications.Cache {
	if s.prefs == nil {
		return nil
	}
	return s.prefs.NewCache()
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

	// One preferences cache per sweep — every IsEnabled call inside
	// processOne reads from it instead of hitting SettingsRegistry per
	// (recipient, commodity, threshold). Same admin user fanned out
	// across many commodities = one fetch, not one-per-row. Discarded
	// when this RemindOnce returns so the next sweep sees the user's
	// freshest toggle flips.
	prefsCache := s.prefsForSweep()
	// Admin recipients depend only on the group, so they are resolved once
	// per group per sweep. Every commodity within a year of expiry needs
	// them now that each recipient picks their own lead times.
	recipientsByGroup := map[string][]warrantyRecipient{}

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
		thresholds, recipients, planErr := s.planCommodity(ctx, c, now, prefsCache, recipientsByGroup)
		if planErr != nil {
			stats.Failed++
			slog.Error("warranty reminder failed",
				"commodity_id", c.ID,
				"error", planErr,
			)
			continue
		}
		for _, threshold := range thresholds {
			ok, processErr := s.processOne(ctx, c, threshold, recipients, now, prefsCache)
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

// planCommodity returns the lead times at which c is due a reminder now,
// largest first, and the recipients to consider for them.
//
// Each recipient has their own lead times (#2805), so the commodity is
// checked at the union of them. Each recipient is due only the closest
// lead time they have reached (DueLeadTime), and processOne emails a
// threshold only to the recipients it is due for. The larger thresholds
// still get their idempotency row, without an email. Without recipients,
// or without an email service, the defaults apply so the rows are still
// written. A commodity 30 days from expiry with [60, 30, 7] returns
// [60, 30]: the 60-day row is written silently and the recipient gets the
// 30-day email.
func (s *WarrantyReminderService) planCommodity(
	ctx context.Context,
	c *models.Commodity,
	now time.Time,
	prefsCache *notifications.Cache,
	recipientsByGroup map[string][]warrantyRecipient,
) ([]models.WarrantyReminderThreshold, []warrantyRecipient, error) {
	daysLeft, ok := warrantyDaysLeft(c.WarrantyExpiresAt, now)
	if !ok || daysLeft > models.ReminderMaxLeadDays {
		return nil, nil, nil
	}
	if s.emailSvc == nil {
		return matchedThresholds(daysLeft, models.DefaultWarrantyLeadTimes()), nil, nil
	}
	recipients, err := s.cachedRecipients(ctx, c, recipientsByGroup)
	if err != nil {
		return nil, nil, errxtrace.Wrap("warranty reminder: resolve recipients", err)
	}
	if len(recipients) == 0 {
		return matchedThresholds(daysLeft, models.DefaultWarrantyLeadTimes()), nil, nil
	}
	withLeads := make([]warrantyRecipient, len(recipients))
	var union []int
	for i, r := range recipients {
		r.leadTimes = prefsCache.WarrantyLeadTimes(ctx, r.user)
		r.due, r.hasDue = DueLeadTime(r.leadTimes, daysLeft)
		withLeads[i] = r
		union = append(union, r.leadTimes...)
	}
	return matchedThresholds(daysLeft, union), withLeads, nil
}

// warrantyDaysLeft returns the whole days from today (UTC) to the expiry
// date, and false when there is no date or it has passed. Expired
// warranties are surfaced in the FE; the email cadence does not message
// after the deadline. `now` is normalised to UTC before deriving today's
// date, the same rationale as ComputeWarrantyStatus.
func warrantyDaysLeft(expires models.PDate, now time.Time) (int, bool) {
	if expires == nil || string(*expires) == "" {
		return 0, false
	}
	exp := expires.ToTime()
	if exp.IsZero() {
		return 0, false
	}
	n := now.UTC()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	if exp.Before(today) {
		return 0, false
	}
	return int(exp.Sub(today).Hours() / 24), true
}

// matchedThresholds returns the distinct lead times T with daysLeft <= T,
// largest first. Every one of them gets its (commodity, threshold)
// idempotency row in this sweep, but a recipient is emailed only for the
// closest one they have reached. Sending every matched threshold would
// burst after a worker outage, or when a recipient adds a longer lead
// time: adding 90 would send a "90 days left" email for every item that
// expires within 90 days, tomorrow's included.
func matchedThresholds(daysLeft int, leadTimes []int) []models.WarrantyReminderThreshold {
	var out []models.WarrantyReminderThreshold
	for _, d := range leadTimes {
		t := models.WarrantyReminderThreshold(d)
		if daysLeft <= d && t.IsValid() && !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	slices.Sort(out)
	slices.Reverse(out)
	return out
}

// cachedRecipients returns recipientsForCommodity, reusing the admin list
// already resolved for the commodity's group in this sweep. The creator
// fallback depends on the commodity, so an empty admin list is not cached.
func (s *WarrantyReminderService) cachedRecipients(ctx context.Context, c *models.Commodity, byGroup map[string][]warrantyRecipient) ([]warrantyRecipient, error) {
	if cached, ok := byGroup[c.GroupID]; ok {
		return cached, nil
	}
	recipients, err := s.recipientsForCommodity(ctx, c)
	if err != nil {
		return nil, err
	}
	if len(recipients) > 0 && recipients[0].fromMembership {
		byGroup[c.GroupID] = recipients
	}
	return recipients, nil
}

// dueFor reports whether t is the lead time the recipient is due now: the
// closest one they have reached.
func (r warrantyRecipient) dueFor(t models.WarrantyReminderThreshold) bool {
	return r.hasDue && r.due == int(t)
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
//     the CreateOnce race OR no recipients matched OR no recipient is
//     due this threshold OR a closer one was already sent
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
func (s *WarrantyReminderService) processOne(ctx context.Context, c *models.Commodity, threshold models.WarrantyReminderThreshold, recipients []warrantyRecipient, now time.Time, prefsCache *notifications.Cache) (bool, error) {
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

	superseded, err := s.supersededByCloserSend(ctx, c, threshold, recipients, now)
	if err != nil {
		return false, err
	}
	if superseded {
		if _, err := s.commitReminderRow(ctx, c, threshold, now); err != nil {
			return false, errxtrace.Wrap("warranty reminder: insert idempotency row (superseded)", err)
		}
		return false, nil
	}

	attempted, enqueueErrs, firstEnqueueErr := s.fanOutToRecipients(ctx, c, threshold, recipients, prefsCache)
	if attempted > 0 && enqueueErrs == attempted {
		// Every recipient enqueue failed. Don't commit the idempotency
		// row — let the next sweep retry. Once any enqueue succeeds the
		// async email service handles its own per-job retry inside the
		// queue worker pool.
		return false, errxtrace.Wrap("warranty reminder: all enqueues failed", firstEnqueueErr)
	}
	if attempted == 0 {
		// Every recipient opted out of warranty notifications, or none
		// is due this threshold because each has reached a closer one.
		// Persist the idempotency row so the worker doesn't sweep them
		// again next tick, but DON'T flag this row as "sent" — the
		// SentByThreshold counter (Prometheus
		// `inventario_warranty_reminders_sent_total`) is for actually-
		// emitted reminders and would otherwise drift upwards every
		// time a fully-opted-out cohort matched a threshold.
		if _, err := s.commitReminderRow(ctx, c, threshold, now); err != nil {
			return false, errxtrace.Wrap("warranty reminder: insert idempotency row (all opted out)", err)
		}
		return false, nil
	}
	ok, err := s.commitReminderRow(ctx, c, threshold, now)
	if err != nil {
		return false, errxtrace.Wrap("warranty reminder: insert idempotency row", err)
	}
	return ok, nil
}

// fanOutToRecipients enqueues the warranty email for every recipient the
// threshold is due for who has not opted out. It returns how many enqueues
// were attempted, how many failed and the first error, for processOne to
// pick between its terminal states. Split out of processOne to keep it
// under the gocognit ceiling.
func (s *WarrantyReminderService) fanOutToRecipients(
	ctx context.Context,
	c *models.Commodity,
	threshold models.WarrantyReminderThreshold,
	recipients []warrantyRecipient,
	prefsCache *notifications.Cache,
) (attempted, enqueueErrs int, firstEnqueueErr error) {
	expiry := string(*c.WarrantyExpiresAt)
	url := s.buildCommodityURL(ctx, c)
	for _, r := range recipients {
		if !r.dueFor(threshold) {
			// This recipient did not choose this lead time, or has
			// already reached a closer one and gets only that. They are
			// not counted as attempted; when nobody is, the row is
			// written without an email below.
			continue
		}
		// Per-recipient opt-out gate. Skipped recipients still count
		// toward "this commodity/threshold was processed" — the
		// idempotency row gets written below so we don't sweep them
		// again on every tick. When prefsCache is nil (legacy / test
		// path), every recipient is treated as opted-in. Using the
		// cache means the same admin user fanning out across many
		// commodities hits the SettingsRegistry exactly once per
		// sweep, not once per row.
		//
		// IsEnabledForGroup (issue #1648) consults the per-group
		// override row first and falls back to the user-global pref
		// from #1373. When no per-group row exists or the per-group
		// registry isn't wired (test path), the answer matches the
		// pre-#1648 IsEnabled.
		if prefsCache != nil && !prefsCache.IsEnabledForGroup(ctx, r.user, c.TenantID, c.GroupID, notifications.CategoryWarrantyExpiry, notifications.ChannelEmail) {
			slog.Debug("warranty reminder: recipient opted out",
				"commodity_id", c.ID,
				"group_id", c.GroupID,
				"to", r.email,
			)
			continue
		}
		attempted++
		sendErr := s.emailSvc.SendWarrantyReminderEmail(withReminderLanguage(ctx, prefsCache, r.user), r.email, r.name, c.Name, expiry, url, int(threshold))
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
	return attempted, enqueueErrs, firstEnqueueErr
}

// supersededByCloserSend reports whether a reminder at a closer lead time
// than threshold has already gone out for c, while threshold is due for
// some recipient. That happens when a recipient raises a lead time: with
// 60 sent and the lead times changed to [90, 30, 7], an item 45 days out
// is due the 90-day reminder, but its owner already had the 60-day one.
// A larger lead time is never sent after a smaller one, the same rule as
// item reminders.
func (s *WarrantyReminderService) supersededByCloserSend(ctx context.Context, c *models.Commodity, threshold models.WarrantyReminderThreshold, recipients []warrantyRecipient, now time.Time) (bool, error) {
	if !slices.ContainsFunc(recipients, func(r warrantyRecipient) bool { return r.dueFor(threshold) }) {
		return false, nil
	}
	daysLeft, ok := warrantyDaysLeft(c.WarrantyExpiresAt, now)
	if !ok {
		return false, nil
	}
	closer, err := s.factorySet.WarrantyReminderRegistry.HasSentInRange(ctx, c.ID, daysLeft, int(threshold))
	if err != nil {
		return false, errxtrace.Wrap("warranty reminder: check closer reminders", err)
	}
	return closer, nil
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
	// leadTimes are the recipient's warranty lead times, set by
	// planCommodity from their notification settings.
	leadTimes []int
	// due is the closest lead time the recipient has reached, the only
	// one they are emailed for; hasDue is false when none is reached.
	due    int
	hasDue bool
	// fromMembership is true for group admins and false for the
	// commodity-creator fallback.
	fromMembership bool
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
		out = append(out, warrantyRecipient{email: user.Email, name: user.Name, user: user, fromMembership: true})
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
