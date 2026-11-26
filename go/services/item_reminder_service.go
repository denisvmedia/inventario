package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/notifications"
)

// ItemReminderService runs one sweep over the reminders users set on items
// (#2805). For each reminder it computes the next occurrence, picks the
// lead time that is due, and emails the reminder's author once per
// (reminder, occurrence, lead time).
//
// The send is claimed before the email is enqueued: CreateOnce inserts the
// commodity_reminder_sends row, and only the worker whose insert succeeded
// sends. Two replicas sweeping at the same moment therefore send one email,
// and a restart does not resend. When the enqueue fails the claim is
// released, so the next sweep retries.
//
// A recipient who has turned off notifications.item_reminder, or who has
// left the group, gets nothing and no row is written; re-enabling the
// toggle picks up the reminder at its current lead time.
type ItemReminderService struct {
	factorySet *registry.FactorySet
	emailSvc   EmailService
	// commodityURLBuilder builds the item link printed in the email.
	// Optional; when nil the email has no link.
	commodityURLBuilder func(groupSlug, commodityID string) string
	// prefs gates each recipient on their notification toggles. Optional;
	// when nil every recipient is treated as opted in.
	prefs *notifications.Service
}

func NewItemReminderService(factorySet *registry.FactorySet, emailSvc EmailService, urlBuilder func(groupSlug, commodityID string) string) *ItemReminderService {
	return &ItemReminderService{
		factorySet:          factorySet,
		emailSvc:            emailSvc,
		commodityURLBuilder: urlBuilder,
	}
}

// WithPreferences attaches the notification preferences service.
func (s *ItemReminderService) WithPreferences(prefs *notifications.Service) *ItemReminderService {
	s.prefs = prefs
	return s
}

// ItemReminderStats summarizes one sweep.
type ItemReminderStats struct {
	// Sent counts the emails enqueued.
	Sent int
	// Failed counts reminders whose processing failed; they are retried
	// on the next sweep.
	Failed int
}

// DueLeadTime returns the lead time an occurrence daysLeft days away is
// due for: the smallest lead time L with daysLeft <= L. It returns false
// when no lead time has been reached yet or the occurrence has passed.
//
// Picking the smallest reached lead time is what keeps a late start from
// sending a burst: a reminder created three days before its date with lead
// times 30, 7 and 1 sends one email now (for 7) and one on the eve (for 1),
// not three.
func DueLeadTime(leads []int, daysLeft int) (int, bool) {
	if daysLeft < 0 {
		return 0, false
	}
	best, found := 0, false
	for _, l := range leads {
		if daysLeft <= l && (!found || l < best) {
			best, found = l, true
		}
	}
	return best, found
}

// sweepState carries the per-sweep caches.
type sweepState struct {
	now         time.Time
	prefs       *notifications.Cache
	commodities map[string]*models.Commodity
	groupSlugs  map[string]string
}

// RemindOnce runs one sweep pinned to now.
func (s *ItemReminderService) RemindOnce(ctx context.Context, now time.Time) (ItemReminderStats, error) {
	var stats ItemReminderStats
	if s.factorySet == nil {
		return stats, errxtrace.Wrap("item reminder service: factorySet is required", registry.ErrFieldRequired)
	}
	reminders, err := s.factorySet.CommodityReminderRegistryFactory.CreateServiceRegistry().List(ctx)
	if err != nil {
		return stats, errxtrace.Wrap("item reminder: list reminders", err)
	}

	st := &sweepState{
		now:         now,
		commodities: make(map[string]*models.Commodity),
		groupSlugs:  make(map[string]string),
	}
	if s.prefs != nil {
		st.prefs = s.prefs.NewCache()
	}

	for _, r := range reminders {
		if r == nil {
			continue
		}
		occurrence, ok := r.NextOccurrence(now)
		if !ok {
			continue
		}
		daysLeft := models.DaysUntil(occurrence, now)
		lead, ok := DueLeadTime(r.RemindDaysBefore, daysLeft)
		if !ok {
			continue
		}
		sent, processErr := s.processOne(ctx, st, r, occurrence, lead, daysLeft)
		if processErr != nil {
			stats.Failed++
			slog.ErrorContext(ctx, "item reminder failed",
				"reminder_id", r.ID,
				"occurrence_on", string(occurrence),
				"lead_days", lead,
				"error", processErr,
			)
			continue
		}
		if sent {
			stats.Sent++
		}
	}
	return stats, nil
}

