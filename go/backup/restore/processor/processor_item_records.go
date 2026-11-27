//go:build !legacy_xml_backup

package processor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/backup/restore/security"
	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
)

// itemRecordsStep is the restore log step that counts the item records.
const itemRecordsStep = "Loans, service records, maintenance and reminders"

// ErrItemRecordsUnsupported is the error a record of an item gets when the
// server has no registry for it.
var ErrItemRecordsUnsupported = errx.NewSentinel("this server cannot store the records of an item")

// itemRecordTally counts the item records of a restore for its log step.
type itemRecordTally struct {
	restored, kept, failed int
}

// itemRecord is what restoreItemRecord needs of a record model.
type itemRecord interface {
	ValidateWithContext(ctx context.Context) error
	SetTenantID(tenantID string)
	SetGroupID(groupID string)
	SetCreatedByUserID(userID string)
}

// recordCreator is the write every item record registry has.
type recordCreator[T any] interface {
	Create(ctx context.Context, item T) (*T, error)
}

// applyCommodityStrategy applies the restore strategy to the archived item
// c and then restores its loans, service records, maintenance schedules,
// supply links and reminders. They come back with an item the restore
// created and only then: the records of an item the group already has stay
// as they are, whatever the archive says. existing is the group's item with
// the archived UUID, nil when there was none.
func (w *inbWalker) applyCommodityStrategy(ctx context.Context, commodity, existing *models.Commodity, c *types.INBCommodity) error {
	createdBefore := w.stats.CreatedCount
	if err := w.proc.applyStrategyForCommodityModel(ctx, commodity, existing, c.ID, w.stats, w.existing, w.idMapping, w.options); err != nil {
		return err
	}
	// The strategy counts a created item, in a dry run too, and nothing else.
	if w.stats.CreatedCount == createdBefore {
		w.keepItemRecords(c, existing)
		return nil
	}
	return w.restoreItemRecords(c)
}

// keepItemRecords counts the records of an archived item the restore did
// not create as kept, when the group has the item. An item that failed to
// restore reports its own error.
func (w *inbWalker) keepItemRecords(c *types.INBCommodity, existing *models.Commodity) {
	if n := c.RecordCount(); n > 0 && (existing != nil || w.existing.TrashedCommodities[c.ID] != "") {
		w.records.kept += n
		w.stats.SkippedCount += n
	}
}

// restoreItemRecords restores the records of the archived item c, which the
// restore just created. A record that fails is counted as an error and the
// rest go on; an unparseable cost is a corrupt archive and stops the
// restore, as an unparseable price of the item does.
func (w *inbWalker) restoreItemRecords(c *types.INBCommodity) error {
	n := c.RecordCount()
	if n == 0 {
		return nil
	}
	services := make([]*models.CommodityService, len(c.ServiceRecords))
	for i := range c.ServiceRecords {
		rec, err := c.ServiceRecords[i].ConvertToService("")
		if err != nil {
			return errx.Classify(
				errxtrace.Wrap("failed to convert a service record", err, errx.Attrs("commodity_id", c.ID, "service_record_id", c.ServiceRecords[i].ID)),
				ErrMalformedEntity,
			)
		}
		services[i] = rec
	}
	commodityID := w.idMapping.Commodities[c.ID]
	if w.options.DryRun {
		// A dry run creates no item; the UUID stands in for its id.
		commodityID = c.ID
	}
	fs := w.proc.factorySet
	if fs.CommodityLoanRegistryFactory == nil || fs.CommodityServiceRegistryFactory == nil || fs.MaintenanceScheduleRegistryFactory == nil ||
		fs.SupplyLinkRegistryFactory == nil || fs.CommodityReminderRegistryFactory == nil {
		for range n {
			w.countItemRecord(c.ID, "record", ErrItemRecordsUnsupported)
		}
		return nil
	}
	for i := range c.Loans {
		rec := c.Loans[i].ConvertToLoan(commodityID)
		w.countItemRecord(c.ID, "loan", restoreItemRecord(w, c.Loans[i].AuthorID, rec, fs.CommodityLoanRegistryFactory.CreateUserRegistry))
	}
	for i, rec := range services {
		rec.CommodityID = commodityID
		w.countItemRecord(c.ID, "service record", restoreItemRecord(w, c.ServiceRecords[i].AuthorID, rec, fs.CommodityServiceRegistryFactory.CreateUserRegistry))
	}
	for i := range c.MaintenanceSchedules {
		rec := c.MaintenanceSchedules[i].ConvertToSchedule(commodityID)
		w.countItemRecord(c.ID, "maintenance schedule", restoreItemRecord(w, c.MaintenanceSchedules[i].AuthorID, rec, fs.MaintenanceScheduleRegistryFactory.CreateUserRegistry))
	}
	for i := range c.SupplyLinks {
		rec := c.SupplyLinks[i].ConvertToSupplyLink(commodityID)
		w.countItemRecord(c.ID, "supply link", restoreItemRecord(w, c.SupplyLinks[i].AuthorID, rec, fs.SupplyLinkRegistryFactory.CreateUserRegistry))
	}
	for i := range c.Reminders {
		rec := c.Reminders[i].ConvertToReminder(commodityID)
		w.countItemRecord(c.ID, "reminder", restoreItemRecord(w, c.Reminders[i].AuthorID, rec, fs.CommodityReminderRegistryFactory.CreateUserRegistry))
	}
	return nil
}

