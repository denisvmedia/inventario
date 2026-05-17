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

	for _, m := range schedules {
		if m == nil || !m.Enabled {
			continue
		}
		for _, threshold := range matchedMaintenanceThresholds(m, now) {
			ok, processErr := s.processOne(ctx, m, threshold, now, prefsCache)
			if processErr != nil {
				stats.Failed++
				slog.Error("maintenance reminder failed",
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

// matchedMaintenanceThresholds returns every threshold whose
// "days remaining" window contains the schedule at the given clock,
// plus MaintenanceReminderOverdue when the schedule is past due. A
// schedule N days from due matches every threshold T where N <= T
// (largest → smallest order), so a row 7 days out returns [14, 7].
// The overdue sentinel returns alone — once we're past due the
// 14/7/1 thresholds no longer apply, only the one-shot overdue
// reminder. The idempotency row guarantees each (schedule, threshold)
// fires at most once until the next MarkDone clears them.
func matchedMaintenanceThresholds(m *models.MaintenanceSchedule, now time.Time) []models.MaintenanceReminderThreshold {
	if m == nil || string(m.NextDueAt) == "" {
		return nil
	}
	if m.IsOverdue(now) {
		return []models.MaintenanceReminderThreshold{models.MaintenanceReminderOverdue}
	}
	due := m.NextDueAt.ToTime()
	if due.IsZero() {
		return nil
	}
	n := now.UTC()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	daysLeft := int(due.Sub(today).Hours() / 24)
	var out []models.MaintenanceReminderThreshold
	for _, t := range []models.MaintenanceReminderThreshold{
		models.MaintenanceReminder14Days,
		models.MaintenanceReminder7Days,
		models.MaintenanceReminder1Day,
	} {
		if daysLeft <= int(t) {
			out = append(out, t)
		}
	}
	return out
}

// processOne handles one (schedule, threshold) pair across the
// commit + send pipeline. Identical shape to
// WarrantyReminderService.processOne; see the comments there for the
// idempotency / retry / opt-out reasoning.
func (s *MaintenanceReminderService) processOne(ctx context.Context, m *models.MaintenanceSchedule, threshold models.MaintenanceReminderThreshold, now time.Time, prefsCache *notifications.Cache) (bool, error) {
	already, err := s.factorySet.MaintenanceReminderRegistry.HasSent(ctx, m.ID, int(threshold))
	if err != nil {
		return false, errxtrace.Wrap("maintenance reminder: check existing row", err)
	}
	if already {
		return false, nil
	}

	if s.emailSvc == nil {
		ok, commitErr := s.commitReminderRow(ctx, m, threshold, now)
		if commitErr != nil {
			return false, errxtrace.Wrap("maintenance reminder: insert idempotency row", commitErr)
		}
		if ok {
			slog.Info("maintenance reminder row inserted (no email service configured)",
				"schedule_id", m.ID,
				"threshold", int(threshold),
			)
		}
		return ok, nil
	}

	commodity, err := s.lookupCommodity(ctx, m.CommodityID)
	if err != nil {
		return false, errxtrace.Wrap("maintenance reminder: lookup commodity", err)
	}
	if commodity == nil {
		// Schedule references a commodity that's gone (cross-tenant
		// drift / mid-purge). Write the row so we stop revisiting and
		// move on.
		ok, commitErr := s.commitReminderRow(ctx, m, threshold, now)
		if commitErr != nil {
			return false, errxtrace.Wrap("maintenance reminder: insert idempotency row (no commodity)", commitErr)
		}
		return ok, nil
	}

	recipients, err := s.recipientsForCommodity(ctx, commodity)
	if err != nil {
		return false, errxtrace.Wrap("maintenance reminder: resolve recipients", err)
	}
	if len(recipients) == 0 {
		ok, commitErr := s.commitReminderRow(ctx, m, threshold, now)
		if commitErr != nil {
			return false, errxtrace.Wrap("maintenance reminder: insert idempotency row (no recipients)", commitErr)
		}
		if ok {
			slog.Warn("maintenance reminder: no recipients found for commodity",
				"schedule_id", m.ID,
				"commodity_id", commodity.ID,
				"group_id", commodity.GroupID,
			)
		}
		return ok, nil
	}

	dueDate := string(m.NextDueAt)
	url := s.buildCommodityURL(ctx, commodity)
	enqueueErrs := 0
	attempted := 0
	var firstEnqueueErr error
	for _, r := range recipients {
		if prefsCache != nil && !prefsCache.IsEnabledForGroup(ctx, r.user, commodity.TenantID, commodity.GroupID, notifications.CategoryMaintenanceReminder, notifications.ChannelEmail) {
			slog.Debug("maintenance reminder: recipient opted out",
				"schedule_id", m.ID,
				"group_id", commodity.GroupID,
				"to", r.email,
			)
			continue
		}
		attempted++
		sendErr := s.emailSvc.SendMaintenanceReminderEmail(ctx, r.email, r.name, commodity.Name, m.Title, dueDate, url, int(threshold))
		if sendErr != nil {
			enqueueErrs++
			if firstEnqueueErr == nil {
				firstEnqueueErr = sendErr
			}
			slog.Error("maintenance reminder: enqueue failed",
				"schedule_id", m.ID,
				"to", r.email,
				"error", sendErr,
			)
		}
	}
	if attempted > 0 && enqueueErrs == attempted {
		return false, errxtrace.Wrap("maintenance reminder: all enqueues failed", firstEnqueueErr)
	}
	if attempted == 0 {
		if _, err := s.commitReminderRow(ctx, m, threshold, now); err != nil {
			return false, errxtrace.Wrap("maintenance reminder: insert idempotency row (all opted out)", err)
		}
		return false, nil
	}
	ok, err := s.commitReminderRow(ctx, m, threshold, now)
	if err != nil {
		return false, errxtrace.Wrap("maintenance reminder: insert idempotency row", err)
	}
	return ok, nil
}

func (s *MaintenanceReminderService) commitReminderRow(ctx context.Context, m *models.MaintenanceSchedule, threshold models.MaintenanceReminderThreshold, now time.Time) (bool, error) {
	reminder := models.MaintenanceReminder{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        m.TenantID,
			GroupID:         m.GroupID,
			CreatedByUserID: m.CreatedByUserID,
		},
		ScheduleID:    m.ID,
		ThresholdDays: int(threshold),
		SentAt:        now,
	}
	return s.factorySet.MaintenanceReminderRegistry.CreateOnce(ctx, reminder)
}

type maintenanceRecipient struct {
	email string
	name  string
	user  *models.User
}

// lookupCommodity resolves a commodity by ID via the service-mode
// registry. Returns (nil, nil) when the commodity has been hard-
// deleted (cross-tenant drift / mid-purge). Other errors propagate.
func (s *MaintenanceReminderService) lookupCommodity(ctx context.Context, commodityID string) (*models.Commodity, error) {
	commodityReg := s.factorySet.CommodityRegistryFactory.CreateServiceRegistry()
	c, err := commodityReg.Get(ctx, commodityID)
	if err != nil {
		// ErrNotFound is treated as "swept-out" — see callers.
		if isMaintenanceNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return c, nil
}

func isMaintenanceNotFound(err error) bool {
	// Avoid importing errors here just to keep the function tight —
	// the wrappers preserve sentinel checks via errors.Is.
	return err != nil && err.Error() != "" && strings.Contains(err.Error(), "not found")
}

// recipientsForCommodity returns every group admin/owner that should
// receive a reminder for this schedule.
func (s *MaintenanceReminderService) recipientsForCommodity(ctx context.Context, c *models.Commodity) ([]maintenanceRecipient, error) {
	out := make([]maintenanceRecipient, 0, 4)
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
			out = append(out, maintenanceRecipient{email: user.Email, name: user.Name, user: user})
		}
	}
	return out, nil
}

// collectAdminRecipients filters memberships to admin-role rows,
// resolves each to a User, and returns the deduplicated recipient
// list. Mirrors the warranty service's collectAdminRecipients.
func (s *MaintenanceReminderService) collectAdminRecipients(
	ctx context.Context,
	groupID string,
	members []*models.GroupMembership,
) []maintenanceRecipient {
	out := make([]maintenanceRecipient, 0, 4)
	seen := make(map[string]struct{}, 4)
	for _, m := range members {
		if m == nil || m.Role != models.GroupRoleAdmin {
			continue
		}
		user, err := s.factorySet.UserRegistry.Get(ctx, m.MemberUserID)
		if err != nil {
			slog.Warn("maintenance reminder: skip member with missing user",
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
		out = append(out, maintenanceRecipient{email: user.Email, name: user.Name, user: user})
	}
	return out
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
