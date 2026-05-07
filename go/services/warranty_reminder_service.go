package services

import (
	"context"
	"log/slog"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
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

// RemindOnce runs one sweep pinned to `now`. Returns the number of
// reminder rows successfully inserted in this tick (`sent`) — equivalent
// to the number of email enqueue attempts since each new row implies an
// email (idempotency-row INSERT and email enqueue happen in lockstep) —
// and the number of commodities whose threshold processing failed
// (`failed`). A non-nil error is only returned when the initial listing
// itself fails.
func (s *WarrantyReminderService) RemindOnce(ctx context.Context, now time.Time) (sent, failed int, err error) {
	if s.factorySet == nil {
		return 0, 0, errxtrace.Wrap("warranty reminder service: factorySet is required", registry.ErrFieldRequired)
	}
	commodityReg := s.factorySet.CommodityRegistryFactory.CreateServiceRegistry()
	commodities, err := commodityReg.List(ctx)
	if err != nil {
		return 0, 0, errxtrace.Wrap("warranty reminder: list commodities", err)
	}

	for _, c := range commodities {
		if c == nil || c.WarrantyExpiresAt == nil || string(*c.WarrantyExpiresAt) == "" {
			continue
		}
		for _, threshold := range matchedThresholds(c.WarrantyExpiresAt, now) {
			ok, processErr := s.processOne(ctx, c, threshold)
			if processErr != nil {
				failed++
				slog.Error("warranty reminder failed",
					"commodity_id", c.ID,
					"threshold_days", threshold,
					"error", processErr,
				)
				continue
			}
			if ok {
				sent++
			}
		}
	}
	return sent, failed, nil
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

// processOne writes the idempotency row for (commodity, threshold) and
// — only if this call won the insert — enqueues an email per recipient.
// Returns (true, nil) on the winner path, (false, nil) when the row
// already existed (no email needed), or (false, err) on a registry
// failure.
func (s *WarrantyReminderService) processOne(ctx context.Context, c *models.Commodity, threshold models.WarrantyReminderThreshold) (bool, error) {
	reminder := models.WarrantyReminder{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        c.TenantID,
			GroupID:         c.GroupID,
			CreatedByUserID: c.CreatedByUserID,
		},
		CommodityID:   c.ID,
		ThresholdDays: int(threshold),
		SentAt:        time.Now(),
	}
	inserted, err := s.factorySet.WarrantyReminderRegistry.CreateOnce(ctx, reminder)
	if err != nil {
		return false, errxtrace.Wrap("warranty reminder: insert idempotency row", err)
	}
	if !inserted {
		return false, nil
	}
	if s.emailSvc == nil {
		// Tests that only care about the idempotency row pass a nil
		// EmailService; treat as a successful "sent" so callers see a
		// non-zero counter. The slog message keeps the no-op visible.
		slog.Info("warranty reminder row inserted (no email service configured)",
			"commodity_id", c.ID,
			"threshold_days", int(threshold),
		)
		return true, nil
	}

	recipients, err := s.recipientsForCommodity(ctx, c)
	if err != nil {
		// The idempotency row is already committed — re-emitting it on
		// the next tick is a no-op, but the recipient list error is
		// worth surfacing.
		return false, errxtrace.Wrap("warranty reminder: resolve recipients", err)
	}
	if len(recipients) == 0 {
		slog.Warn("warranty reminder: no recipients found for commodity",
			"commodity_id", c.ID,
			"group_id", c.GroupID,
		)
		return true, nil
	}

	expiry := string(*c.WarrantyExpiresAt)
	url := s.buildCommodityURL(ctx, c)
	for _, r := range recipients {
		if err := s.emailSvc.SendWarrantyReminderEmail(ctx, r.email, r.name, c.Name, expiry, url, int(threshold)); err != nil {
			// Logged but not bubbled — the email queue handles retries
			// internally. We still count this iteration as "sent".
			slog.Error("warranty reminder: enqueue failed",
				"commodity_id", c.ID,
				"to", r.email,
				"error", err,
			)
		}
	}
	return true, nil
}

type warrantyRecipient struct {
	email string
	name  string
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
			out = append(out, warrantyRecipient{email: user.Email, name: user.Name})
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
		out = append(out, warrantyRecipient{email: user.Email, name: user.Name})
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
