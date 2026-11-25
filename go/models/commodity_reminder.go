package models

import (
	"context"
	"slices"
	"time"

	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*CommodityReminder)(nil)
	_ validation.ValidatableWithContext = (*CommodityReminder)(nil)
	_ TenantGroupAwareIDable            = (*CommodityReminder)(nil)
)

// ReminderRepeatUnit is the unit of a reminder's repeat rule. A reminder
// repeats every RepeatInterval units; ReminderRepeatNone means it fires for
// one date only. Values are part of the API surface.
type ReminderRepeatUnit string

const (
	// ReminderRepeatNone marks a one-off reminder.
	ReminderRepeatNone ReminderRepeatUnit = "none"
	// ReminderRepeatDay repeats every N days.
	ReminderRepeatDay ReminderRepeatUnit = "day"
	// ReminderRepeatWeek repeats every N weeks.
	ReminderRepeatWeek ReminderRepeatUnit = "week"
	// ReminderRepeatMonth repeats every N months on the day of the month of
	// DueOn, moved back to the month's last day when the month is shorter.
	ReminderRepeatMonth ReminderRepeatUnit = "month"
	// ReminderRepeatYear repeats every N years on the month and day of
	// DueOn; a 29 February date falls on 28 February in common years.
	ReminderRepeatYear ReminderRepeatUnit = "year"
)

// IsValid reports whether u is one of the known repeat units.
func (u ReminderRepeatUnit) IsValid() bool {
	switch u {
	case ReminderRepeatNone, ReminderRepeatDay, ReminderRepeatWeek, ReminderRepeatMonth, ReminderRepeatYear:
		return true
	}
	return false
}

// Limits on a reminder's user-supplied values.
const (
	// ReminderTitleMaxLength caps the title like other short text fields.
	ReminderTitleMaxLength = 200
	// ReminderMaxLeadDays is the largest "remind me N days before" value.
	ReminderMaxLeadDays = 365
	// ReminderMaxLeadTimes is how many lead times one reminder may carry.
	ReminderMaxLeadTimes = 5
	// ReminderMaxRepeatInterval caps "every N units".
	ReminderMaxRepeatInterval = 999
)

