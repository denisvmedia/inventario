package export

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
)

// This file collects the records a user created on items: loans, maintenance
// schedules, service visits, reminders, supply links, history entries, and
// the user's drafts and saved views. Each is listed through the group's user
// registry and kept when the user created it.

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func dateString(d models.PDate) string {
	if d == nil {
		return ""
	}
	return string(*d)
}

// createdBy keeps the rows the user created, in a stable order.
func createdBy[T any](rows []*T, author func(*T) string, userID string, order func(a, b *T) int) []*T {
	out := make([]*T, 0, len(rows))
	for _, r := range rows {
		if r != nil && author(r) == userID {
			out = append(out, r)
		}
	}
	slices.SortFunc(out, order)
	return out
}

func (b *personalBuilder) planLoans(ctx context.Context, l *groupLookups, doc *PersonalGroupDocument) error {
	if b.fs.CommodityLoanRegistryFactory == nil {
		return nil
	}
	reg, err := b.fs.CommodityLoanRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create loan registry", err)
	}
	rows, err := reg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list loans", err)
	}
	for _, r := range createdBy(rows, func(r *models.CommodityLoan) string { return r.CreatedByUserID }, b.user.ID, func(x, y *models.CommodityLoan) int {
		return cmp.Or(x.CreatedAt.Compare(y.CreatedAt), cmp.Compare(x.UUID, y.UUID))
	}) {
		doc.Loans = append(doc.Loans, PersonalLoan{
			PersonalItemRef: l.itemRef(r.CommodityID),
			BorrowerName:    r.BorrowerName,
			BorrowerContact: r.BorrowerContact,
			BorrowerNote:    r.BorrowerNote,
			LentAt:          string(r.LentAt),
			DueBackAt:       dateString(r.DueBackAt),
			ReturnedAt:      dateString(r.ReturnedAt),
			CreatedAt:       formatTime(r.CreatedAt),
			UpdatedAt:       formatTime(r.UpdatedAt),
		})
	}
	b.stats.LoanCount += len(doc.Loans)
	return nil
}

func (b *personalBuilder) planMaintenanceSchedules(ctx context.Context, l *groupLookups, doc *PersonalGroupDocument) error {
	if b.fs.MaintenanceScheduleRegistryFactory == nil {
		return nil
	}
	reg, err := b.fs.MaintenanceScheduleRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create maintenance schedule registry", err)
	}
	rows, err := reg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list maintenance schedules", err)
	}
	for _, r := range createdBy(rows, func(r *models.MaintenanceSchedule) string { return r.CreatedByUserID }, b.user.ID, func(x, y *models.MaintenanceSchedule) int {
		return cmp.Or(x.CreatedAt.Compare(y.CreatedAt), cmp.Compare(x.UUID, y.UUID))
	}) {
		doc.MaintenanceSchedules = append(doc.MaintenanceSchedules, PersonalMaintenanceSchedule{
			PersonalItemRef: l.itemRef(r.CommodityID),
			Title:           r.Title,
			IntervalDays:    r.IntervalDays,
			NextDueAt:       string(r.NextDueAt),
			LastDoneAt:      dateString(r.LastDoneAt),
			Notes:           r.Notes,
			Enabled:         r.Enabled,
			CreatedAt:       formatTime(r.CreatedAt),
			UpdatedAt:       formatTime(r.UpdatedAt),
		})
	}
	b.stats.MaintenanceScheduleCount += len(doc.MaintenanceSchedules)
	return nil
}

func (b *personalBuilder) planServiceRecords(ctx context.Context, l *groupLookups, doc *PersonalGroupDocument) error {
	if b.fs.CommodityServiceRegistryFactory == nil {
		return nil
	}
	reg, err := b.fs.CommodityServiceRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create service record registry", err)
	}
	rows, err := reg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list service records", err)
	}
	for _, r := range createdBy(rows, func(r *models.CommodityService) string { return r.CreatedByUserID }, b.user.ID, func(x, y *models.CommodityService) int {
		return cmp.Or(x.CreatedAt.Compare(y.CreatedAt), cmp.Compare(x.UUID, y.UUID))
	}) {
		rec := PersonalServiceRecord{
			PersonalItemRef:  l.itemRef(r.CommodityID),
			ProviderName:     r.ProviderName,
			ProviderContact:  r.ProviderContact,
			Reason:           r.Reason,
			SentAt:           string(r.SentAt),
			ExpectedReturnAt: dateString(r.ExpectedReturnAt),
			ReturnedAt:       dateString(r.ReturnedAt),
			CostCurrency:     r.CostCurrency,
			CreatedAt:        formatTime(r.CreatedAt),
			UpdatedAt:        formatTime(r.UpdatedAt),
		}
		if !r.CostAmount.IsZero() || r.CostCurrency != "" {
			rec.CostAmount = r.CostAmount.String()
		}
		doc.ServiceRecords = append(doc.ServiceRecords, rec)
	}
	b.stats.ServiceRecordCount += len(doc.ServiceRecords)
	return nil
}

