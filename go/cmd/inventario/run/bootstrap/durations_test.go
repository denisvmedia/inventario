package bootstrap_test

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/run/bootstrap"
)

func TestParseWorkerDurations_Valid(t *testing.T) {
	c := qt.New(t)

	cfg := &bootstrap.Config{
		ExportPollInterval:           "11s",
		ImportPollInterval:           "12s",
		RestorePollInterval:          "13s",
		RefreshTokenCleanupInterval:  "2h",
		GroupPurgeInterval:           "7m",
		WarrantyReminderInterval:     "30m",
		StorageQuotaReminderInterval: "20m",
		LoanReminderInterval:         "45m",
		MaintenanceReminderInterval:  "55m",
		CurrencyMigrationInterval:    "8s",
		BusinessMetricsInterval:      "90s",
		ThumbnailPollInterval:        "7s",
		ThumbnailCleanupInterval:     "6m",
		ThumbnailJobRetentionPeriod:  "48h",
		ThumbnailJobBatchTimeout:     "45s",
		DetachedThumbnailJobTimeout:  "3m",
	}

	got, err := bootstrap.ParseWorkerDurations(cfg)
	c.Assert(err, qt.IsNil)
	c.Assert(got.ExportPollInterval, qt.Equals, 11*time.Second)
	c.Assert(got.ImportPollInterval, qt.Equals, 12*time.Second)
	c.Assert(got.RestorePollInterval, qt.Equals, 13*time.Second)
	c.Assert(got.RefreshTokenCleanupInterval, qt.Equals, 2*time.Hour)
	c.Assert(got.GroupPurgeInterval, qt.Equals, 7*time.Minute)
	c.Assert(got.WarrantyReminderInterval, qt.Equals, 30*time.Minute)
	c.Assert(got.StorageQuotaReminderInterval, qt.Equals, 20*time.Minute)
	c.Assert(got.LoanReminderInterval, qt.Equals, 45*time.Minute)
	c.Assert(got.MaintenanceReminderInterval, qt.Equals, 55*time.Minute)
	c.Assert(got.CurrencyMigrationInterval, qt.Equals, 8*time.Second)
	c.Assert(got.BusinessMetricsInterval, qt.Equals, 90*time.Second)
	c.Assert(got.ThumbnailPollInterval, qt.Equals, 7*time.Second)
	c.Assert(got.ThumbnailCleanupInterval, qt.Equals, 6*time.Minute)
	c.Assert(got.ThumbnailJobRetentionPeriod, qt.Equals, 48*time.Hour)
	c.Assert(got.ThumbnailJobBatchTimeout, qt.Equals, 45*time.Second)
	c.Assert(got.DetachedThumbnailJobTimeout, qt.Equals, 3*time.Minute)
}

func TestParseWorkerDurations_FailsOnInvalidFlag(t *testing.T) {
	cases := []struct {
		name        string
		mutate      func(*bootstrap.Config)
		wantFlag    string
		wantMessage string
	}{
		{
			name:        "invalid export poll interval",
			mutate:      func(c *bootstrap.Config) { c.ExportPollInterval = "nope" },
			wantFlag:    "export-poll-interval",
			wantMessage: "invalid",
		},
		{
			name:        "non-positive import poll interval",
			mutate:      func(c *bootstrap.Config) { c.ImportPollInterval = "0s" },
			wantFlag:    "import-poll-interval",
			wantMessage: "must be positive",
		},
		{
			name:        "non-positive thumbnail retention",
			mutate:      func(c *bootstrap.Config) { c.ThumbnailJobRetentionPeriod = "-1h" },
			wantFlag:    "thumbnail-job-retention-period",
			wantMessage: "must be positive",
		},
		{
			name:        "empty export poll interval",
			mutate:      func(c *bootstrap.Config) { c.ExportPollInterval = "" },
			wantFlag:    "export-poll-interval",
			wantMessage: "invalid",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			cfg := &bootstrap.Config{}
			cfg.SetDefaults()
			tc.mutate(cfg)

			got, err := bootstrap.ParseWorkerDurations(cfg)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.wantFlag)
			c.Assert(err.Error(), qt.Contains, tc.wantMessage)
			c.Assert(got, qt.Equals, bootstrap.WorkerDurations{})
		})
	}
}
