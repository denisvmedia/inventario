package memory

import (
	"context"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

var _ registry.CommodityReminderSendRegistry = (*CommodityReminderSendRegistry)(nil)

type baseCommodityReminderSendRegistry = Registry[models.CommodityReminderSend, *models.CommodityReminderSend]

// CommodityReminderSendRegistry is the in-memory idempotency store of the
// item reminder worker. CreateOnce enforces the (reminder, occurrence, lead
// time) uniqueness under the registry lock.
type CommodityReminderSendRegistry struct {
	*baseCommodityReminderSendRegistry
}

func NewCommodityReminderSendRegistry() *CommodityReminderSendRegistry {
	return &CommodityReminderSendRegistry{
		baseCommodityReminderSendRegistry: NewRegistry[models.CommodityReminderSend, *models.CommodityReminderSend](),
	}
}

func validateReminderSendKey(reminderID string, occurrenceOn models.Date) error {
	if reminderID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ReminderID"))
	}
	if occurrenceOn == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "OccurrenceOn"))
	}
	return nil
}

func (r *CommodityReminderSendRegistry) HasSentAtOrBelow(_ context.Context, reminderID string, occurrenceOn models.Date, leadDays int) (bool, error) {
	if err := validateReminderSendKey(reminderID, occurrenceOn); err != nil {
		return false, err
	}
	r.lock.RLock()
	defer r.lock.RUnlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.ReminderID == reminderID && v.OccurrenceOn == occurrenceOn && v.LeadDays <= leadDays {
			return true, nil
		}
	}
	return false, nil
}

func (r *CommodityReminderSendRegistry) CreateOnce(_ context.Context, send models.CommodityReminderSend) (bool, error) {
	if err := validateReminderSendKey(send.ReminderID, send.OccurrenceOn); err != nil {
		return false, err
	}
	if send.SentAt.IsZero() {
		send.SentAt = time.Now()
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		v := pair.Value
		if v.ReminderID == send.ReminderID && v.OccurrenceOn == send.OccurrenceOn && v.LeadDays == send.LeadDays {
			return false, nil
		}
	}
	// The base Create would take the lock again, so set the ids here.
	row := send
	if row.ID == "" {
		row.ID = uuid.New().String()
	}
	if row.UUID == "" {
		row.UUID = uuid.New().String()
	}
	r.items.Set(row.ID, &row)
	return true, nil
}

func (r *CommodityReminderSendRegistry) Release(_ context.Context, reminderID string, occurrenceOn models.Date, leadDays int) error {
	if err := validateReminderSendKey(reminderID, occurrenceOn); err != nil {
		return err
	}
	r.deleteWhere(func(v *models.CommodityReminderSend) bool {
		return v.ReminderID == reminderID && v.OccurrenceOn == occurrenceOn && v.LeadDays == leadDays
	})
	return nil
}

func (r *CommodityReminderSendRegistry) DeleteByReminder(_ context.Context, reminderID string) (int, error) {
	return r.deleteWhere(func(v *models.CommodityReminderSend) bool {
		return v.ReminderID == reminderID
	}), nil
}

func (r *CommodityReminderSendRegistry) deleteWhere(match func(*models.CommodityReminderSend) bool) int {
	r.lock.Lock()
	defer r.lock.Unlock()
	var victims []string
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if match(pair.Value) {
			victims = append(victims, pair.Value.ID)
		}
	}
	for _, id := range victims {
		r.items.Delete(id)
	}
	return len(victims)
}
