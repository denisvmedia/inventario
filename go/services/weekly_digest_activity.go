package services

import (
	"context"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
)

// groupActivity is what one group contributes to every digest that includes it.
// Computed once per sweep and shared by each member, which is the point of
// bucketing by group rather than querying per user.
type groupActivity struct {
	counts   WeeklyDigestGroupCounts
	upcoming []WeeklyDigestUpcoming
}

// collectActivity reads the collections the digest summarises once each and
// buckets them by group.
//
// Every read is a full list filtered in Go, which is what the reminder workers
// already do: the registries expose no group-plus-time-range query, and adding
// one for this would be a wider change than the digest. It is the sweep's cost
// model, not the request path's.
func (s *WeeklyDigestService) collectActivity(ctx context.Context, window WeeklyDigestWindow, now time.Time) (map[string]groupActivity, error) {
	activity := map[string]groupActivity{}

	commoditiesByID, err := s.countItemActivity(ctx, window, activity)
	if err != nil {
		return nil, err
	}
	if err := s.countFileActivity(ctx, window, activity); err != nil {
		return nil, err
	}
	if err := s.collectUpcomingWarranties(ctx, now, commoditiesByID, activity); err != nil {
		return nil, err
	}
	if err := s.collectUpcomingMaintenance(ctx, now, commoditiesByID, activity); err != nil {
		return nil, err
	}
	return activity, nil
}

// countItemActivity fills in items added and items changed from the commodity
// event log, and returns the commodities by id for the callers that need names
// and group slugs.
//
// The event log rather than the commodities table because commodities carry no
// created_at column: "added this week" is only answerable from the log, which
// records a `created` event per commodity.
func (s *WeeklyDigestService) countItemActivity(ctx context.Context, window WeeklyDigestWindow, activity map[string]groupActivity) (map[string]*models.Commodity, error) {
	commoditiesByID := map[string]*models.Commodity{}
	if s.factorySet.CommodityRegistryFactory == nil || s.factorySet.CommodityEventRegistryFactory == nil {
		return commoditiesByID, nil
	}

	commodities, err := s.factorySet.CommodityRegistryFactory.CreateServiceRegistry().List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("weekly digest: list commodities", err)
	}
	for _, c := range commodities {
		if c != nil {
			commoditiesByID[c.ID] = c
		}
	}

	events, err := s.factorySet.CommodityEventRegistryFactory.CreateServiceRegistry().List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("weekly digest: list commodity events", err)
	}

	// A commodity edited five times is one changed item, not five, so the
	// changed count comes from a set of commodity ids rather than a counter.
	changed := map[string]map[string]bool{}
	for _, e := range events {
		if e == nil || !window.Contains(e.OccurredAt) {
			continue
		}
		entry := activity[e.GroupID]
		switch e.Kind {
		case models.CommodityEventKindCreated:
			entry.counts.ItemsAdded++
		case models.CommodityEventKindDeleted:
			// A deletion is not activity worth reporting: the item is gone, so
			// there is nothing for the reader to look at.
		default:
			set, ok := changed[e.GroupID]
			if !ok {
				set = map[string]bool{}
				changed[e.GroupID] = set
			}
			set[e.CommodityID] = true
		}
		activity[e.GroupID] = entry
	}
	for groupID, set := range changed {
		entry := activity[groupID]
		entry.counts.ItemsChanged = len(set)
		activity[groupID] = entry
	}
	return commoditiesByID, nil
}

// countFileActivity counts the files attached in the window per group.
func (s *WeeklyDigestService) countFileActivity(ctx context.Context, window WeeklyDigestWindow, activity map[string]groupActivity) error {
	if s.factorySet.FileRegistryFactory == nil {
		return nil
	}
	files, err := s.factorySet.FileRegistryFactory.CreateServiceRegistry().List(ctx)
	if err != nil {
		return errxtrace.Wrap("weekly digest: list files", err)
	}
	for _, f := range files {
		if f == nil || !window.Contains(f.CreatedAt) {
			continue
		}
		entry := activity[f.GroupID]
		entry.counts.FilesAdded++
		activity[f.GroupID] = entry
	}
	return nil
}

// collectUpcomingWarranties adds the warranties expiring inside the lookahead.
func (s *WeeklyDigestService) collectUpcomingWarranties(ctx context.Context, now time.Time, commoditiesByID map[string]*models.Commodity, activity map[string]groupActivity) error {
	_ = ctx
	today := truncateToDayUTC(now)
	cutoff := today.AddDate(0, 0, digestWarrantyLookaheadDays)

	for _, c := range commoditiesByID {
		if c.WarrantyExpiresAt == nil || string(*c.WarrantyExpiresAt) == "" {
			continue
		}
		expiry := c.WarrantyExpiresAt.ToTime()
		if expiry.IsZero() || expiry.Before(today) || expiry.After(cutoff) {
			continue
		}
		entry := activity[c.GroupID]
		entry.upcoming = append(entry.upcoming, WeeklyDigestUpcoming{
			Kind: WeeklyDigestUpcomingWarranty,
			Name: c.Name,
			Date: string(*c.WarrantyExpiresAt),
			URL:  s.commodityURL(c),
		})
		activity[c.GroupID] = entry
	}
	return nil
}

// collectUpcomingMaintenance adds the maintenance schedules due inside the
// lookahead.
func (s *WeeklyDigestService) collectUpcomingMaintenance(ctx context.Context, now time.Time, commoditiesByID map[string]*models.Commodity, activity map[string]groupActivity) error {
	if s.factorySet.MaintenanceScheduleRegistryFactory == nil {
		return nil
	}
	schedules, err := s.factorySet.MaintenanceScheduleRegistryFactory.CreateServiceRegistry().List(ctx)
	if err != nil {
		return errxtrace.Wrap("weekly digest: list maintenance schedules", err)
	}
	for _, m := range schedules {
		if m == nil || !m.IsDueWithin(now, digestMaintenanceLookaheadDays) {
			continue
		}
		commodity := commoditiesByID[m.CommodityID]
		name := m.Title
		if commodity != nil && commodity.Name != "" {
			name = commodity.Name + " — " + m.Title
		}
		entry := activity[m.GroupID]
		entry.upcoming = append(entry.upcoming, WeeklyDigestUpcoming{
			Kind: WeeklyDigestUpcomingMaintenance,
			Name: name,
			Date: string(m.NextDueAt),
			URL:  s.commodityURL(commodity),
		})
		activity[m.GroupID] = entry
	}
	return nil
}

// commodityURL builds the deep link for a commodity, or returns empty when
// there is no builder or no group slug to build it from.
func (s *WeeklyDigestService) commodityURL(c *models.Commodity) string {
	if c == nil || s.commodityURLBuilder == nil || s.factorySet.LocationGroupRegistry == nil {
		return ""
	}
	group, err := s.factorySet.LocationGroupRegistry.Get(context.Background(), c.GroupID)
	if err != nil || group == nil || group.Slug == "" {
		return ""
	}
	return s.commodityURLBuilder(group.Slug, c.ID)
}

func truncateToDayUTC(t time.Time) time.Time {
	utc := t.UTC()
	return time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
}
