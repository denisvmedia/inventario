package services

import (
	"context"
	"log/slog"
	"sort"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/notifications"
)

// Windows the digest looks ahead over. Both are longer than the matching
// reminder thresholds on purpose: the digest is the weekly overview, so it
// should mention something before the reminder for it fires, not after.
const (
	digestWarrantyLookaheadDays    = 30
	digestMaintenanceLookaheadDays = 14
	// digestMaxUpcoming caps the "coming up" list. A digest is a nudge; a user
	// with forty expiring warranties needs the warranties page, not a longer
	// email.
	digestMaxUpcoming = 6
)

// WeeklyDigestService sends one digest per user per week: what happened in the
// groups they belong to, and what is coming up (#1391).
//
// One sweep reads each collection once and buckets it by group, rather than
// querying per user. A user in three groups then costs no more queries than a
// user in one, and two users in the same group share the work.
//
// The service is stateless and takes the clock as an argument so a test can pin
// "now" to a Monday and assert the window boundaries exactly.
type WeeklyDigestService struct {
	factorySet *registry.FactorySet
	emailSvc   EmailService
	prefs      *notifications.Service

	// URL builders for the links in the email. Both optional: the template drops
	// the matching block when the builder is nil or returns empty, which is what
	// a deployment with no public URL configured gets.
	appURLBuilder       func(groupSlug string) string
	settingsURLBuilder  func(groupSlug string) string
	commodityURLBuilder func(groupSlug, commodityID string) string
}

// NewWeeklyDigestService constructs the service. emailSvc may be nil in tests
// that only assert the claim side.
func NewWeeklyDigestService(
	factorySet *registry.FactorySet,
	emailSvc EmailService,
	appURLBuilder func(groupSlug string) string,
	settingsURLBuilder func(groupSlug string) string,
	commodityURLBuilder func(groupSlug, commodityID string) string,
) *WeeklyDigestService {
	return &WeeklyDigestService{
		factorySet:          factorySet,
		emailSvc:            emailSvc,
		appURLBuilder:       appURLBuilder,
		settingsURLBuilder:  settingsURLBuilder,
		commodityURLBuilder: commodityURLBuilder,
	}
}

// WithPreferences attaches the notification preference service, so a user who
// has the weekly digest switched off for a group does not get that group's
// activity. Returns the same service for chaining at the bootstrap site.
//
// Without it every group counts, which is what the constructor alone gives a
// test that does not care about preferences.
func (s *WeeklyDigestService) WithPreferences(prefs *notifications.Service) *WeeklyDigestService {
	s.prefs = prefs
	return s
}

// WeeklyDigestStats summarises one sweep.
type WeeklyDigestStats struct {
	// Sent counts digests handed to the email service.
	Sent int
	// Nothing counts users whose groups had no activity in the window. A digest
	// saying nothing happened is how a weekly email trains people to filter it.
	Nothing int
	// AlreadySent counts users whose week was already claimed — another replica,
	// or a restart inside the same week.
	AlreadySent int
	// Failed counts users whose digest could not be built or enqueued. The next
	// tick retries them, because the claim is rolled back on a send failure.
	Failed int
}

// SendOnce runs one sweep for the week ending at the start of now's week, and
// reports what it did.
//
// A non-nil error means the sweep could not start — a missing registry, or the
// group listing itself failing. A user whose own digest fails is counted in
// Failed and does not stop the others.
func (s *WeeklyDigestService) SendOnce(ctx context.Context, now time.Time) (WeeklyDigestStats, error) {
	var stats WeeklyDigestStats

	if s.factorySet == nil {
		return stats, errxtrace.Wrap("weekly digest service: factorySet is required", registry.ErrFieldRequired)
	}
	for name, present := range map[string]bool{
		"LocationGroupRegistry":    s.factorySet.LocationGroupRegistry != nil,
		"GroupMembershipRegistry":  s.factorySet.GroupMembershipRegistry != nil,
		"UserRegistry":             s.factorySet.UserRegistry != nil,
		"WeeklyDigestSendRegistry": s.factorySet.WeeklyDigestSendRegistry != nil,
	} {
		if !present {
			return stats, errxtrace.Wrap("weekly digest service: "+name+" is required", registry.ErrFieldRequired)
		}
	}

	window := WeeklyDigestWindowFor(now)

	// One cache per sweep: it memoises the per-user preference lookups, which
	// repeat for every group a user belongs to.
	var prefsCache *notifications.Cache
	if s.prefs != nil {
		prefsCache = s.prefs.NewCache()
	}

	activity, err := s.collectActivity(ctx, window, now)
	if err != nil {
		return stats, err
	}

	groups, err := s.factorySet.LocationGroupRegistry.List(ctx)
	if err != nil {
		return stats, errxtrace.Wrap("weekly digest: list groups", err)
	}

	drafts, order, failed := s.buildDrafts(ctx, groups, activity, prefsCache)
	stats.Failed += failed

	for _, userID := range order {
		draft := drafts[userID]
		switch sent, sendErr := s.deliver(ctx, draft, window, prefsCache); {
		case sendErr != nil:
			slog.Error("Weekly digest: failed to send",
				"user_id", userID, "week_start", window.Start.Format(time.DateOnly), "error", sendErr)
			stats.Failed++
		case sent == digestSent:
			stats.Sent++
		case sent == digestAlreadySent:
			stats.AlreadySent++
		default:
			stats.Nothing++
		}
	}

	return stats, nil
}