// CommodityReminder is a dated reminder on an item: the end of a return
// window, an insurance renewal, a vehicle inspection. It carries a title, a
// date, one or more "remind me N days before" lead times and an optional
// repeat rule. The reminder worker emails the reminder's author before each
// occurrence; the group-wide Upcoming page lists the next occurrence.
//
// DueOn is the anchor of the repeat rule, not the next occurrence: every
// occurrence is computed from DueOn so that a reminder on 31 January keeps
// falling on the last day of each shorter month and returns to the 31st in
// the months that have one. NextOccurrence derives the current one.
//
//ptah:schema:rls:enable table="commodity_reminders" comment="Enable RLS for multi-tenant commodity reminder isolation"
//ptah:schema:rls:policy name="commodity_reminder_isolation" table="commodity_reminders" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures commodity reminders can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="commodity_reminder_background_worker_access" table="commodity_reminders" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to read commodity reminders across all groups"
//ptah:schema:table name="commodity_reminders"
type CommodityReminder struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	// CommodityID is the item the reminder belongs to. Deleting the item
	// deletes its reminders.
	//ptah:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_commodity_reminder_commodity" on_delete="CASCADE"
	CommodityID string `json:"commodity_id" db:"commodity_id"`

	// Title says what the date is about ("Return window ends").
	//ptah:schema:field name="title" type="TEXT" not_null="true"
	Title string `json:"title" db:"title"`

	// DueOn is the reminder's date (YYYY-MM-DD) and the anchor of its
	// repeat rule.
	//ptah:schema:field name="due_on" type="TEXT" not_null="true"
	DueOn Date `json:"due_on" db:"due_on"`

	// RemindDaysBefore lists the lead times in days, largest first. 0 means
	// on the day itself.
	//ptah:schema:field name="remind_days_before" type="JSONB" not_null="true"
	RemindDaysBefore ValuerSlice[int] `json:"remind_days_before" db:"remind_days_before"`

	// RepeatUnit is the unit of the repeat rule; "none" for a one-off
	// reminder.
	//ptah:schema:field name="repeat_unit" type="TEXT" not_null="true" default="none"
	RepeatUnit ReminderRepeatUnit `json:"repeat_unit" db:"repeat_unit"`

	// RepeatInterval is N in "every N units". It is 1 for a one-off
	// reminder.
	//ptah:schema:field name="repeat_interval" type="INTEGER" not_null="true" default="1"
	RepeatInterval int `json:"repeat_interval" db:"repeat_interval"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// CommodityReminderIndexes defines the postgres indexes for
// commodity_reminders.
type CommodityReminderIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore).
	//ptah:schema:index name="idx_commodity_reminders_uuid" fields="uuid" unique="true" table="commodity_reminders"
	_ int

	// Composite index for tenant+group RLS-filtered queries.
	//ptah:schema:index name="idx_commodity_reminders_tenant_group" fields="tenant_id,group_id" table="commodity_reminders"
	_ int

	// Per-item reads on the item page.
	//ptah:schema:index name="idx_commodity_reminders_commodity" fields="commodity_id" table="commodity_reminders"
	_ int
}

// NormalizeLeadTimes sorts the lead times largest first and drops
// duplicates, the order the worker and the API expect.
func (r *CommodityReminder) NormalizeLeadTimes() {
	days := slices.Clone([]int(r.RemindDaysBefore))
	slices.Sort(days)
	days = slices.Compact(days)
	slices.Reverse(days)
	r.RemindDaysBefore = days
}

// NextOccurrence returns the first occurrence of the reminder on or after
// today (the UTC date of now). A one-off reminder whose date has passed has
// no next occurrence and returns ("", false).
func (r *CommodityReminder) NextOccurrence(now time.Time) (Date, bool) {
	anchor := r.DueOn.ToTime()
	if anchor.IsZero() {
		return "", false
	}
	today := utcDay(now)
	if !anchor.Before(today) {
		return Date(anchor.Format(dateFormat)), true
	}
	interval := max(r.RepeatInterval, 1)
	var next time.Time
	switch r.RepeatUnit {
	case ReminderRepeatDay:
		next = nextByDays(anchor, today, interval)
	case ReminderRepeatWeek:
		next = nextByDays(anchor, today, interval*7)
	case ReminderRepeatMonth:
		next = nextByMonths(anchor, today, interval)
	case ReminderRepeatYear:
		next = nextByMonths(anchor, today, interval*12)
	default:
		return "", false
	}
	return Date(next.Format(dateFormat)), true
}

// DaysUntil returns the whole days from today (the UTC date of now) to d;
// negative when d is in the past.
func DaysUntil(d Date, now time.Time) int {
	t := d.ToTime()
	return int(t.Sub(utcDay(now)).Hours() / 24)
}

// utcDay truncates now to midnight UTC — the "today" every reminder worker
// uses.
func utcDay(now time.Time) time.Time {
	n := now.UTC()
	return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
}

// nextByDays returns the first anchor + k*step days on or after today.
// anchor is before today.
func nextByDays(anchor, today time.Time, step int) time.Time {
	diff := int(today.Sub(anchor).Hours() / 24)
	k := (diff + step - 1) / step
	return anchor.AddDate(0, 0, k*step)
}

// nextByMonths returns the first occurrence anchor + k*step months on or
// after today, each occurrence computed from the anchor with the day moved
// back to the end of a shorter month. anchor is before today.
func nextByMonths(anchor, today time.Time, step int) time.Time {
	months := (today.Year()-anchor.Year())*12 + int(today.Month()) - int(anchor.Month())
	k := max(months/step, 0)
	occ := addMonthsClamped(anchor, k*step)
	if occ.Before(today) {
		occ = addMonthsClamped(anchor, (k+1)*step)
	}
	return occ
}

// addMonthsClamped adds n months to t and keeps the day of the month,
// moving it back to the month's last day when the target month is shorter.
// time.AddDate would roll 31 January + 1 month over to 3 March instead.
func addMonthsClamped(t time.Time, n int) time.Time {
	total := int(t.Month()) - 1 + n
	year := t.Year() + total/12
	month := total % 12
	if month < 0 {
		month += 12
		year--
	}
	firstOfMonth := time.Date(year, time.Month(month+1), 1, 0, 0, 0, 0, time.UTC)
	lastDay := firstOfMonth.AddDate(0, 1, -1).Day()
	return time.Date(year, time.Month(month+1), min(t.Day(), lastDay), 0, 0, 0, 0, time.UTC)
}

func (*CommodityReminder) Validate() error {
	return ErrMustUseValidateWithContext
}

func (r *CommodityReminder) ValidateWithContext(ctx context.Context) error {
	if err := validation.ValidateStructWithContext(ctx, r,
		validation.Field(&r.TenantGroupAwareEntityID),
		validation.Field(&r.CommodityID, rules.NotEmpty),
	); err != nil {
		return err
	}
	return r.ValidateFields(ctx)
}

// ValidateFields checks the user-supplied fields only. The service calls it
// before a write, when the registry has not filled in the tenant, group and
// author yet.
func (r *CommodityReminder) ValidateFields(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, r,
		validation.Field(&r.Title, rules.NotEmpty, validation.Length(1, ReminderTitleMaxLength)),
		validation.Field(&r.DueOn, validation.Required, DateFormat),
		validation.Field(&r.RemindDaysBefore, ReminderLeadTimesRule),
		validation.Field(&r.RepeatUnit, validation.Required, ReminderRepeatUnitRule),
		validation.Field(&r.RepeatInterval, validation.Required, validation.Min(1), validation.Max(ReminderMaxRepeatInterval)),
	)
}

// ReminderLeadTimesRule checks a lead-time list: between one and
// ReminderMaxLeadTimes values, each between 0 and ReminderMaxLeadDays, no
// duplicates. It accepts ValuerSlice[int] and []int.
var ReminderLeadTimesRule validation.Rule = reminderLeadTimesRule{}

type reminderLeadTimesRule struct{}

func (reminderLeadTimesRule) Validate(value any) error {
	var days []int
	switch v := value.(type) {
	case ValuerSlice[int]:
		days = v
	case []int:
		days = v
	case *[]int:
		if v == nil {
			return nil
		}
		days = *v
	default:
		return nil
	}
	if len(days) == 0 {
		return validation.NewError("validation_reminder_lead_times_required", "at least one lead time is required")
	}
	if len(days) > ReminderMaxLeadTimes {
		return validation.NewError("validation_reminder_lead_times_too_many", "at most 5 lead times are allowed")
	}
	seen := make(map[int]struct{}, len(days))
	for _, d := range days {
		if d < 0 || d > ReminderMaxLeadDays {
			return validation.NewError("validation_reminder_lead_time_range", "each lead time must be between 0 and 365 days")
		}
		if _, dup := seen[d]; dup {
			return validation.NewError("validation_reminder_lead_times_duplicate", "lead times must not repeat")
		}
		seen[d] = struct{}{}
	}
	return nil
}

// ReminderRepeatUnitRule checks that a value is a known ReminderRepeatUnit.
// An empty value passes, so pair it with validation.Required when the field
// is mandatory.
var ReminderRepeatUnitRule validation.Rule = reminderRepeatUnitRule{}

type reminderRepeatUnitRule struct{}

func (reminderRepeatUnitRule) Validate(value any) error {
	var u ReminderRepeatUnit
	switch v := value.(type) {
	case ReminderRepeatUnit:
		u = v
	case *ReminderRepeatUnit:
		if v == nil {
			return nil
		}
		u = *v
	default:
		return nil
	}
	if u != "" && !u.IsValid() {
		return validation.NewError("validation_reminder_repeat_unit", "repeat must be one of: none, day, week, month, year")
	}
	return nil
}

var (
	_ validation.Validatable            = (*CommodityReminderSend)(nil)
	_ validation.ValidatableWithContext = (*CommodityReminderSend)(nil)
	_ TenantGroupAwareIDable            = (*CommodityReminderSend)(nil)
)

// CommodityReminderSend records that the reminder worker has claimed the
// email for one (reminder, occurrence, lead time). The unique index on that
// tuple is what keeps a reminder from going out twice, across restarts and
// across worker replicas: the worker inserts the row before it enqueues the
// email and only the inserting worker sends.
//
//ptah:schema:rls:enable table="commodity_reminder_sends" comment="Enable RLS for multi-tenant commodity reminder send isolation"
//ptah:schema:rls:policy name="commodity_reminder_send_isolation" table="commodity_reminder_sends" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures commodity reminder sends are accessible only by their tenant and group"
//ptah:schema:rls:policy name="commodity_reminder_send_background_worker_access" table="commodity_reminder_sends" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to record reminder emails across all groups"
//ptah:schema:table name="commodity_reminder_sends"
type CommodityReminderSend struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	// ReminderID is the reminder the email belongs to. Deleting the
	// reminder deletes its send records.
	//ptah:schema:field name="reminder_id" type="TEXT" not_null="true" foreign="commodity_reminders(id)" foreign_key_name="fk_commodity_reminder_send_reminder" on_delete="CASCADE"
	ReminderID string `json:"reminder_id" db:"reminder_id"`

	// OccurrenceOn is the occurrence (YYYY-MM-DD) the email was about.
	//ptah:schema:field name="occurrence_on" type="TEXT" not_null="true"
	OccurrenceOn Date `json:"occurrence_on" db:"occurrence_on"`

	// LeadDays is the lead time the email was sent for.
	//ptah:schema:field name="lead_days" type="INTEGER" not_null="true"
	LeadDays int `json:"lead_days" db:"lead_days"`

	// SentAt is when the worker claimed the email.
	//ptah:schema:field name="sent_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	SentAt time.Time `json:"sent_at" db:"sent_at"`
}

// CommodityReminderSendIndexes defines indexes for commodity_reminder_sends.
type CommodityReminderSendIndexes struct {
	// The idempotency key: one email per (reminder, occurrence, lead time).
	//ptah:schema:index name="idx_commodity_reminder_sends_key" fields="reminder_id,occurrence_on,lead_days" unique="true" table="commodity_reminder_sends"
	_ int

	// Tenant-scoped index for housekeeping queries.
	//ptah:schema:index name="idx_commodity_reminder_sends_tenant_id" fields="tenant_id" table="commodity_reminder_sends"
	_ int

	// Group-scoped index for the group purge.
	//ptah:schema:index name="idx_commodity_reminder_sends_group_id" fields="group_id" table="commodity_reminder_sends"
	_ int
}

func (*CommodityReminderSend) Validate() error {
	return ErrMustUseValidateWithContext
}

func (s *CommodityReminderSend) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, s,
		validation.Field(&s.TenantGroupAwareEntityID),
		validation.Field(&s.ReminderID, rules.NotEmpty),
		validation.Field(&s.OccurrenceOn, validation.Required, DateFormat),
		validation.Field(&s.LeadDays, validation.Min(0), validation.Max(ReminderMaxLeadDays)),
	)
}
