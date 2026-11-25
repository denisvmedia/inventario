package models_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func day(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t.Add(15 * time.Hour) // any time of day maps to the same UTC date
}

func TestCommodityReminder_NextOccurrence(t *testing.T) {
	tests := []struct {
		name     string
		dueOn    string
		unit     models.ReminderRepeatUnit
		interval int
		today    string
		want     string
		wantOK   bool
	}{
		{name: "one-off in the future", dueOn: "2026-12-24", unit: models.ReminderRepeatNone, interval: 1, today: "2026-11-25", want: "2026-12-24", wantOK: true},
		{name: "one-off today", dueOn: "2026-11-25", unit: models.ReminderRepeatNone, interval: 1, today: "2026-11-25", want: "2026-11-25", wantOK: true},
		{name: "one-off in the past", dueOn: "2026-11-24", unit: models.ReminderRepeatNone, interval: 1, today: "2026-11-25", wantOK: false},
		{name: "future anchor ignores the repeat rule", dueOn: "2027-01-31", unit: models.ReminderRepeatMonth, interval: 1, today: "2026-11-25", want: "2027-01-31", wantOK: true},

		{name: "every 10 days, between occurrences", dueOn: "2026-11-01", unit: models.ReminderRepeatDay, interval: 10, today: "2026-11-25", want: "2026-12-01", wantOK: true},
		{name: "every 10 days, on an occurrence", dueOn: "2026-11-01", unit: models.ReminderRepeatDay, interval: 10, today: "2026-11-21", want: "2026-11-21", wantOK: true},
		{name: "every 2 weeks", dueOn: "2026-11-02", unit: models.ReminderRepeatWeek, interval: 2, today: "2026-11-25", want: "2026-11-30", wantOK: true},

		{name: "monthly on the 31st clamps to the end of November", dueOn: "2026-01-31", unit: models.ReminderRepeatMonth, interval: 1, today: "2026-11-25", want: "2026-11-30", wantOK: true},
		{name: "monthly on the 31st clamps to 28 February", dueOn: "2026-01-31", unit: models.ReminderRepeatMonth, interval: 1, today: "2026-02-01", want: "2026-02-28", wantOK: true},
		{name: "monthly on the 31st clamps to 29 February in a leap year", dueOn: "2028-01-31", unit: models.ReminderRepeatMonth, interval: 1, today: "2028-02-01", want: "2028-02-29", wantOK: true},
		{name: "monthly on the 31st returns to the 31st after a short month", dueOn: "2026-01-31", unit: models.ReminderRepeatMonth, interval: 1, today: "2026-03-01", want: "2026-03-31", wantOK: true},
		{name: "monthly, occurrence later this month", dueOn: "2026-01-28", unit: models.ReminderRepeatMonth, interval: 1, today: "2026-11-25", want: "2026-11-28", wantOK: true},
		{name: "monthly, occurrence earlier this month rolls to next month", dueOn: "2026-01-10", unit: models.ReminderRepeatMonth, interval: 1, today: "2026-11-25", want: "2026-12-10", wantOK: true},
		{name: "every 3 months crosses the year", dueOn: "2026-02-15", unit: models.ReminderRepeatMonth, interval: 3, today: "2026-11-16", want: "2027-02-15", wantOK: true},

		{name: "yearly", dueOn: "2024-03-10", unit: models.ReminderRepeatYear, interval: 1, today: "2026-11-25", want: "2027-03-10", wantOK: true},
		{name: "yearly on 29 February falls on 28 February in a common year", dueOn: "2024-02-29", unit: models.ReminderRepeatYear, interval: 1, today: "2026-11-25", want: "2027-02-28", wantOK: true},
		{name: "yearly on 29 February returns to 29 February in a leap year", dueOn: "2024-02-29", unit: models.ReminderRepeatYear, interval: 1, today: "2027-03-01", want: "2028-02-29", wantOK: true},
		{name: "every 2 years skips the off year", dueOn: "2024-06-01", unit: models.ReminderRepeatYear, interval: 2, today: "2026-06-02", want: "2028-06-01", wantOK: true},

		// More than 292 years back, where time.Duration saturates.
		{name: "daily from 1700", dueOn: "1700-01-01", unit: models.ReminderRepeatDay, interval: 1, today: "2026-11-25", want: "2026-11-25", wantOK: true},
		{name: "every 10 days from 1700", dueOn: "1700-01-01", unit: models.ReminderRepeatDay, interval: 10, today: "2026-11-25", want: "2026-11-28", wantOK: true},
		{name: "weekly from year 226", dueOn: "0226-01-01", unit: models.ReminderRepeatWeek, interval: 1, today: "2026-11-25", want: "2026-11-29", wantOK: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			r := models.CommodityReminder{DueOn: models.Date(tc.dueOn), RepeatUnit: tc.unit, RepeatInterval: tc.interval}
			got, ok := r.NextOccurrence(day(tc.today))
			c.Assert(ok, qt.Equals, tc.wantOK)
			c.Assert(string(got), qt.Equals, tc.want)
		})
	}
}

