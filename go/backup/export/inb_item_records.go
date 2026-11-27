//go:build !legacy_xml_backup

package export

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// itemRecords are the records of one item that an archive carries with it
// (format 3.9).
type itemRecords struct {
	loans     []*models.CommodityLoan
	services  []*models.CommodityService
	schedules []*models.MaintenanceSchedule
	links     []*models.SupplyLink
	reminders []*models.CommodityReminder
}

// recordLister is the read every item record registry has.
type recordLister[T any] interface {
	List(ctx context.Context) ([]*T, error)
}

// listRecords lists every row of one item record registry through the
// RLS-scoped user registry create makes.
func listRecords[T any, R recordLister[T]](ctx context.Context, create func(context.Context) (R, error), what string) ([]*T, error) {
	reg, err := create(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create "+what+" registry", err)
	}
	rows, err := reg.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list "+what, err)
	}
	return rows, nil
}

// loadItemRecords lists the group's loans, service records, maintenance
// schedules, supply links and reminders once each and indexes them by item.
// The group-wide reads leave out the records of items in Trash, which the
// archive does not carry either. The members who recorded them are looked up
// once each, so planning an item needs no read.
func (b *inbBuilder) loadItemRecords() error {
	b.recordsByCommodityID = map[string]*itemRecords{}
	b.authorUUIDByID = map[string]string{}
	of := func(commodityID string) *itemRecords {
		r := b.recordsByCommodityID[commodityID]
		if r == nil {
			r = &itemRecords{}
			b.recordsByCommodityID[commodityID] = r
		}
		return r
	}
	authors := map[string]bool{}
	fs := b.svc.factorySet
	if fs.CommodityLoanRegistryFactory != nil {
		rows, err := listRecords[models.CommodityLoan](b.ctx, fs.CommodityLoanRegistryFactory.CreateUserRegistry, "loans")
		if err != nil {
			return err
		}
		for _, r := range rows {
			of(r.CommodityID).loans = append(of(r.CommodityID).loans, r)
			authors[r.CreatedByUserID] = true
		}
	}
	if fs.CommodityServiceRegistryFactory != nil {
		rows, err := listRecords[models.CommodityService](b.ctx, fs.CommodityServiceRegistryFactory.CreateUserRegistry, "service records")
		if err != nil {
			return err
		}
		for _, r := range rows {
			of(r.CommodityID).services = append(of(r.CommodityID).services, r)
			authors[r.CreatedByUserID] = true
		}
	}
	if fs.MaintenanceScheduleRegistryFactory != nil {
		rows, err := listRecords[models.MaintenanceSchedule](b.ctx, fs.MaintenanceScheduleRegistryFactory.CreateUserRegistry, "maintenance schedules")
		if err != nil {
			return err
		}
		for _, r := range rows {
			of(r.CommodityID).schedules = append(of(r.CommodityID).schedules, r)
			authors[r.CreatedByUserID] = true
		}
	}
	if err := b.loadSupplyLinksAndReminders(of, authors); err != nil {
		return err
	}
	return b.loadAuthors(authors)
}

// loadSupplyLinksAndReminders is the second half of loadItemRecords.
func (b *inbBuilder) loadSupplyLinksAndReminders(of func(string) *itemRecords, authors map[string]bool) error {
	fs := b.svc.factorySet
	if fs.SupplyLinkRegistryFactory != nil {
		rows, err := listRecords[models.SupplyLink](b.ctx, fs.SupplyLinkRegistryFactory.CreateUserRegistry, "supply links")
		if err != nil {
			return err
		}
		for _, r := range rows {
			of(r.CommodityID).links = append(of(r.CommodityID).links, r)
			authors[r.CreatedByUserID] = true
		}
	}
	if fs.CommodityReminderRegistryFactory != nil {
		rows, err := listRecords[models.CommodityReminder](b.ctx, fs.CommodityReminderRegistryFactory.CreateUserRegistry, "reminders")
		if err != nil {
			return err
		}
		for _, r := range rows {
			of(r.CommodityID).reminders = append(of(r.CommodityID).reminders, r)
			authors[r.CreatedByUserID] = true
		}
	}
	return nil
}

// loadAuthors resolves the DB id of every member who recorded one of the
// records to the member's UUID, the identity a restore matches the members
// of its group by. An account that is gone resolves to no UUID.
func (b *inbBuilder) loadAuthors(authors map[string]bool) error {
	users := b.svc.factorySet.UserRegistry
	if users == nil {
		return nil
	}
	for id := range authors {
		if id == "" {
			continue
		}
		user, err := users.Get(b.ctx, id)
		switch {
		case errors.Is(err, registry.ErrNotFound):
			continue
		case err != nil:
			return errxtrace.Wrap("failed to look up the member who recorded an item record", err, errx.Attrs("user_id", id))
		}
		b.authorUUIDByID[id] = user.UUID
	}
	return nil
}

