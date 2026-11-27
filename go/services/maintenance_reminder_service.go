package services

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/notifications"
)

// MaintenanceReminderService runs one maintenance-reminder sweep at a
// time (#1368): it scans every enabled maintenance schedule (in service
// mode, across tenants), figures out which 14 / 7 / 1-day reminder
// thresholds it crosses against the configured `now` plus the
// overdue-once threshold, and for each newly crossed threshold:
//
//  1. checks the idempotency row in maintenance_reminders (HasSent);
//  2. enqueues a SendMaintenanceReminderEmail through the
//     AsyncEmailService to every group admin/owner who can act on the
//     schedule (per-recipient opt-out gated on the
//     notifications.maintenance_reminder toggle);
//  3. commits the idempotency row after at least one enqueue succeeds.
//
// Mirrors WarrantyReminderService one-to-one — the only differences
// are the threshold semantics (matchedMaintenanceThresholds adds the
// overdue sentinel) and the email shape.
type MaintenanceReminderService struct {
	factorySet *registry.FactorySet
	emailSvc   EmailService
	// commodityURLBuilder builds the deep-link printed in the reminder
	// email. Optional — when nil, the email omits the link block.
	commodityURLBuilder func(groupSlug, commodityID string) string
	// prefs is the per-user notification preferences service. Optional
	// — when nil, the opt-out check is skipped (legacy / test path).
	prefs *notifications.Service
}

func NewMaintenanceReminderService(factorySet *registry.FactorySet, emailSvc EmailService, urlBuilder func(groupSlug, commodityID string) string) *MaintenanceReminderService {
	return &MaintenanceReminderService{
		factorySet:          factorySet,
		emailSvc:            emailSvc,
		commodityURLBuilder: urlBuilder,
	}
}

// WithPreferences attaches a notifications.Service so the worker gates
// each recipient on their `notifications.maintenance_reminder` toggle.
func (s *MaintenanceReminderService) WithPreferences(prefs *notifications.Service) *MaintenanceReminderService {
	s.prefs = prefs
	return s
}

// MaintenanceReminderStats summarises the outcome of one sweep.
type MaintenanceReminderStats struct {
	SentByThreshold map[models.MaintenanceReminderThreshold]int
	Failed          int
}

// Sent returns the total number of newly-inserted idempotency rows
// across every threshold.
func (s MaintenanceReminderStats) Sent() int {
	total := 0
	for _, v := range s.SentByThreshold {
		total += v
	}
	return total
}

// prefsForSweep returns a per-sweep preferences cache when the service
// is wired with a notifications.Service, or nil otherwise.
func (s *MaintenanceReminderService) prefsForSweep() *notifications.Cache {
	if s.prefs == nil {
		return nil
	}
	return s.prefs.NewCache()
}