func (b *personalBuilder) planReminders(ctx context.Context, l *groupLookups, doc *PersonalGroupDocument) error {
	if b.fs.CommodityReminderRegistryFactory == nil {
		return nil
	}
	reg, err := b.fs.CommodityReminderRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create reminder registry", err)
	}
	rows, err := reg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list reminders", err)
	}
	for _, r := range createdBy(rows, func(r *models.CommodityReminder) string { return r.CreatedByUserID }, b.user.ID, func(x, y *models.CommodityReminder) int {
		return cmp.Or(x.CreatedAt.Compare(y.CreatedAt), cmp.Compare(x.UUID, y.UUID))
	}) {
		doc.Reminders = append(doc.Reminders, PersonalReminder{
			PersonalItemRef:  l.itemRef(r.CommodityID),
			Title:            r.Title,
			DueOn:            string(r.DueOn),
			RemindDaysBefore: []int(r.RemindDaysBefore),
			RepeatUnit:       string(r.RepeatUnit),
			RepeatInterval:   r.RepeatInterval,
			CreatedAt:        formatTime(r.CreatedAt),
			UpdatedAt:        formatTime(r.UpdatedAt),
		})
	}
	b.stats.ReminderCount += len(doc.Reminders)
	return nil
}

func (b *personalBuilder) planSupplyLinks(ctx context.Context, l *groupLookups, doc *PersonalGroupDocument) error {
	if b.fs.SupplyLinkRegistryFactory == nil {
		return nil
	}
	reg, err := b.fs.SupplyLinkRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create supply link registry", err)
	}
	rows, err := reg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list supply links", err)
	}
	for _, r := range createdBy(rows, func(r *models.SupplyLink) string { return r.CreatedByUserID }, b.user.ID, func(x, y *models.SupplyLink) int {
		return cmp.Or(x.CreatedAt.Compare(y.CreatedAt), cmp.Compare(x.UUID, y.UUID))
	}) {
		doc.SupplyLinks = append(doc.SupplyLinks, PersonalSupplyLink{
			PersonalItemRef: l.itemRef(r.CommodityID),
			Label:           r.Label,
			URL:             r.URL,
			Notes:           r.Notes,
			CreatedAt:       formatTime(r.CreatedAt),
			UpdatedAt:       formatTime(r.UpdatedAt),
		})
	}
	b.stats.SupplyLinkCount += len(doc.SupplyLinks)
	return nil
}

// planDrafts keeps the user's open drafts. A promoted draft is an item now,
// which the archive carries already.
func (b *personalBuilder) planDrafts(_ context.Context, l *groupLookups, doc *PersonalGroupDocument) error {
	drafts := make([]*models.CommodityDraft, 0, len(l.drafts))
	for _, d := range l.drafts {
		if d.IsOpen() && d.CreatedByUserID == b.user.ID {
			drafts = append(drafts, d)
		}
	}
	slices.SortFunc(drafts, func(x, y *models.CommodityDraft) int {
		return cmp.Or(x.CreatedAt.Compare(y.CreatedAt), cmp.Compare(x.UUID, y.UUID))
	})
	for _, d := range drafts {
		doc.Drafts = append(doc.Drafts, l.personalDraft(d))
	}
	b.stats.DraftCount += len(doc.Drafts)
	return nil
}

func (l *groupLookups) personalDraft(d *models.CommodityDraft) PersonalDraft {
	values := make(map[string]any, len(d.Values))
	maps.Copy(values, d.Values)
	out := PersonalDraft{
		ID:        d.UUID,
		CreatedAt: formatTime(d.CreatedAt),
		UpdatedAt: formatTime(d.UpdatedAt),
	}
	if areaID, ok := values["area_id"].(string); ok && areaID != "" {
		out.Location, out.Area = l.place(&areaID)
	}
	if sellerID, ok := values["seller_id"].(string); ok && sellerID != "" {
		out.Seller = l.sellerName[sellerID]
	}
	if entries, ok := values["custom_fields"].(map[string]any); ok {
		out.CustomFields = l.draftCustomValues(entries)
	}
	delete(values, "area_id")
	delete(values, "seller_id")
	delete(values, "custom_fields")
	out.Values = values
	return out
}