func TestDaysUntil(t *testing.T) {
	c := qt.New(t)
	now := day("2026-11-25")
	c.Assert(models.DaysUntil("2026-11-25", now), qt.Equals, 0)
	c.Assert(models.DaysUntil("2026-11-26", now), qt.Equals, 1)
	c.Assert(models.DaysUntil("2026-11-20", now), qt.Equals, -5)
	c.Assert(models.DaysUntil("2027-11-25", now), qt.Equals, 365)
	// Beyond the ~292 years a time.Duration can hold.
	c.Assert(models.DaysUntil("1626-11-25", now), qt.Equals, -146097)
	c.Assert(models.DaysUntil("2426-11-25", now), qt.Equals, 146097)
}

func TestCommodityReminder_NormalizeLeadTimes(t *testing.T) {
	c := qt.New(t)
	r := models.CommodityReminder{RemindDaysBefore: models.ValuerSlice[int]{1, 30, 7, 30, 0}}
	r.NormalizeLeadTimes()
	c.Assert([]int(r.RemindDaysBefore), qt.DeepEquals, []int{30, 7, 1, 0})
}

func validReminder() models.CommodityReminder {
	return models.CommodityReminder{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("r1", "t1", "g1", "u1"),
		CommodityID:              "c1",
		Title:                    "Return window ends",
		DueOn:                    "2026-12-24",
		RemindDaysBefore:         models.ValuerSlice[int]{7, 1},
		RepeatUnit:               models.ReminderRepeatNone,
		RepeatInterval:           1,
	}
}

