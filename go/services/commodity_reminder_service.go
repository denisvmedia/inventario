package services

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// ReminderTargetActive reports whether reminders on the item may still be
// sent and listed. Every path that sends or lists item reminders asks this
// one function, so a future trash state (#2799) only has to be added here.
// A nil commodity is a deleted one.
func ReminderTargetActive(c *models.Commodity) bool {
	return c != nil
}

// CommodityReminderService holds the write rules for item reminders
// (#2805): defaults for the repeat rule, lead times stored largest first,
// and field validation before the registry fills in tenant, group and
// author.
type CommodityReminderService struct {
	factorySet *registry.FactorySet
}

func NewCommodityReminderService(factorySet *registry.FactorySet) *CommodityReminderService {
	return &CommodityReminderService{factorySet: factorySet}
}

// normalizeReminder fills in the repeat defaults, drops duplicate lead
// times and orders them largest first.
func normalizeReminder(r *models.CommodityReminder) {
	if r.RepeatUnit == "" {
		r.RepeatUnit = models.ReminderRepeatNone
	}
	if r.RepeatUnit == models.ReminderRepeatNone || r.RepeatInterval == 0 {
		r.RepeatInterval = 1
	}
	r.NormalizeLeadTimes()
}

// Create validates and stores a new reminder for reminder.CommodityID.
func (s *CommodityReminderService) Create(ctx context.Context, reminder models.CommodityReminder) (*models.CommodityReminder, error) {
	normalizeReminder(&reminder)
	if err := reminder.ValidateFields(ctx); err != nil {
		return nil, errxtrace.Wrap("invalid reminder", err)
	}

	reg, err := s.factorySet.CommodityReminderRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity reminder registry", err)
	}
	created, err := reg.Create(ctx, reminder)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity reminder", err)
	}
	return created, nil
}

// CommodityReminderUpdate is the patch payload for Update. A nil field is
// left unchanged.
type CommodityReminderUpdate struct {
	Title            *string
	DueOn            *models.Date
	RemindDaysBefore []int
	RepeatUnit       *models.ReminderRepeatUnit
	RepeatInterval   *int
}

// Update applies a patch to the reminder with the given id. Send records
// need no reset: they are keyed by occurrence date, so a new date opens new
// emails, and a changed lead time is matched against what was already sent
// for the current occurrence.
func (s *CommodityReminderService) Update(ctx context.Context, id string, patch CommodityReminderUpdate) (*models.CommodityReminder, error) {
	reg, err := s.factorySet.CommodityReminderRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity reminder registry", err)
	}
	current, err := reg.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to fetch commodity reminder", err)
	}

	if patch.Title != nil {
		current.Title = *patch.Title
	}
	if patch.DueOn != nil {
		current.DueOn = *patch.DueOn
	}
	if patch.RemindDaysBefore != nil {
		current.RemindDaysBefore = patch.RemindDaysBefore
	}
	if patch.RepeatUnit != nil {
		current.RepeatUnit = *patch.RepeatUnit
	}
	if patch.RepeatInterval != nil {
		current.RepeatInterval = *patch.RepeatInterval
	}
	normalizeReminder(current)
	if err := current.ValidateFields(ctx); err != nil {
		return nil, errxtrace.Wrap("invalid reminder", err)
	}

	updated, err := reg.Update(ctx, *current)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update commodity reminder", err)
	}
	return updated, nil
}