// byCreation orders records oldest first, the UUID breaking a tie.
func byCreation(aCreated, bCreated time.Time, aUUID, bUUID string) int {
	return cmp.Or(aCreated.Compare(bCreated), cmp.Compare(aUUID, bUUID))
}

// archiveItemRecords puts com's records on inbCom.
func (b *inbBuilder) archiveItemRecords(com *models.Commodity, inbCom *INBCommodity) {
	recs := b.recordsByCommodityID[com.ID]
	if recs == nil {
		return
	}
	slices.SortFunc(recs.loans, func(x, y *models.CommodityLoan) int { return byCreation(x.CreatedAt, y.CreatedAt, x.UUID, y.UUID) })
	for _, r := range recs.loans {
		inbCom.Loans = append(inbCom.Loans, INBLoan{
			ID:                  r.UUID,
			AuthorID:            b.authorUUIDByID[r.CreatedByUserID],
			BorrowerName:        r.BorrowerName,
			BorrowerContact:     r.BorrowerContact,
			BorrowerNote:        r.BorrowerNote,
			LentAt:              string(r.LentAt),
			DueBackAt:           dateString(r.DueBackAt),
			ReturnedAt:          dateString(r.ReturnedAt),
			ReminderSentDueSoon: r.ReminderSentDueSoon,
			ReminderSentOverdue: r.ReminderSentOverdue,
		})
	}
	slices.SortFunc(recs.services, func(x, y *models.CommodityService) int { return byCreation(x.CreatedAt, y.CreatedAt, x.UUID, y.UUID) })
	for _, r := range recs.services {
		rec := INBServiceRecord{
			ID:                  r.UUID,
			AuthorID:            b.authorUUIDByID[r.CreatedByUserID],
			ProviderName:        r.ProviderName,
			ProviderContact:     r.ProviderContact,
			Reason:              r.Reason,
			SentAt:              string(r.SentAt),
			ExpectedReturnAt:    dateString(r.ExpectedReturnAt),
			ReturnedAt:          dateString(r.ReturnedAt),
			CostCurrency:        r.CostCurrency,
			ReminderSentDueSoon: r.ReminderSentDueSoon,
			ReminderSentOverdue: r.ReminderSentOverdue,
		}
		if r.HasCost() {
			rec.CostAmount = r.CostAmount.String()
		}
		inbCom.ServiceRecords = append(inbCom.ServiceRecords, rec)
	}
	b.archiveSchedulesLinksAndReminders(recs, inbCom)
}

// archiveSchedulesLinksAndReminders is the second half of archiveItemRecords.
func (b *inbBuilder) archiveSchedulesLinksAndReminders(recs *itemRecords, inbCom *INBCommodity) {
	slices.SortFunc(recs.schedules, func(x, y *models.MaintenanceSchedule) int {
		return byCreation(x.CreatedAt, y.CreatedAt, x.UUID, y.UUID)
	})
	for _, r := range recs.schedules {
		inbCom.MaintenanceSchedules = append(inbCom.MaintenanceSchedules, INBMaintenanceSchedule{
			ID:           r.UUID,
			AuthorID:     b.authorUUIDByID[r.CreatedByUserID],
			Title:        r.Title,
			IntervalDays: r.IntervalDays,
			NextDueAt:    string(r.NextDueAt),
			LastDoneAt:   dateString(r.LastDoneAt),
			Notes:        r.Notes,
			Enabled:      r.Enabled,
		})
	}
	slices.SortFunc(recs.links, func(x, y *models.SupplyLink) int {
		return cmp.Or(cmp.Compare(x.SortOrder, y.SortOrder), byCreation(x.CreatedAt, y.CreatedAt, x.UUID, y.UUID))
	})
	for _, r := range recs.links {
		inbCom.SupplyLinks = append(inbCom.SupplyLinks, INBSupplyLink{
			ID:        r.UUID,
			AuthorID:  b.authorUUIDByID[r.CreatedByUserID],
			Label:     r.Label,
			URL:       r.URL,
			Notes:     r.Notes,
			SortOrder: r.SortOrder,
		})
	}
	slices.SortFunc(recs.reminders, func(x, y *models.CommodityReminder) int {
		return byCreation(x.CreatedAt, y.CreatedAt, x.UUID, y.UUID)
	})
	for _, r := range recs.reminders {
		inbCom.Reminders = append(inbCom.Reminders, INBReminder{
			ID:               r.UUID,
			AuthorID:         b.authorUUIDByID[r.CreatedByUserID],
			Title:            r.Title,
			DueOn:            string(r.DueOn),
			RemindDaysBefore: slices.Clone([]int(r.RemindDaysBefore)),
			RepeatUnit:       string(r.RepeatUnit),
			RepeatInterval:   r.RepeatInterval,
		})
	}
}