func TestCommodityReminder_ValidateWithContext(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(r *models.CommodityReminder)
		wantErr string
	}{
		{name: "valid", mutate: func(*models.CommodityReminder) {}},
		{name: "missing title", mutate: func(r *models.CommodityReminder) { r.Title = "" }, wantErr: "title"},
		{name: "bad date", mutate: func(r *models.CommodityReminder) { r.DueOn = "24.12.2026" }, wantErr: "due_on"},
		{name: "year typo", mutate: func(r *models.CommodityReminder) { r.DueOn = "0226-12-24" }, wantErr: "due_on: the year must be between 1900 and 2200"},
		{name: "year 1", mutate: func(r *models.CommodityReminder) { r.DueOn = "0001-01-01" }, wantErr: "due_on: the year must be between 1900 and 2200"},
		{name: "year before 1900", mutate: func(r *models.CommodityReminder) { r.DueOn = "1899-12-31" }, wantErr: "due_on: the year must be between 1900 and 2200"},
		{name: "year after 2200", mutate: func(r *models.CommodityReminder) { r.DueOn = "2201-01-01" }, wantErr: "due_on: the year must be between 1900 and 2200"},
		{name: "first year allowed", mutate: func(r *models.CommodityReminder) { r.DueOn = "1900-01-01" }},
		{name: "last year allowed", mutate: func(r *models.CommodityReminder) { r.DueOn = "2200-12-31" }},
		{name: "lead time as long as the daily repeat", mutate: func(r *models.CommodityReminder) {
			r.RepeatUnit, r.RepeatInterval, r.RemindDaysBefore = models.ReminderRepeatDay, 3, models.ValuerSlice[int]{3}
		}, wantErr: "remind_days_before: each lead time must be shorter than the repeat interval"},
		{name: "lead time longer than the weekly repeat", mutate: func(r *models.CommodityReminder) {
			r.RepeatUnit, r.RepeatInterval, r.RemindDaysBefore = models.ReminderRepeatWeek, 1, models.ValuerSlice[int]{10, 1}
		}, wantErr: "remind_days_before: each lead time must be shorter than the repeat interval"},
		{name: "lead time inside the weekly repeat", mutate: func(r *models.CommodityReminder) {
			r.RepeatUnit, r.RepeatInterval, r.RemindDaysBefore = models.ReminderRepeatWeek, 1, models.ValuerSlice[int]{6, 0}
		}},
		{name: "lead time of a month on a monthly repeat", mutate: func(r *models.CommodityReminder) {
			r.RepeatUnit, r.RepeatInterval, r.RemindDaysBefore = models.ReminderRepeatMonth, 1, models.ValuerSlice[int]{28}
		}, wantErr: "remind_days_before: each lead time must be shorter than the repeat interval"},
		{name: "lead time inside a quarterly repeat", mutate: func(r *models.CommodityReminder) {
			r.RepeatUnit, r.RepeatInterval, r.RemindDaysBefore = models.ReminderRepeatMonth, 3, models.ValuerSlice[int]{60, 30}
		}},
		{name: "lead time of a year on a yearly repeat", mutate: func(r *models.CommodityReminder) {
			r.RepeatUnit, r.RepeatInterval, r.RemindDaysBefore = models.ReminderRepeatYear, 1, models.ValuerSlice[int]{365}
		}, wantErr: "remind_days_before: each lead time must be shorter than the repeat interval"},
		{name: "any lead time on a one-off reminder", mutate: func(r *models.CommodityReminder) { r.RemindDaysBefore = models.ValuerSlice[int]{365} }},
		{name: "no lead times", mutate: func(r *models.CommodityReminder) { r.RemindDaysBefore = nil }, wantErr: "remind_days_before"},
		{name: "lead time out of range", mutate: func(r *models.CommodityReminder) { r.RemindDaysBefore = models.ValuerSlice[int]{366} }, wantErr: "remind_days_before"},
		{name: "negative lead time", mutate: func(r *models.CommodityReminder) { r.RemindDaysBefore = models.ValuerSlice[int]{-1} }, wantErr: "remind_days_before"},
		{name: "duplicate lead times", mutate: func(r *models.CommodityReminder) { r.RemindDaysBefore = models.ValuerSlice[int]{7, 7} }, wantErr: "remind_days_before"},
		{name: "too many lead times", mutate: func(r *models.CommodityReminder) { r.RemindDaysBefore = models.ValuerSlice[int]{1, 2, 3, 4, 5, 6} }, wantErr: "remind_days_before"},
		{name: "unknown repeat unit", mutate: func(r *models.CommodityReminder) { r.RepeatUnit = "fortnight" }, wantErr: "repeat_unit"},
		{name: "zero interval", mutate: func(r *models.CommodityReminder) { r.RepeatInterval = 0 }, wantErr: "repeat_interval"},
		{name: "missing commodity", mutate: func(r *models.CommodityReminder) { r.CommodityID = "" }, wantErr: "commodity_id"},
		{name: "missing group", mutate: func(r *models.CommodityReminder) { r.GroupID = "" }, wantErr: "GroupID"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			r := validReminder()
			tc.mutate(&r)
			err := r.ValidateWithContext(context.Background())
			if tc.wantErr == "" {
				c.Assert(err, qt.IsNil)
				return
			}
			c.Assert(err, qt.ErrorMatches, ".*"+tc.wantErr+".*")
		})
	}
}

func TestCommodityReminderSend_ValidateWithContext(t *testing.T) {
	c := qt.New(t)
	s := models.CommodityReminderSend{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("s1", "t1", "g1", "u1"),
		ReminderID:               "r1",
		OccurrenceOn:             "2026-12-24",
		LeadDays:                 7,
	}
	c.Assert(s.ValidateWithContext(context.Background()), qt.IsNil)
	s.ReminderID = ""
	c.Assert(s.ValidateWithContext(context.Background()), qt.ErrorMatches, ".*reminder_id.*")
}
