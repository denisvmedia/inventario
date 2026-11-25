package services

import (
	"context"
	"sort"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// UpcomingKind is the source of an entry on the Upcoming page.
type UpcomingKind string

const (
	UpcomingKindReminder    UpcomingKind = "reminder"
	UpcomingKindWarranty    UpcomingKind = "warranty"
	UpcomingKindLoan        UpcomingKind = "loan"
	UpcomingKindMaintenance UpcomingKind = "maintenance"
)

// Window limits for the Upcoming list, in days.
const (
	UpcomingDefaultDays = 30
	UpcomingMaxDays     = 365
	// upcomingPerKindLimit caps the rows read per source. A group with
	// more dated entries than this inside one window is far beyond what
	// one page can show.
	upcomingPerKindLimit = 500
)

// UpcomingEntry is one dated entry on the Upcoming page.
type UpcomingEntry struct {
	Kind UpcomingKind
	// Date is the entry's date: the reminder occurrence, the warranty
	// expiry, the loan's due-back date or the maintenance due date.
	Date models.Date
	// DaysLeft is the whole days from today to Date; negative when the
	// entry is overdue (loans and maintenance only).
	DaysLeft int
	// Title is the reminder or maintenance title, or the borrower's name
	// for a loan. It is empty for a warranty.
	Title string
	// SourceID is the reminder, loan or maintenance schedule id; for a
	// warranty it is the item id.
	SourceID    string
	CommodityID string
	Commodity   *models.Commodity
}

// UpcomingService builds the group-wide Upcoming list (#2805): item
// reminders, warranty expiries, loan due dates and maintenance due dates
// in one date-ordered list.
type UpcomingService struct{}

func NewUpcomingService() *UpcomingService {
	return &UpcomingService{}
}

// List returns the entries of the current group dated from today up to
// today+days, plus overdue loans and maintenance, oldest first. regSet is
// the request's group-scoped registry set. days is clamped to
// [1, UpcomingMaxDays]. Entries whose item is gone, or no longer active
// (see ReminderTargetActive), are left out.
func (*UpcomingService) List(ctx context.Context, regSet *registry.Set, now time.Time, days int) ([]UpcomingEntry, error) {
	if regSet == nil {
		return nil, errxtrace.Wrap("upcoming: registry set is required", registry.ErrFieldRequired)
	}
	days = min(max(days, 1), UpcomingMaxDays)
	n := now.UTC()
	end := models.Date(time.Date(n.Year(), n.Month(), n.Day()+days, 0, 0, 0, 0, time.UTC).Format("2006-01-02"))

	var entries []UpcomingEntry
	collectors := []func(context.Context, *registry.Set, time.Time, models.Date) ([]UpcomingEntry, error){
		upcomingReminders,
		upcomingWarranties,
		upcomingLoans,
		upcomingMaintenance,
	}
	for _, collect := range collectors {
		part, err := collect(ctx, regSet, now, end)
		if err != nil {
			return nil, err
		}
		entries = append(entries, part...)
	}

	entries, err := attachCommodities(ctx, regSet.CommodityRegistry, entries)
	if err != nil {
		return nil, err
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Date != entries[j].Date {
			return entries[i].Date < entries[j].Date
		}
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		return entries[i].Title < entries[j].Title
	})
	return entries, nil
}

// attachCommodities resolves the items of entries that do not carry one
// yet in a single batched read, and drops entries whose item is not
// active.
func attachCommodities(ctx context.Context, reg registry.CommodityRegistry, entries []UpcomingEntry) ([]UpcomingEntry, error) {
	var ids []string
	for _, e := range entries {
		if e.Commodity == nil {
			ids = append(ids, e.CommodityID)
		}
	}
	byID := make(map[string]*models.Commodity, len(ids))
	if len(ids) > 0 {
		items, err := reg.GetMany(ctx, ids)
		if err != nil {
			return nil, errxtrace.Wrap("upcoming: load items", err)
		}
		for _, c := range items {
			byID[c.ID] = c
		}
	}
	out := entries[:0]
	for _, e := range entries {
		if e.Commodity == nil {
			e.Commodity = byID[e.CommodityID]
		}
		if !ReminderTargetActive(e.Commodity) {
			continue
		}
		out = append(out, e)
	}
	return out, nil
}

