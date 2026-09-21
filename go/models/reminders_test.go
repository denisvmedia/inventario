package models_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func validReminderIDs() models.TenantGroupAwareEntityID {
	return models.TenantGroupAwareEntityID{
		TenantID:        "tenant-1",
		GroupID:         "group-1",
		CreatedByUserID: "user-1",
	}
}

func TestWarrantyReminderThreshold_IsValid(t *testing.T) {
	c := qt.New(t)

	for _, threshold := range models.WarrantyReminderThresholds {
		c.Check(threshold.IsValid(), qt.IsTrue, qt.Commentf("canonical threshold %d", threshold))
	}

	// 0 is the maintenance worker's "overdue" marker, not a warranty
	// threshold — a row carrying it came from the wrong worker.
	for _, threshold := range []models.WarrantyReminderThreshold{0, 1, 14, 45, 90, -7} {
		c.Check(threshold.IsValid(), qt.IsFalse, qt.Commentf("threshold %d", threshold))
	}
}

func TestWarrantyReminder_ValidateWithContext(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		reminder models.WarrantyReminder
		wantErr  bool
	}{
		{
			name: "valid",
			reminder: models.WarrantyReminder{
				TenantGroupAwareEntityID: validReminderIDs(),
				CommodityID:              "commodity-1",
				ThresholdDays:            int(models.WarrantyReminder30Days),
				SentAt:                   time.Now(),
			},
		},
		{
			name: "missing commodity",
			reminder: models.WarrantyReminder{
				TenantGroupAwareEntityID: validReminderIDs(),
				ThresholdDays:            int(models.WarrantyReminder30Days),
			},
			wantErr: true,
		},
		{
			name: "missing tenant",
			reminder: models.WarrantyReminder{
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
					GroupID:         "group-1",
					CreatedByUserID: "user-1",
				},
				CommodityID:   "commodity-1",
				ThresholdDays: int(models.WarrantyReminder30Days),
			},
			wantErr: true,
		},
		{
			name: "missing group",
			reminder: models.WarrantyReminder{
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
					TenantID:        "tenant-1",
					CreatedByUserID: "user-1",
				},
				CommodityID:   "commodity-1",
				ThresholdDays: int(models.WarrantyReminder30Days),
			},
			wantErr: true,
		},
		{
			name: "threshold off the canonical list",
			reminder: models.WarrantyReminder{
				TenantGroupAwareEntityID: validReminderIDs(),
				CommodityID:              "commodity-1",
				ThresholdDays:            45,
			},
			wantErr: true,
		},
		{
			// The idempotency key is (commodity, threshold), so a zero
			// threshold would collapse every tier into one row.
			name: "zero threshold",
			reminder: models.WarrantyReminder{
				TenantGroupAwareEntityID: validReminderIDs(),
				CommodityID:              "commodity-1",
				ThresholdDays:            0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			err := tt.reminder.ValidateWithContext(ctx)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
				return
			}
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestMaintenanceReminderThreshold_IsValid(t *testing.T) {
	c := qt.New(t)

	for _, threshold := range models.MaintenanceReminderThresholds {
		c.Check(threshold.IsValid(), qt.IsTrue, qt.Commentf("canonical threshold %d", threshold))
	}

	for _, threshold := range []models.MaintenanceReminderThreshold{2, 3, 30, 60, -1} {
		c.Check(threshold.IsValid(), qt.IsFalse, qt.Commentf("threshold %d", threshold))
	}
}

func TestMaintenanceReminderThreshold_Label(t *testing.T) {
	c := qt.New(t)

	c.Check(models.MaintenanceReminder14Days.Label(), qt.Equals, "14-day")
	c.Check(models.MaintenanceReminder7Days.Label(), qt.Equals, "7-day")
	c.Check(models.MaintenanceReminder1Day.Label(), qt.Equals, "1-day")
	c.Check(models.MaintenanceReminderOverdue.Label(), qt.Equals, "overdue")

	// Labels reach email copy, so an unknown threshold must produce
	// nothing rather than a number the reader cannot interpret.
	c.Check(models.MaintenanceReminderThreshold(3).Label(), qt.Equals, "")

	seen := make(map[string]bool, len(models.MaintenanceReminderThresholds))
	for _, threshold := range models.MaintenanceReminderThresholds {
		label := threshold.Label()
		c.Check(seen[label], qt.IsFalse, qt.Commentf("label %q is used twice", label))
		seen[label] = true
	}
}

func TestMaintenanceReminder_ValidateWithContext(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		reminder models.MaintenanceReminder
		wantErr  bool
	}{
		{
			name: "valid",
			reminder: models.MaintenanceReminder{
				TenantGroupAwareEntityID: validReminderIDs(),
				ScheduleID:               "schedule-1",
				ThresholdDays:            int(models.MaintenanceReminder7Days),
				SentAt:                   time.Now(),
			},
		},
		{
			// Unlike the warranty thresholds, 0 is a real tier here: it
			// marks the one email sent after the due date passed.
			name: "overdue threshold is zero and valid",
			reminder: models.MaintenanceReminder{
				TenantGroupAwareEntityID: validReminderIDs(),
				ScheduleID:               "schedule-1",
				ThresholdDays:            int(models.MaintenanceReminderOverdue),
			},
		},
		{
			name: "missing schedule",
			reminder: models.MaintenanceReminder{
				TenantGroupAwareEntityID: validReminderIDs(),
				ThresholdDays:            int(models.MaintenanceReminder7Days),
			},
			wantErr: true,
		},
		{
			name: "missing created_by",
			reminder: models.MaintenanceReminder{
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
					TenantID: "tenant-1",
					GroupID:  "group-1",
				},
				ScheduleID:    "schedule-1",
				ThresholdDays: int(models.MaintenanceReminder7Days),
			},
			wantErr: true,
		},
		{
			name: "threshold off the canonical list",
			reminder: models.MaintenanceReminder{
				TenantGroupAwareEntityID: validReminderIDs(),
				ScheduleID:               "schedule-1",
				ThresholdDays:            3,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			err := tt.reminder.ValidateWithContext(ctx)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
				return
			}
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestStorageQuotaThreshold(t *testing.T) {
	c := qt.New(t)

	for _, threshold := range models.StorageQuotaThresholds {
		c.Check(threshold.IsValid(), qt.IsTrue, qt.Commentf("canonical threshold %d", threshold))
	}

	for _, threshold := range []models.StorageQuotaThreshold{0, 80, 95, 100} {
		c.Check(threshold.IsValid(), qt.IsFalse, qt.Commentf("threshold %d", threshold))
	}

	// The worker compares Ratio() against used_bytes/quota_bytes, so the
	// conversion has to be the fraction and not the percentage.
	c.Check(models.StorageQuota90Percent.Ratio(), qt.Equals, 0.9)
}

func TestStorageQuotaReminder_ValidateWithContext(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		reminder models.StorageQuotaReminder
		wantErr  bool
	}{
		{
			name: "valid",
			reminder: models.StorageQuotaReminder{
				TenantGroupAwareEntityID: validReminderIDs(),
				ThresholdPercent:         int(models.StorageQuota90Percent),
				SentAt:                   time.Now(),
			},
		},
		{
			name: "missing group",
			reminder: models.StorageQuotaReminder{
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
					TenantID:        "tenant-1",
					CreatedByUserID: "user-1",
				},
				ThresholdPercent: int(models.StorageQuota90Percent),
			},
			wantErr: true,
		},
		{
			name: "threshold off the canonical list",
			reminder: models.StorageQuotaReminder{
				TenantGroupAwareEntityID: validReminderIDs(),
				ThresholdPercent:         80,
			},
			wantErr: true,
		},
		{
			name: "zero threshold",
			reminder: models.StorageQuotaReminder{
				TenantGroupAwareEntityID: validReminderIDs(),
				ThresholdPercent:         0,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			err := tt.reminder.ValidateWithContext(ctx)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
				return
			}
			c.Assert(err, qt.IsNil)
		})
	}
}

// Every reminder model refuses the context-free Validate so a caller
// cannot skip the tenant and group checks by reaching for the shorter
// method name.
func TestReminders_ValidateRequiresContext(t *testing.T) {
	c := qt.New(t)

	c.Check((&models.WarrantyReminder{}).Validate(), qt.ErrorIs, models.ErrMustUseValidateWithContext)
	c.Check((&models.MaintenanceReminder{}).Validate(), qt.ErrorIs, models.ErrMustUseValidateWithContext)
	c.Check((&models.StorageQuotaReminder{}).Validate(), qt.ErrorIs, models.ErrMustUseValidateWithContext)
}