// draftCustomValues returns a draft's custom field entries by field name, an
// item link naming the linked item's UUID.
func (l *groupLookups) draftCustomValues(entries map[string]any) []PersonalCustomValue {
	out := []PersonalCustomValue{}
	for id, entry := range entries {
		d := l.fields[id]
		if d == nil || entry == nil {
			continue
		}
		if d.Type == models.CustomFieldTypeItemLink {
			entry = models.MapDraftCustomFieldReference(entry, func(itemID string) string {
				if item := l.items[itemID]; item != nil {
					return item.UUID
				}
				return ""
			})
			if entry == nil {
				continue
			}
		}
		raw, err := encodeJSONMember(id, entry)
		if err != nil {
			continue
		}
		out = append(out, PersonalCustomValue{Name: d.Name, Type: string(d.Type), Value: raw})
	}
	if len(out) == 0 {
		return nil
	}
	slices.SortFunc(out, func(x, y PersonalCustomValue) int { return cmp.Compare(x.Name, y.Name) })
	return out
}

// planSavedViews keeps the views the user created and where the user shows
// any view they see, also one another member shared.
func (b *personalBuilder) planSavedViews(ctx context.Context, _ *groupLookups, doc *PersonalGroupDocument) error {
	if b.fs.SavedViewRegistryFactory == nil {
		return nil
	}
	reg, err := b.fs.SavedViewRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create saved view registry", err)
	}
	views, err := reg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list saved views", err)
	}
	viewUUID := make(map[string]string, len(views))
	for _, v := range views {
		viewUUID[v.ID] = v.UUID
	}
	for _, v := range createdBy(views, func(v *models.SavedView) string { return v.CreatedByUserID }, b.user.ID, func(x, y *models.SavedView) int {
		return cmp.Or(cmp.Compare(x.SortOrder, y.SortOrder), cmp.Compare(x.UUID, y.UUID))
	}) {
		doc.SavedViews = append(doc.SavedViews, PersonalSavedView{
			ID:           v.UUID,
			Name:         v.Name,
			Icon:         string(v.Icon),
			Entity:       string(v.Entity),
			Query:        v.Query,
			QueryVersion: v.QueryVersion,
			Layout:       string(v.Layout),
			Shared:       v.Shared,
			SortOrder:    v.SortOrder,
			CreatedAt:    formatTime(v.CreatedAt),
			UpdatedAt:    formatTime(v.UpdatedAt),
		})
	}
	b.stats.SavedViewCount += len(doc.SavedViews)

	placements, err := reg.Placements(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list saved view placements", err)
	}
	for viewID, p := range placements {
		if p.UserID != "" && p.UserID != b.user.ID {
			continue
		}
		id := viewUUID[viewID]
		if id == "" {
			continue
		}
		doc.SavedViewPlacements = append(doc.SavedViewPlacements, PersonalSavedViewPlacement{
			ViewID:          id,
			ShowInSidebar:   p.ShowInSidebar,
			ShowOnDashboard: p.ShowOnDashboard,
		})
	}
	slices.SortFunc(doc.SavedViewPlacements, func(x, y PersonalSavedViewPlacement) int { return cmp.Compare(x.ViewID, y.ViewID) })
	return nil
}

// planHistory keeps the history entries the user made, also on items other
// members created.
func (b *personalBuilder) planHistory(ctx context.Context, l *groupLookups, doc *PersonalGroupDocument) error {
	if b.fs.CommodityEventRegistryFactory == nil {
		return nil
	}
	reg, err := b.fs.CommodityEventRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create history registry", err)
	}
	rows, err := reg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list history", err)
	}
	for _, r := range createdBy(rows, func(r *models.CommodityEvent) string { return r.CreatedByUserID }, b.user.ID, func(x, y *models.CommodityEvent) int {
		return cmp.Or(x.OccurredAt.Compare(y.OccurredAt), cmp.Compare(x.UUID, y.UUID))
	}) {
		doc.History = append(doc.History, PersonalHistoryEntry{
			PersonalItemRef: l.itemRef(r.CommodityID),
			Kind:            string(r.Kind),
			OccurredAt:      formatTime(r.OccurredAt),
			Before:          map[string]any(r.Before),
			After:           map[string]any(r.After),
			Note:            r.Note,
		})
	}
	b.stats.HistoryEntryCount += len(doc.History)
	return nil
}