func upcomingReminders(ctx context.Context, regSet *registry.Set, now time.Time, end models.Date) ([]UpcomingEntry, error) {
	reminders, err := regSet.CommodityReminderRegistry.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("upcoming: list reminders", err)
	}
	var out []UpcomingEntry
	for _, r := range reminders {
		next, ok := r.NextOccurrence(now)
		if !ok || next > end {
			continue
		}
		out = append(out, UpcomingEntry{
			Kind:        UpcomingKindReminder,
			Date:        next,
			DaysLeft:    models.DaysUntil(next, now),
			Title:       r.Title,
			SourceID:    r.ID,
			CommodityID: r.CommodityID,
		})
	}
	return out, nil
}

func upcomingWarranties(ctx context.Context, regSet *registry.Set, now time.Time, end models.Date) ([]UpcomingEntry, error) {
	// WarrantyExpiresBefore is exclusive, so pass the day after the window.
	before := end.ToTime().AddDate(0, 0, 1).Format("2006-01-02")
	items, _, err := regSet.CommodityRegistry.ListPaginated(ctx, 0, upcomingPerKindLimit, registry.CommodityListOptions{
		WarrantyStatuses: []registry.WarrantyStatusFilter{
			registry.WarrantyStatusFilterActive,
			registry.WarrantyStatusFilterExpiring,
		},
		WarrantyExpiresBefore: before,
		WarrantyNow:           now,
	})
	if err != nil {
		return nil, errxtrace.Wrap("upcoming: list warranties", err)
	}
	out := make([]UpcomingEntry, 0, len(items))
	for _, c := range items {
		if c.WarrantyExpiresAt == nil || *c.WarrantyExpiresAt == "" {
			continue
		}
		out = append(out, UpcomingEntry{
			Kind:        UpcomingKindWarranty,
			Date:        *c.WarrantyExpiresAt,
			DaysLeft:    models.DaysUntil(*c.WarrantyExpiresAt, now),
			SourceID:    c.ID,
			CommodityID: c.ID,
			Commodity:   c,
		})
	}
	return out, nil
}

func upcomingLoans(ctx context.Context, regSet *registry.Set, now time.Time, end models.Date) ([]UpcomingEntry, error) {
	loans, _, err := regSet.CommodityLoanRegistry.ListPaginated(ctx, 0, upcomingPerKindLimit, registry.LoanListOptions{
		State: registry.LoanStateOpen,
		Now:   now,
	})
	if err != nil {
		return nil, errxtrace.Wrap("upcoming: list loans", err)
	}
	var out []UpcomingEntry
	for _, l := range loans {
		if l.DueBackAt == nil || *l.DueBackAt == "" || *l.DueBackAt > end {
			continue
		}
		out = append(out, UpcomingEntry{
			Kind:        UpcomingKindLoan,
			Date:        *l.DueBackAt,
			DaysLeft:    models.DaysUntil(*l.DueBackAt, now),
			Title:       l.BorrowerName,
			SourceID:    l.ID,
			CommodityID: l.CommodityID,
		})
	}
	return out, nil
}

func upcomingMaintenance(ctx context.Context, regSet *registry.Set, now time.Time, end models.Date) ([]UpcomingEntry, error) {
	schedules, _, err := regSet.MaintenanceScheduleRegistry.ListPaginated(ctx, 0, upcomingPerKindLimit, registry.MaintenanceListOptions{
		DueBefore:   string(end),
		EnabledOnly: true,
	})
	if err != nil {
		return nil, errxtrace.Wrap("upcoming: list maintenance", err)
	}
	var out []UpcomingEntry
	for _, m := range schedules {
		if m.NextDueAt == "" {
			continue
		}
		out = append(out, UpcomingEntry{
			Kind:        UpcomingKindMaintenance,
			Date:        m.NextDueAt,
			DaysLeft:    models.DaysUntil(m.NextDueAt, now),
			Title:       m.Title,
			SourceID:    m.ID,
			CommodityID: m.CommodityID,
		})
	}
	return out, nil
}