// processOne handles one due (reminder, occurrence, lead time). It returns
// true only when an email was enqueued.
func (s *ItemReminderService) processOne(ctx context.Context, st *sweepState, r *models.CommodityReminder, occurrence models.Date, lead, daysLeft int) (bool, error) {
	sends := s.factorySet.CommodityReminderSendRegistry
	already, err := sends.HasSentAtOrBelow(ctx, r.ID, occurrence, lead)
	if err != nil {
		return false, errxtrace.Wrap("item reminder: check sends", err)
	}
	if already {
		return false, nil
	}

	commodity, err := s.lookupCommodity(ctx, st, r.CommodityID)
	if err != nil {
		return false, errxtrace.Wrap("item reminder: lookup commodity", err)
	}
	if !ReminderTargetActive(commodity) {
		return false, nil
	}

	if s.emailSvc == nil {
		// Stub mode (tests, dev): record the send so the sweep moves on,
		// but report nothing sent.
		_, claimErr := sends.CreateOnce(ctx, newReminderSend(r, occurrence, lead, st.now))
		return false, claimErr
	}

	user, err := s.recipient(ctx, r)
	if err != nil {
		return false, errxtrace.Wrap("item reminder: resolve recipient", err)
	}
	if user == nil {
		return false, nil
	}
	if st.prefs != nil && !st.prefs.IsEnabledForGroup(ctx, user, r.TenantID, r.GroupID, notifications.CategoryItemReminder, notifications.ChannelEmail) {
		slog.DebugContext(ctx, "item reminder: recipient opted out", "reminder_id", r.ID, "user_id", user.ID)
		return false, nil
	}

	claimed, err := sends.CreateOnce(ctx, newReminderSend(r, occurrence, lead, st.now))
	if err != nil {
		return false, errxtrace.Wrap("item reminder: claim send", err)
	}
	if !claimed {
		// Another worker claimed it between the check and the insert.
		return false, nil
	}

	url := s.buildCommodityURL(ctx, st, commodity)
	sendErr := s.emailSvc.SendItemReminderEmail(withReminderLanguage(ctx, st.prefs, user), user.Email, user.Name, commodity.Name, r.Title, string(occurrence), url, daysLeft)
	if sendErr != nil {
		if relErr := sends.Release(ctx, r.ID, occurrence, lead); relErr != nil {
			return false, errxtrace.Wrap("item reminder: release claim after failed enqueue", errors.Join(sendErr, relErr))
		}
		return false, errxtrace.Wrap("item reminder: enqueue email", sendErr)
	}
	return true, nil
}

func newReminderSend(r *models.CommodityReminder, occurrence models.Date, lead int, now time.Time) models.CommodityReminderSend {
	return models.CommodityReminderSend{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        r.TenantID,
			GroupID:         r.GroupID,
			CreatedByUserID: r.CreatedByUserID,
		},
		ReminderID:   r.ID,
		OccurrenceOn: occurrence,
		LeadDays:     lead,
		SentAt:       now,
	}
}

// recipient returns the reminder's author when they can still receive it:
// the account exists, is active, has an email address and is still a
// member of the reminder's group. It returns (nil, nil) otherwise.
func (s *ItemReminderService) recipient(ctx context.Context, r *models.CommodityReminder) (*models.User, error) {
	if r.CreatedByUserID == "" || s.factorySet.UserRegistry == nil {
		return nil, nil
	}
	user, err := s.factorySet.UserRegistry.Get(ctx, r.CreatedByUserID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if user == nil || !user.IsActive || strings.TrimSpace(user.Email) == "" {
		return nil, nil
	}
	if s.factorySet.GroupMembershipRegistry != nil {
		if _, err := s.factorySet.GroupMembershipRegistry.GetByGroupAndUser(ctx, r.GroupID, user.ID); err != nil {
			if errors.Is(err, registry.ErrNotFound) {
				return nil, nil
			}
			return nil, err
		}
	}
	return user, nil
}

// lookupCommodity resolves an item through the service registry and caches
// the result for the sweep. A deleted item resolves to nil.
func (s *ItemReminderService) lookupCommodity(ctx context.Context, st *sweepState, commodityID string) (*models.Commodity, error) {
	if c, ok := st.commodities[commodityID]; ok {
		return c, nil
	}
	c, err := s.factorySet.CommodityRegistryFactory.CreateServiceRegistry().Get(ctx, commodityID)
	if err != nil {
		if !errors.Is(err, registry.ErrNotFound) {
			return nil, err
		}
		c = nil
	}
	st.commodities[commodityID] = c
	return c, nil
}

func (s *ItemReminderService) buildCommodityURL(ctx context.Context, st *sweepState, c *models.Commodity) string {
	if s.commodityURLBuilder == nil || s.factorySet.LocationGroupRegistry == nil {
		return ""
	}
	slug, ok := st.groupSlugs[c.GroupID]
	if !ok {
		if group, err := s.factorySet.LocationGroupRegistry.Get(ctx, c.GroupID); err == nil && group != nil {
			slug = group.Slug
		}
		st.groupSlugs[c.GroupID] = slug
	}
	if slug == "" {
		return ""
	}
	return s.commodityURLBuilder(slug, c.ID)
}