// restoreItemRecord validates rec and creates it as the member recordAuthor
// picks for authorUUID. A dry run stops after the validation.
func restoreItemRecord[T any, P interface {
	*T
	itemRecord
}, R recordCreator[T]](w *inbWalker, authorUUID string, rec P, newRegistry func(context.Context) (R, error)) error {
	author, ctx, err := w.recordAuthor(authorUUID)
	if err != nil {
		return err
	}
	rec.SetTenantID(author.TenantID)
	rec.SetGroupID(appctx.GroupIDFromContext(ctx))
	rec.SetCreatedByUserID(author.ID)
	if err := rec.ValidateWithContext(ctx); err != nil {
		return errxtrace.Wrap("invalid record", err)
	}
	if w.options.DryRun {
		return nil
	}
	reg, err := newRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create the record registry", err)
	}
	if _, err := reg.Create(ctx, *rec); err != nil {
		return errxtrace.Wrap("failed to create the record", err)
	}
	return nil
}

// countItemRecord counts one record of the item with UUID itemUUID as
// restored, or as failed when err is set.
func (w *inbWalker) countItemRecord(itemUUID, what string, err error) {
	if err != nil {
		w.records.failed++
		w.stats.ErrorCount++
		w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("failed to restore a %s of item %s: %v", what, itemUUID, err))
		return
	}
	w.records.restored++
	w.stats.CreatedCount++
}

// recordAuthor returns the member a restored record is recorded as, with
// the context to write it in. That is the member who recorded it when they
// belong to the group the restore writes to, and the member running the
// restore otherwise: a loan's and a reminder's emails go to the member who
// recorded it, and someone outside the group must not get them.
func (w *inbWalker) recordAuthor(authorUUID string) (*models.User, context.Context, error) {
	restorer := appctx.UserFromContext(w.ctx)
	if restorer == nil {
		return nil, nil, security.ErrNoUserContext
	}
	if authorUUID == "" || authorUUID == restorer.UUID {
		return restorer, w.ctx, nil
	}
	if w.recordAuthors == nil {
		members, err := w.groupMembersByUUID(restorer)
		if err != nil {
			return nil, nil, err
		}
		w.recordAuthors = members
	}
	if author := w.recordAuthors[authorUUID]; author != nil {
		return author, appctx.WithUser(w.ctx, author), nil
	}
	return restorer, w.ctx, nil
}

// groupMembersByUUID returns the members of the group the restore writes to
// by user UUID, leaving out any account outside the restoring member's
// tenant.
func (w *inbWalker) groupMembersByUUID(restorer *models.User) (map[string]*models.User, error) {
	fs := w.proc.factorySet
	out := map[string]*models.User{}
	if fs.GroupMembershipRegistry == nil || fs.UserRegistry == nil {
		return out, nil
	}
	groupID := appctx.GroupIDFromContext(w.ctx)
	memberships, err := fs.GroupMembershipRegistry.ListByGroup(w.ctx, groupID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list the group's members", err, errx.Attrs("group_id", groupID))
	}
	for _, m := range memberships {
		user, err := fs.UserRegistry.Get(w.ctx, m.MemberUserID)
		if err != nil {
			slog.WarnContext(w.ctx, "Restore: a group member could not be looked up; records they made are recorded as the restoring member",
				"restore_operation_id", w.proc.restoreOperationID, "user_id", m.MemberUserID, "error", err)
			continue
		}
		if user.TenantID == restorer.TenantID {
			out[user.UUID] = user
		}
	}
	return out, nil
}

// writeItemRecordsStep writes the restore log step that counts the item
// records, when the archive carried any.
func (w *inbWalker) writeItemRecordsStep() {
	t := w.records
	if t.restored+t.kept+t.failed == 0 {
		return
	}
	result := models.RestoreStepResultSuccess
	if t.failed > 0 {
		result = models.RestoreStepResultError
	}
	w.proc.createRestoreStep(w.ctx, itemRecordsStep, result,
		fmt.Sprintf("%d restored, %d kept as they were, %d failed", t.restored, t.kept, t.failed))
}
