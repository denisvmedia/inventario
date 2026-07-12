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
		ExportPollInterval:               "11s",
		ImportPollInterval:               "12s",
		RestorePollInterval:              "13s",
		RefreshTokenCleanupInterval:      "2h",
		EmailVerificationCleanupInterval: "45m",
		MagicLinkTokenCleanupInterval:    "40m",
		OperationSlotCleanupInterval:     "4m",
		GroupPurgeInterval:               "7m",
		WarrantyReminderInterval:         "30m",
		StorageQuotaReminderInterval:     "20m",
		LoanReminderInterval:             "45m",
		MaintenanceReminderInterval:      "55m",
		CurrencyMigrationInterval:        "8s",
		BusinessMetricsInterval:          "90s",
		OrphanFileGCInterval:             "12h",
		OrphanFileGCMinAge:               "48h",
		OrphanFileGCMode:                 "report",
		ThumbnailPollInterval:            "7s",
		ThumbnailCleanupInterval:         "6m",
		ThumbnailJobRetentionPeriod:      "48h",
		ThumbnailJobBatchTimeout:         "45s",
		DetachedThumbnailJobTimeout:      "3m",
	}

	got, err := bootstrap.ParseWorkerDurations(cfg)
	c.Assert(err, qt.IsNil)
	// Unset soft-pause refresh interval (#1308) falls back to 10s rather
	// than failing the parse, unlike the fail-fast worker intervals above.
	c.Assert(got.WorkerControlRefreshInterval, qt.Equals, 10*time.Second)
	c.Assert(got.ExportPollInterval, qt.Equals, 11*time.Second)
	c.Assert(got.ImportPollInterval, qt.Equals, 12*time.Second)
	c.Assert(got.RestorePollInterval, qt.Equals, 13*time.Second)
	c.Assert(got.RefreshTokenCleanupInterval, qt.Equals, 2*time.Hour)
	c.Assert(got.EmailVerificationCleanupInterval, qt.Equals, 45*time.Minute)
	c.Assert(got.MagicLinkTokenCleanupInterval, qt.Equals, 40*time.Minute)
	c.Assert(got.OperationSlotCleanupInterval, qt.Equals, 4*time.Minute)
	c.Assert(got.GroupPurgeInterval, qt.Equals, 7*time.Minute)
	c.Assert(got.WarrantyReminderInterval, qt.Equals, 30*time.Minute)
	c.Assert(got.StorageQuotaReminderInterval, qt.Equals, 20*time.Minute)
	c.Assert(got.LoanReminderInterval, qt.Equals, 45*time.Minute)
	c.Assert(got.MaintenanceReminderInterval, qt.Equals, 55*time.Minute)
	c.Assert(got.CurrencyMigrationInterval, qt.Equals, 8*time.Second)
	c.Assert(got.BusinessMetricsInterval, qt.Equals, 90*time.Second)
	c.Assert(got.OrphanFileGCInterval, qt.Equals, 12*time.Hour)
	c.Assert(got.OrphanFileGCMinAge, qt.Equals, 48*time.Hour)
	c.Assert(got.ThumbnailPollInterval, qt.Equals, 7*time.Second)
	c.Assert(got.ThumbnailCleanupInterval, qt.Equals, 6*time.Minute)
	c.Assert(got.ThumbnailJobRetentionPeriod, qt.Equals, 48*time.Hour)
	c.Assert(got.ThumbnailJobBatchTimeout, qt.Equals, 45*time.Second)
	c.Assert(got.DetachedThumbnailJobTimeout, qt.Equals, 3*time.Minute)
}

func TestParseWorkerDurations_WorkerControlRefreshOverride(t *testing.T) {
	c := qt.New(t)

	cfg := &bootstrap.Config{}
	cfg.SetDefaults()
	cfg.WorkerControlRefreshInterval = "3s"

	got, err := bootstrap.ParseWorkerDurations(cfg)
	c.Assert(err, qt.IsNil)
	c.Assert(got.WorkerControlRefreshInterval, qt.Equals, 3*time.Second)
}

func TestParseWorkerDurations_WorkerControlRefreshInvalidFallsBack(t *testing.T) {
	c := qt.New(t)

	cfg := &bootstrap.Config{}
	cfg.SetDefaults()
	// A malformed / non-positive value falls back to 10s rather than
	// aborting startup — unlike the fail-fast worker intervals.
	cfg.WorkerControlRefreshInterval = "-5s"

	got, err := bootstrap.ParseWorkerDurations(cfg)
	c.Assert(err, qt.IsNil)
	c.Assert(got.WorkerControlRefreshInterval, qt.Equals, 10*time.Second)
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
			name:        "non-positive email verification cleanup interval",
			mutate:      func(c *bootstrap.Config) { c.EmailVerificationCleanupInterval = "0s" },
			wantFlag:    "email-verification-cleanup-interval",
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

// The orphan-file GC is the only DESTRUCTIVE periodic worker in the tree, so
// both of its safety knobs are validated at startup and a bad value is a hard
// failure, not a warning an operator can miss in a log (#2237).
func TestParseWorkerDurations_OrphanFileGCSafetyKnobs(t *testing.T) {
	cases := []struct {
		name        string
		mutate      func(*bootstrap.Config)
		wantMessage string
	}{
		{
			name:        "min-age below the 24h hard floor is rejected",
			mutate:      func(c *bootstrap.Config) { c.OrphanFileGCMinAge = "1h" },
			wantMessage: "must be at least",
		},
		{
			name:        "min-age exactly one second below the floor is rejected",
			mutate:      func(c *bootstrap.Config) { c.OrphanFileGCMinAge = "23h59m59s" },
			wantMessage: "must be at least",
		},
		{
			name:        "non-positive interval is rejected",
			mutate:      func(c *bootstrap.Config) { c.OrphanFileGCInterval = "0s" },
			wantMessage: "must be positive",
		},
		{
			name:        "malformed interval is rejected",
			mutate:      func(c *bootstrap.Config) { c.OrphanFileGCInterval = "nope" },
			wantMessage: "invalid",
		},
		{
			name:        "unknown mode is rejected",
			mutate:      func(c *bootstrap.Config) { c.OrphanFileGCMode = "purge" },
			wantMessage: "must be one of off, report, delete",
		},
		{
			name:        "mode is case-sensitive",
			mutate:      func(c *bootstrap.Config) { c.OrphanFileGCMode = "DELETE" },
			wantMessage: "must be one of off, report, delete",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			cfg := &bootstrap.Config{}
			cfg.SetDefaults()
			tc.mutate(cfg)

			_, err := bootstrap.ParseWorkerDurations(cfg)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.wantMessage)
		})
	}
}

// The floor itself is accepted — it is a floor, not an exclusive bound.
func TestParseWorkerDurations_OrphanFileGCMinAgeFloorIsInclusive(t *testing.T) {
	c := qt.New(t)

	cfg := &bootstrap.Config{}
	cfg.SetDefaults()
	cfg.OrphanFileGCMinAge = "24h"

	got, err := bootstrap.ParseWorkerDurations(cfg)
	c.Assert(err, qt.IsNil)
	c.Assert(got.OrphanFileGCMinAge, qt.Equals, 24*time.Hour)
}