// buildDrafts turns the groups and their activity into one draft per recipient,
// in the order the groups were listed so two runs over the same data produce
// the same email.
//
// The third return is how many groups could not be read; those are counted as
// failures rather than silently producing a thinner digest.
func (s *WeeklyDigestService) buildDrafts(
	ctx context.Context,
	groups []*models.LocationGroup,
	activity map[string]groupActivity,
	prefsCache *notifications.Cache,
) (drafts map[string]*digestDraft, order []string, failed int) {
	drafts = map[string]*digestDraft{}

	for _, group := range groups {
		if group == nil {
			continue
		}
		members, err := s.factorySet.GroupMembershipRegistry.ListByGroup(ctx, group.ID)
		if err != nil {
			slog.Error("Weekly digest: failed to list group members",
				"group_id", group.ID, "error", err)
			failed++
			continue
		}
		for _, m := range members {
			if m == nil {
				continue
			}
			user := s.digestRecipient(ctx, m.MemberUserID)
			if user == nil {
				continue
			}
			if prefsCache != nil && !prefsCache.IsEnabledForGroup(ctx, user, group.TenantID, group.ID,
				notifications.CategoryWeeklyDigest, notifications.ChannelEmail) {
				continue
			}

			draft, ok := drafts[user.ID]
			if !ok {
				draft = &digestDraft{user: user}
				drafts[user.ID] = draft
				order = append(order, user.ID)
			}
			draft.add(group, activity[group.ID])
		}
	}
	return drafts, order, failed
}

// digestRecipient returns the user to send to, or nil when there is nobody to
// send to: a blocked account or one with no address gets no digest, and a
// lookup failure is treated the same way rather than stopping the sweep.
func (s *WeeklyDigestService) digestRecipient(ctx context.Context, userID string) *models.User {
	user, err := s.factorySet.UserRegistry.Get(ctx, userID)
	if err != nil || user == nil || !user.IsActive || user.Email == "" {
		return nil
	}
	return user
}

// digestOutcome is what deliver did with one draft.
type digestOutcome int

const (
	digestNothingToSay digestOutcome = iota
	digestSent
	digestAlreadySent
)

// digestDraft accumulates one recipient's digest across the groups they belong
// to. The issue asks for one combined email rather than one per group.
type digestDraft struct {
	user     *models.User
	groups   []WeeklyDigestGroupCounts
	upcoming []WeeklyDigestUpcoming
	// firstSlug is the group whose settings page the unsubscribe link points at:
	// the toggle is per group, and the first group in the digest is the one the
	// reader will recognise.
	firstSlug string
}

func (d *digestDraft) add(group *models.LocationGroup, a groupActivity) {
	if d.firstSlug == "" {
		d.firstSlug = group.Slug
	}
	counts := a.counts
	counts.GroupName = group.Name
	if !counts.Empty() {
		d.groups = append(d.groups, counts)
	}
	// The group name is stamped here rather than while collecting, because the
	// activity is shared by every member of the group and the name is what the
	// "coming up" line ends with. Left unset it renders as an empty pair of
	// brackets.
	for _, u := range a.upcoming {
		u.GroupName = group.Name
		d.upcoming = append(d.upcoming, u)
	}
}

// deliver claims the week and hands the digest to the email service.
//
// The claim comes first, so two replicas cannot both send. When the send then
// fails the claim is released, which is the one case where a duplicate is
// possible — a send that failed after the mail was accepted. That is a worse
// trade for a reminder with a deadline and a fair one here.
func (s *WeeklyDigestService) deliver(ctx context.Context, draft *digestDraft, window WeeklyDigestWindow, prefsCache *notifications.Cache) (digestOutcome, error) {
	if len(draft.groups) == 0 {
		return digestNothingToSay, nil
	}

	claimed, err := s.factorySet.WeeklyDigestSendRegistry.ClaimWeek(ctx, models.WeeklyDigestSend{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: draft.user.TenantID,
			UserID:   draft.user.ID,
		},
		WeekStart: window.Start,
	})
	if err != nil {
		return digestNothingToSay, errxtrace.Wrap("weekly digest: claim week", err)
	}
	if !claimed {
		return digestAlreadySent, nil
	}
	if s.emailSvc == nil {
		return digestSent, nil
	}

	upcoming := draft.upcoming
	sort.SliceStable(upcoming, func(i, j int) bool { return upcoming[i].Date < upcoming[j].Date })
	if len(upcoming) > digestMaxUpcoming {
		upcoming = upcoming[:digestMaxUpcoming]
	}

	name := draft.user.Name
	if name == "" {
		name = draft.user.Email
	}

	err = s.emailSvc.SendWeeklyDigestEmail(
		withReminderLanguage(ctx, prefsCache, draft.user),
		draft.user.Email, name,
		WeeklyDigestEmail{
			WeekStart:   window.Start.Format(time.DateOnly),
			WeekEnd:     window.End.AddDate(0, 0, -1).Format(time.DateOnly),
			Groups:      draft.groups,
			Upcoming:    upcoming,
			URL:         buildDigestURL(s.appURLBuilder, draft.firstSlug),
			SettingsURL: buildDigestURL(s.settingsURLBuilder, draft.firstSlug),
		})
	if err != nil {
		// Release this user's claim so the next tick can try again. Leaving it
		// would turn a transient queue outage into a silently skipped week.
		if relErr := s.factorySet.WeeklyDigestSendRegistry.ReleaseWeek(ctx, draft.user.ID, window.Start); relErr != nil {
			slog.Error("Weekly digest: failed to release the claim after a send error",
				"user_id", draft.user.ID, "error", relErr)
		}
		return digestNothingToSay, errxtrace.Wrap("weekly digest: send email", err)
	}
	return digestSent, nil
}

func buildDigestURL(builder func(string) string, slug string) string {
	if builder == nil || slug == "" {
		return ""
	}
	return builder(slug)
}
