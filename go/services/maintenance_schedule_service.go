package services

import (
	"context"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// MaintenanceScheduleService coordinates CRUD + the "mark done" action
// on top of MaintenanceScheduleRegistry (#1368). Mirrors the
// commodity-loan / commodity-service services: thin layer on top of a
// per-row registry, with the small domain rules ("done advances
// next_due_at") enforced here rather than at the SQL layer.
type MaintenanceScheduleService struct {
	factorySet *registry.FactorySet
}

func NewMaintenanceScheduleService(factorySet *registry.FactorySet) *MaintenanceScheduleService {
	return &MaintenanceScheduleService{factorySet: factorySet}
}

// Create persists a new schedule, defaulting NextDueAt to
// `today + interval_days` when the caller passes an empty value. The
// commodity must be trackable (count == 1, #1554) — bundles cannot
// carry per-instance maintenance.
func (s *MaintenanceScheduleService) Create(ctx context.Context, schedule models.MaintenanceSchedule, now time.Time) (*models.MaintenanceSchedule, error) {
	if err := EnsureCommodityTrackable(ctx, s.factorySet, schedule.CommodityID); err != nil {
		return nil, err
	}

	if string(schedule.NextDueAt) == "" {
		schedule.NextDueAt = schedule.AdvanceFromDone(now)
	}

	reg, err := s.factorySet.MaintenanceScheduleRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create maintenance schedule registry", err)
	}

	created, err := reg.Create(ctx, schedule)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create maintenance schedule", err)
	}
	return created, nil
}

// MaintenanceScheduleUpdate is the per-field patch payload for Update.
// Each pointer field uses the "nil = leave unchanged, non-nil = set to
// this value" convention. The clear-flag pair on LastDoneAt mirrors
// LoanUpdate.ClearDueBackAt.
type MaintenanceScheduleUpdate struct {
	Title          *string
	IntervalDays   *int
	NextDueAt      *models.Date
	LastDoneAt     models.PDate
	ClearLastDone  bool
	Notes          *string
	Enabled        *bool
}

// Update applies partial updates to an existing schedule.
func (s *MaintenanceScheduleService) Update(ctx context.Context, id string, patch MaintenanceScheduleUpdate) (*models.MaintenanceSchedule, error) {
	reg, err := s.factorySet.MaintenanceScheduleRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create maintenance schedule registry", err)
	}

	current, err := reg.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to fetch maintenance schedule", err)
	}

	if patch.Title != nil {
		current.Title = *patch.Title
	}
	if patch.IntervalDays != nil {
		current.IntervalDays = *patch.IntervalDays
	}
	if patch.NextDueAt != nil {
		current.NextDueAt = *patch.NextDueAt
	}
	switch {
	case patch.ClearLastDone:
		current.LastDoneAt = nil
	case patch.LastDoneAt != nil:
		current.LastDoneAt = patch.LastDoneAt
	}
	if patch.Notes != nil {
		current.Notes = *patch.Notes
	}
	if patch.Enabled != nil {
		current.Enabled = *patch.Enabled
	}

	updated, err := reg.Update(ctx, *current)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update maintenance schedule", err)
	}

	// Editing the schedule resets the idempotency rows so the next
	// cycle gets a clean slate — the user's "I just changed the
	// cadence / due date" intent implies they want fresh reminders.
	if _, derr := s.factorySet.MaintenanceReminderRegistry.DeleteBySchedule(ctx, updated.ID); derr != nil {
		return updated, errxtrace.Wrap("failed to reset maintenance reminders", derr)
	}

	return updated, nil
}

// MarkDone advances NextDueAt by IntervalDays from the supplied done
// date (or `now` when zero), records LastDoneAt, and clears the
// idempotency rows so the next cycle's reminders fire fresh.
//
// `doneDate` is the user-supplied "I did this on …" date — typically
// today, but may be a recent past date (the user logging a maintenance
// they performed earlier and forgot to tick off).
func (s *MaintenanceScheduleService) MarkDone(ctx context.Context, id string, doneDate models.PDate, now time.Time) (*models.MaintenanceSchedule, error) {
	reg, err := s.factorySet.MaintenanceScheduleRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create maintenance schedule registry", err)
	}

	current, err := reg.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to fetch maintenance schedule", err)
	}

	// Resolve done date: explicit PDate wins, otherwise the server
	// clock. Stored alongside the recomputed NextDueAt.
	var doneTime time.Time
	if doneDate != nil && string(*doneDate) != "" {
		doneTime = doneDate.ToTime()
	}
	if doneTime.IsZero() {
		doneTime = now
	}

	current.NextDueAt = current.AdvanceFromDone(doneTime)
	doneDateValue := models.Date(doneTime.UTC().Format("2006-01-02"))
	current.LastDoneAt = &doneDateValue

	updated, err := reg.Update(ctx, *current)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update maintenance schedule", err)
	}

	// Reset idempotency rows — the new NextDueAt opens a fresh
	// 14/7/1/overdue cycle.
	if _, derr := s.factorySet.MaintenanceReminderRegistry.DeleteBySchedule(ctx, updated.ID); derr != nil {
		return updated, errxtrace.Wrap("failed to reset maintenance reminders", derr)
	}

	return updated, nil
}