// RemindOnce runs one sweep pinned to `now`.
func (s *MaintenanceReminderService) RemindOnce(ctx context.Context, now time.Time) (MaintenanceReminderStats, error) {
	stats := MaintenanceReminderStats{SentByThreshold: map[models.MaintenanceReminderThreshold]int{}}
	if s.factorySet == nil {
		return stats, errxtrace.Wrap("maintenance reminder service: factorySet is required", registry.ErrFieldRequired)
	}
	scheduleReg := s.factorySet.MaintenanceScheduleRegistryFactory.CreateServiceRegistry()
	schedules, err := scheduleReg.List(ctx)
	if err != nil {
		return stats, errxtrace.Wrap("maintenance reminder: list schedules", err)
	}

	prefsCache := s.prefsForSweep()
	sweep := newMaintenanceSweep()
	gate := newReminderGate(s.factorySet)

	for _, m := range schedules {
		if m == nil || !m.Enabled {
			continue
		}
		skipped, skipErr := gate.skips(ctx, m.TenantID, m.GroupID, models.GroupSectionMaintenance)
		if skipErr != nil {
			stats.Failed++
			slog.ErrorContext(ctx, "maintenance reminder failed",
				"schedule_id", m.ID,
				"error", skipErr,
			)
			continue
		}
		if skipped {
			// The tenant is not active or the group hides the section:
			// no row, so the reminder goes out once that changes.
			continue
		}
		target, planErr := s.planSchedule(ctx, m, now, prefsCache, sweep)
		if planErr != nil {
			stats.Failed++
			slog.ErrorContext(ctx, "maintenance reminder failed",
				"schedule_id", m.ID,
				"error", planErr,
			)
			continue
		}
		for _, threshold := range target.thresholds {
			ok, processErr := s.processOne(ctx, m, threshold, target, now, prefsCache)
			if processErr != nil {
				stats.Failed++
				slog.ErrorContext(ctx, "maintenance reminder failed",
					"schedule_id", m.ID,
					"threshold", threshold,
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

// maintenanceTarget is what one schedule needs for this sweep: the lead
// times it is due at, its commodity and the recipients to consider.
type maintenanceTarget struct {
	thresholds []models.MaintenanceReminderThreshold
	// commodity is nil when the schedule's commodity is gone or when no
	// email service is configured; processOne then only writes rows.
	commodity  *models.Commodity
	recipients []maintenanceRecipient
	// daysLeft is the days until the due date; 0 for an overdue schedule.
	daysLeft int
}

// planSchedule works out maintenanceTarget for m. A past-due schedule gets
// the overdue reminder alone, for every recipient: once the date has passed
// the lead times no longer apply. Otherwise each recipient has their own
// lead times (#2805) and the schedule is checked at the union of them. A
// schedule N days from due matches every lead time T where N <= T, so with
// [14, 7, 1] a row 7 days out returns [14, 7]. Every matched threshold gets
// its idempotency row, but a recipient is emailed only for the closest lead
// time they have reached (DueLeadTime), here 7, so a missed sweep or a
// newly added longer lead time does not send a burst. The rows keep each
// (schedule, threshold) to one send until the next MarkDone clears them.
func (s *MaintenanceReminderService) planSchedule(
	ctx context.Context,
	m *models.MaintenanceSchedule,
	now time.Time,
	prefsCache *notifications.Cache,
	sweep *maintenanceSweep,
) (maintenanceTarget, error) {
	var target maintenanceTarget
	if string(m.NextDueAt) == "" {
		return target, nil
	}
	overdue := m.IsOverdue(now)
	daysLeft := 0
	if overdue {
		// The overdue reminder is the only one left for this cycle, so
		// once it is sent there is nothing to resolve.
		sent, err := s.factorySet.MaintenanceReminderRegistry.HasSent(ctx, m.ID, int(models.MaintenanceReminderOverdue))
		if err != nil {
			return target, errxtrace.Wrap("maintenance reminder: check existing row", err)
		}
		if sent {
			return target, nil
		}
	} else {
		due := m.NextDueAt.ToTime()
		if due.IsZero() {
			return target, nil
		}
		n := now.UTC()
		today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
		daysLeft = int(due.Sub(today).Hours() / 24)
		if daysLeft > models.ReminderMaxLeadDays {
			return target, nil
		}
	}
	leadTimes := models.DefaultMaintenanceLeadTimes()
	if s.emailSvc != nil {
		if err := s.resolveTarget(ctx, m, prefsCache, sweep, &target); err != nil {
			return target, err
		}
		var union []int
		for _, r := range target.recipients {
			union = append(union, r.leadTimes...)
		}
		if len(union) > 0 {
			leadTimes = union
		}
	}
	if overdue {
		target.thresholds = []models.MaintenanceReminderThreshold{models.MaintenanceReminderOverdue}
		return target, nil
	}
	target.daysLeft = daysLeft
	for i := range target.recipients {
		r := &target.recipients[i]
		r.due, r.hasDue = DueLeadTime(r.leadTimes, daysLeft)
	}
	for _, d := range leadTimes {
		t := models.MaintenanceReminderThreshold(d)
		if daysLeft <= d && t != models.MaintenanceReminderOverdue && t.IsValid() && !slices.Contains(target.thresholds, t) {
			target.thresholds = append(target.thresholds, t)
		}
	}
	slices.Sort(target.thresholds)
	slices.Reverse(target.thresholds)
	return target, nil
}

// resolveTarget fills target's commodity and recipients, each recipient
// with their maintenance lead times. A missing commodity leaves both empty.
func (s *MaintenanceReminderService) resolveTarget(
	ctx context.Context,
	m *models.MaintenanceSchedule,
	prefsCache *notifications.Cache,
	sweep *maintenanceSweep,
	target *maintenanceTarget,
) error {
	commodity, err := s.cachedCommodity(ctx, m.CommodityID, sweep)
	if err != nil {
		return errxtrace.Wrap("maintenance reminder: lookup commodity", err)
	}
	target.commodity = commodity
	if commodity == nil {
		return nil
	}
	recipients, err := s.cachedRecipients(ctx, commodity, sweep.recipientsByGroup)
	if err != nil {
		return errxtrace.Wrap("maintenance reminder: resolve recipients", err)
	}
	target.recipients = make([]maintenanceRecipient, len(recipients))
	for i, r := range recipients {
		r.leadTimes = prefsCache.MaintenanceLeadTimes(ctx, r.user)
		target.recipients[i] = r
	}
	return nil
}

// maintenanceSweep holds what one sweep looks up once and reuses: the
// commodities behind the schedules (several schedules often share one) and
// the admin recipients of each group, which depend only on the group.
type maintenanceSweep struct {
	commodities       map[string]*models.Commodity
	recipientsByGroup map[string][]maintenanceRecipient
}

func newMaintenanceSweep() *maintenanceSweep {
	return &maintenanceSweep{
		commodities:       map[string]*models.Commodity{},
		recipientsByGroup: map[string][]maintenanceRecipient{},
	}
}

// cachedCommodity returns lookupCommodity, reusing a commodity already
// looked up in this sweep. A missing commodity is cached as nil.
func (s *MaintenanceReminderService) cachedCommodity(ctx context.Context, id string, sweep *maintenanceSweep) (*models.Commodity, error) {
	if c, ok := sweep.commodities[id]; ok {
		return c, nil
	}
	c, err := s.lookupCommodity(ctx, id)
	if err != nil {
		return nil, err
	}
	sweep.commodities[id] = c
	return c, nil
}

// cachedRecipients returns recipientsForCommodity, reusing the admin list
// already resolved for the commodity's group in this sweep. The creator
// fallback depends on the commodity, so it is not cached.
func (s *MaintenanceReminderService) cachedRecipients(ctx context.Context, c *models.Commodity, byGroup map[string][]maintenanceRecipient) ([]maintenanceRecipient, error) {
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

// processOne handles one (schedule, threshold) pair across the
// commit + send pipeline. Returns (true, nil) ONLY when the worker
// actually enqueued at least one email — the four "commit the row
// but no email went out" branches (stub mode, missing commodity,
// no recipients, fully-opted-out cohort) deliberately return
// (false, nil) so the SentByThreshold counter never reports a
// phantom send. The fan-out + enqueue loop is split into
// fanOutToRecipients to keep this function under the gocyclo
// ceiling; the commit-then-skip helper centralises the four "no
// email" early returns. Mirrors WarrantyReminderService.processOne;
// see the comments there for the idempotency / retry / opt-out
// reasoning.
func (s *MaintenanceReminderService) processOne(ctx context.Context, m *models.MaintenanceSchedule, threshold models.MaintenanceReminderThreshold, target maintenanceTarget, now time.Time, prefsCache *notifications.Cache) (bool, error) {
	already, err := s.factorySet.MaintenanceReminderRegistry.HasSent(ctx, m.ID, int(threshold))
	if err != nil {
		return false, errxtrace.Wrap("maintenance reminder: check existing row", err)
	}
	if already {
		return false, nil
	}

	if s.emailSvc == nil {
		// Stub mode (tests, dev). Persist the idempotency row so the
		// worker stops re-evaluating, but DO NOT report this as a
		// sent reminder — there was no email.
		return s.commitWithoutSend(ctx, m, threshold, now, "no email service configured", nil)
	}

	commodity := target.commodity
	if commodity == nil {
		// The item went to Trash after the schedule list was read. Its
		// reminders wait: no idempotency row, so a restore brings them
		// back.
		if s.commodityTrashed(ctx, m.CommodityID) {
			return false, nil
		}
		// Schedule references a commodity that's gone (cross-tenant
		// drift / mid-purge). Write the row so we stop revisiting,
		// but do NOT count this as a sent reminder.
		return s.commitWithoutSend(ctx, m, threshold, now, "commodity missing", nil)
	}

	recipients := target.recipients
	if len(recipients) == 0 {
		// No one to email. Persist the row to stop re-evaluating but
		// don't inflate SentByThreshold — see #1368 / CR feedback.
		return s.commitWithoutSend(ctx, m, threshold, now, "no recipients", commodity)
	}

	superseded, err := s.supersededByCloserSend(ctx, m, threshold, target)
	if err != nil {
		return false, err
	}
	if superseded {
		return s.commitWithoutSend(ctx, m, threshold, now, "closer reminder already sent", commodity)
	}

	attempted, enqueueErrs, firstEnqueueErr := s.fanOutToRecipients(ctx, m, threshold, commodity, recipients, prefsCache)
	if attempted > 0 && enqueueErrs == attempted {
		return false, errxtrace.Wrap("maintenance reminder: all enqueues failed", firstEnqueueErr)
	}
	if attempted == 0 {
		// Every recipient opted out, or none is due this threshold
		// because each has reached a closer one. Persist the row so we
		// don't sweep this tuple every tick, but report (false, nil) —
		// no email was enqueued so SentByThreshold must not move.
		return s.commitWithoutSend(ctx, m, threshold, now, "all opted out", commodity)
	}
	ok, err := s.commitReminderRow(ctx, m, threshold, now, false)
	if err != nil {
		return false, errxtrace.Wrap("maintenance reminder: insert idempotency row", err)
	}
	return ok, nil
}

// supersededByCloserSend reports whether a reminder at a closer lead time
// than threshold has already gone out for m in this cycle, while threshold
// is due for some recipient. See the warranty service: a larger lead time
// is never sent after a smaller one, and a silent row does not count. The
// overdue reminder is never superseded.
func (s *MaintenanceReminderService) supersededByCloserSend(ctx context.Context, m *models.MaintenanceSchedule, threshold models.MaintenanceReminderThreshold, target maintenanceTarget) (bool, error) {
	if threshold == models.MaintenanceReminderOverdue {
		return false, nil
	}
	due := func(r maintenanceRecipient) bool { return r.hasDue && r.due == int(threshold) }
	if !slices.ContainsFunc(target.recipients, due) {
		return false, nil
	}
	// The overdue row (threshold 0) belongs to a past due date, not to a
	// closer lead time.
	closer, err := s.factorySet.MaintenanceReminderRegistry.HasSentInRange(ctx, m.ID, max(target.daysLeft, 1), int(threshold))
	if err != nil {
		return false, errxtrace.Wrap("maintenance reminder: check closer reminders", err)
	}
	return closer, nil
}

// commitWithoutSend handles the four "commit the row, no email
// fires" early-return branches of processOne (stub mode, missing
// commodity, no recipients, fully-opted-out cohort). It writes the
// idempotency row, marked silent, so the worker stops re-evaluating the
// (schedule, threshold) tuple without the row holding back a farther
// lead time later; logs a per-branch breadcrumb; and returns
// (false, nil) so the SentByThreshold counter doesn't move.
// Extracted from processOne to keep the parent under gocyclo's
// ceiling and put the per-branch wording in one place. `reason` is
// the short branch label used in the slog message; `commodity` is
// optional context (nil for the stub / commodity-missing paths
// since the commodity is unavailable there).
func (s *MaintenanceReminderService) commitWithoutSend(ctx context.Context, m *models.MaintenanceSchedule, threshold models.MaintenanceReminderThreshold, now time.Time, reason string, commodity *models.Commodity) (bool, error) {
	ok, commitErr := s.commitReminderRow(ctx, m, threshold, now, true)
	if commitErr != nil {
		return false, errxtrace.Wrap("maintenance reminder: insert idempotency row ("+reason+")", commitErr)
	}
	if ok {
		attrs := []any{
			"schedule_id", m.ID,
			"threshold", int(threshold),
			"reason", reason,
		}
		if commodity != nil {
			attrs = append(attrs, "commodity_id", commodity.ID, "group_id", commodity.GroupID)
		}
		slog.InfoContext(ctx, "maintenance reminder row inserted without sending email", attrs...)
	}
	return false, nil
}

// fanOutToRecipients enqueues SendMaintenanceReminderEmail for every
// recipient that isn't opted out of the maintenance-reminder
// notification category. Returns (attempted, enqueueErrs,
// firstEnqueueErr) so the caller can decide between three terminal
// states: every attempt failed, every recipient opted out, or at
// least one enqueue succeeded. Extracted from processOne to keep the
// parent under the gocyclo ceiling.
func (s *MaintenanceReminderService) fanOutToRecipients(
	ctx context.Context,
	m *models.MaintenanceSchedule,
	threshold models.MaintenanceReminderThreshold,
	commodity *models.Commodity,
	recipients []maintenanceRecipient,
	prefsCache *notifications.Cache,
) (attempted, enqueueErrs int, firstEnqueueErr error) {
	dueDate := string(m.NextDueAt)
	url := s.buildCommodityURL(ctx, commodity)
	for _, r := range recipients {
		if threshold != models.MaintenanceReminderOverdue && (!r.hasDue || r.due != int(threshold)) {
			// This recipient did not choose this lead time, or has
			// already reached a closer one and gets only that. The
			// overdue reminder goes to everyone.
			continue
		}
		if prefsCache != nil && !prefsCache.IsEnabledForGroup(ctx, r.user, commodity.TenantID, commodity.GroupID, notifications.CategoryMaintenanceReminder, notifications.ChannelEmail) {
			slog.DebugContext(ctx, "maintenance reminder: recipient opted out",
				"schedule_id", m.ID,
				"group_id", commodity.GroupID,
				"to", r.email,
			)
			continue
		}
		attempted++
		sendErr := s.emailSvc.SendMaintenanceReminderEmail(withReminderLanguage(ctx, prefsCache, r.user), r.email, r.name, commodity.Name, m.Title, dueDate, url, int(threshold))
		if sendErr != nil {
			enqueueErrs++
			if firstEnqueueErr == nil {
				firstEnqueueErr = sendErr
			}
			slog.ErrorContext(ctx, "maintenance reminder: enqueue failed",
				"schedule_id", m.ID,
				"to", r.email,
				"error", sendErr,
			)
		}
	}
	return attempted, enqueueErrs, firstEnqueueErr
}

// commitReminderRow writes the (schedule, threshold) idempotency row.
// silent marks a row written without an email; see commitWithoutSend.
func (s *MaintenanceReminderService) commitReminderRow(ctx context.Context, m *models.MaintenanceSchedule, threshold models.MaintenanceReminderThreshold, now time.Time, silent bool) (bool, error) {
	reminder := models.MaintenanceReminder{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        m.TenantID,
			GroupID:         m.GroupID,
			CreatedByUserID: m.CreatedByUserID,
		},
		ScheduleID:    m.ID,
		ThresholdDays: int(threshold),
		SentAt:        now,
		Silent:        silent,
	}
	return s.factorySet.MaintenanceReminderRegistry.CreateOnce(ctx, reminder)
}

type maintenanceRecipient struct {
	email string
	name  string
	user  *models.User
	// leadTimes are the recipient's maintenance lead times, set by
	// planSchedule from their notification settings.
	leadTimes []int
	// due is the closest lead time the recipient has reached, the only
	// one they are emailed for before the due date; hasDue is false when
	// none is reached.
	due    int
	hasDue bool
	// fromMembership is true for group admins and false for the
	// commodity-creator fallback.
	fromMembership bool
}

// lookupCommodity resolves a commodity by ID via the service-mode
// registry. Returns (nil, nil) when the commodity has been hard-
// deleted (cross-tenant drift / mid-purge) — matches the
// registry.ErrNotFound sentinel via errors.Is so any wrapped form
// (errxtrace.Wrap, fmt.Errorf %w, etc.) still resolves correctly.
// Other errors propagate.
func (s *MaintenanceReminderService) lookupCommodity(ctx context.Context, commodityID string) (*models.Commodity, error) {
	commodityReg := s.factorySet.CommodityRegistryFactory.CreateServiceRegistry()
	c, err := commodityReg.Get(ctx, commodityID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

// commodityTrashed reports whether the item is in Trash. A lookup error reads
// as "not trashed", which falls back to the missing-commodity handling.
func (s *MaintenanceReminderService) commodityTrashed(ctx context.Context, commodityID string) bool {
	_, err := s.factorySet.CommodityRegistryFactory.CreateServiceRegistry().GetTrashed(ctx, commodityID)
	return err == nil
}

// recipientsForCommodity returns every group admin that should receive a
// reminder for this schedule, or the commodity's creator when the group has
// none. See reminderRecipientUsers.
func (s *MaintenanceReminderService) recipientsForCommodity(ctx context.Context, c *models.Commodity) ([]maintenanceRecipient, error) {
	users, fromMembership, err := reminderRecipientUsers(ctx, s.factorySet, c)
	if err != nil {
		return nil, err
	}
	out := make([]maintenanceRecipient, 0, len(users))
	for _, u := range users {
		out = append(out, maintenanceRecipient{email: u.Email, name: u.Name, user: u, fromMembership: fromMembership})
	}
	return out, nil
}

func (s *MaintenanceReminderService) buildCommodityURL(ctx context.Context, c *models.Commodity) string {
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
