package run

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
)

func TestConfigSetDefaults_PreservesExplicitZeroEmailQueueMaxRetries(t *testing.T) {
	c := qt.New(t)

	cfg := Config{
		EmailQueueMaxRetries: 0,
	}

	cfg.setDefaults()

	c.Assert(cfg.EmailQueueMaxRetries, qt.Equals, 0)
}

func TestConfigSetDefaults_DefaultsNegativeEmailQueueMaxRetries(t *testing.T) {
	c := qt.New(t)

	cfg := Config{
		EmailQueueMaxRetries: -1,
	}

	cfg.setDefaults()

	c.Assert(cfg.EmailQueueMaxRetries, qt.Equals, 5)
}

func TestValidatePublicURLForTransactionalEmails_Valid(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name      string
		publicURL string
	}{
		{name: "https scheme", publicURL: "https://inventario.example.com"},
		{name: "http scheme", publicURL: "http://inventario.example.com"},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			err := validatePublicURLForTransactionalEmails(tc.publicURL)
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestConfigSetDefaults_GlobalRateLimitDefaultsApplied(t *testing.T) {
	c := qt.New(t)

	cfg := Config{
		GlobalRateLimit:  0,
		GlobalRateWindow: "",
	}

	cfg.setDefaults()

	c.Assert(cfg.GlobalRateLimit, qt.Equals, 1000)
	c.Assert(cfg.GlobalRateWindow, qt.Equals, "1h")
}

func TestConfigSetDefaults_GlobalRateLimitPreservesExplicitValues(t *testing.T) {
	c := qt.New(t)

	cfg := Config{
		GlobalRateLimit:  250,
		GlobalRateWindow: "30m",
	}

	cfg.setDefaults()

	c.Assert(cfg.GlobalRateLimit, qt.Equals, 250)
	c.Assert(cfg.GlobalRateWindow, qt.Equals, "30m")
}

func TestConfigSetDefaults_GlobalRateLimitDisabledPreservesZero(t *testing.T) {
	c := qt.New(t)

	cfg := Config{
		GlobalRateLimitDisabled: true,
		GlobalRateLimit:         0,
		GlobalRateWindow:        "",
	}

	cfg.setDefaults()

	c.Assert(cfg.GlobalRateLimit, qt.Equals, 0)
	c.Assert(cfg.GlobalRateWindow, qt.Equals, "1h")
}

func TestConfigSetDefaults_WorkerTunablesAppliedFromDefaults(t *testing.T) {
	c := qt.New(t)

	cfg := Config{}

	cfg.setDefaults()

	c.Assert(cfg.MaxConcurrentExports, qt.Equals, 3)
	c.Assert(cfg.MaxConcurrentImports, qt.Equals, 1)
	c.Assert(cfg.MaxConcurrentRestores, qt.Equals, 1)
	c.Assert(cfg.ExportPollInterval, qt.Equals, "10s")
	c.Assert(cfg.ImportPollInterval, qt.Equals, "10s")
	c.Assert(cfg.RestorePollInterval, qt.Equals, "10s")
	c.Assert(cfg.RefreshTokenCleanupInterval, qt.Equals, "1h")
	c.Assert(cfg.ThumbnailBatchSize, qt.Equals, 10)
	c.Assert(cfg.ThumbnailPollInterval, qt.Equals, "5s")
	c.Assert(cfg.ThumbnailCleanupInterval, qt.Equals, "5m")
	c.Assert(cfg.ThumbnailJobRetentionPeriod, qt.Equals, "24h")
	c.Assert(cfg.ThumbnailJobBatchTimeout, qt.Equals, "30s")
	c.Assert(cfg.DetachedThumbnailJobTimeout, qt.Equals, "2m")
}

func TestConfigSetDefaults_WorkerTunablesPreserveExplicitValues(t *testing.T) {
	c := qt.New(t)

	cfg := Config{
		MaxConcurrentExports:        7,
		MaxConcurrentImports:        4,
		MaxConcurrentRestores:       2,
		ExportPollInterval:          "20s",
		ImportPollInterval:          "25s",
		RestorePollInterval:         "30s",
		RefreshTokenCleanupInterval: "15m",
		ThumbnailBatchSize:          25,
		ThumbnailPollInterval:       "2s",
		ThumbnailCleanupInterval:    "10m",
		ThumbnailJobRetentionPeriod: "48h",
		ThumbnailJobBatchTimeout:    "45s",
		DetachedThumbnailJobTimeout: "4m",
	}

	cfg.setDefaults()

	c.Assert(cfg.MaxConcurrentExports, qt.Equals, 7)
	c.Assert(cfg.MaxConcurrentImports, qt.Equals, 4)
	c.Assert(cfg.MaxConcurrentRestores, qt.Equals, 2)
	c.Assert(cfg.ExportPollInterval, qt.Equals, "20s")
	c.Assert(cfg.ImportPollInterval, qt.Equals, "25s")
	c.Assert(cfg.RestorePollInterval, qt.Equals, "30s")
	c.Assert(cfg.RefreshTokenCleanupInterval, qt.Equals, "15m")
	c.Assert(cfg.ThumbnailBatchSize, qt.Equals, 25)
	c.Assert(cfg.ThumbnailPollInterval, qt.Equals, "2s")
	c.Assert(cfg.ThumbnailCleanupInterval, qt.Equals, "10m")
	c.Assert(cfg.ThumbnailJobRetentionPeriod, qt.Equals, "48h")
	c.Assert(cfg.ThumbnailJobBatchTimeout, qt.Equals, "45s")
	c.Assert(cfg.DetachedThumbnailJobTimeout, qt.Equals, "4m")
}

func TestValidatePublicURLForTransactionalEmails_Invalid(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name            string
		publicURL       string
		wantErrContains string
	}{
		{name: "missing", publicURL: "", wantErrContains: "public URL is required"},
		{name: "missing scheme", publicURL: "inventario.example.com", wantErrContains: "scheme and host are required"},
		{name: "unsupported scheme", publicURL: "ftp://inventario.example.com", wantErrContains: "unsupported scheme"},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			err := validatePublicURLForTransactionalEmails(tc.publicURL)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.wantErrContains)
		})
	}
}

func TestValidateEmailPublicURLConfig_Valid(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name      string
		provider  string
		publicURL string
	}{
		{name: "stub provider does not require public url", provider: "stub", publicURL: ""},
		{name: "supported provider with valid public url", provider: "smtp", publicURL: "https://inventario.example.com"},
		{name: "empty provider defaults to stub", provider: "", publicURL: ""},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			err := validateEmailPublicURLConfig(tc.provider, tc.publicURL)
			c.Assert(err, qt.IsNil)
		})
	}
}

func TestValidateEmailPublicURLConfig_Invalid(t *testing.T) {
	c := qt.New(t)

	cases := []struct {
		name            string
		provider        string
		publicURL       string
		wantErrContains string
	}{
		{
			name:            "supported provider with invalid public url",
			provider:        "smtp",
			publicURL:       "",
			wantErrContains: "invalid --public-url for email provider",
		},
		{
			name:            "unknown provider returns provider error",
			provider:        "unknown-provider",
			publicURL:       "",
			wantErrContains: "unsupported email provider",
		},
	}

	for _, tc := range cases {
		c.Run(tc.name, func(c *qt.C) {
			err := validateEmailPublicURLConfig(tc.provider, tc.publicURL)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.wantErrContains)
		})
	}
}

func TestParseWorkerDuration_Valid(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  time.Duration
	}{
		{"seconds", "5s", 5 * time.Second},
		{"minutes", "2m", 2 * time.Minute},
		{"hours", "1h", time.Hour},
		{"compound", "1h30m", 90 * time.Minute},
		{"milliseconds", "250ms", 250 * time.Millisecond},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			got, err := parseWorkerDuration("some-flag", tc.value)
			c.Assert(err, qt.IsNil)
			c.Assert(got, qt.Equals, tc.want)
		})
	}
}

func TestParseWorkerDuration_Errors(t *testing.T) {
	cases := []struct {
		name            string
		value           string
		wantErrContains string
	}{
		{"empty", "", "invalid --some-flag"},
		{"garbage", "not-a-duration", "invalid --some-flag"},
		{"bare number", "10", "invalid --some-flag"},
		{"zero", "0s", "must be positive"},
		{"negative", "-5s", "must be positive"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			got, err := parseWorkerDuration("some-flag", tc.value)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.wantErrContains)
			c.Assert(err.Error(), qt.Contains, "some-flag")
			c.Assert(got, qt.Equals, time.Duration(0))
		})
	}
}

func TestParseWorkerDurations_Valid(t *testing.T) {
	c := qt.New(t)

	cfg := Config{
		ExportPollInterval:          "11s",
		ImportPollInterval:          "12s",
		RestorePollInterval:         "13s",
		RefreshTokenCleanupInterval: "2h",
		ThumbnailPollInterval:       "7s",
		ThumbnailCleanupInterval:    "6m",
		ThumbnailJobRetentionPeriod: "48h",
		ThumbnailJobBatchTimeout:    "45s",
		DetachedThumbnailJobTimeout: "3m",
	}
	cmd := &Command{config: cfg}

	got, err := cmd.parseWorkerDurations()
	c.Assert(err, qt.IsNil)
	c.Assert(got.exportPollInterval, qt.Equals, 11*time.Second)
	c.Assert(got.importPollInterval, qt.Equals, 12*time.Second)
	c.Assert(got.restorePollInterval, qt.Equals, 13*time.Second)
	c.Assert(got.refreshTokenCleanupInterval, qt.Equals, 2*time.Hour)
	c.Assert(got.thumbnailPollInterval, qt.Equals, 7*time.Second)
	c.Assert(got.thumbnailCleanupInterval, qt.Equals, 6*time.Minute)
	c.Assert(got.thumbnailJobRetentionPeriod, qt.Equals, 48*time.Hour)
	c.Assert(got.thumbnailJobBatchTimeout, qt.Equals, 45*time.Second)
	c.Assert(got.detachedThumbnailJobTimeout, qt.Equals, 3*time.Minute)
}

func TestParseWorkerDurations_FailsOnInvalidFlag(t *testing.T) {
	cases := []struct {
		name        string
		mutate      func(*Config)
		wantFlag    string
		wantMessage string
	}{
		{
			name:        "invalid export poll interval",
			mutate:      func(c *Config) { c.ExportPollInterval = "nope" },
			wantFlag:    "export-poll-interval",
			wantMessage: "invalid",
		},
		{
			name:        "non-positive import poll interval",
			mutate:      func(c *Config) { c.ImportPollInterval = "0s" },
			wantFlag:    "import-poll-interval",
			wantMessage: "must be positive",
		},
		{
			name:        "non-positive thumbnail retention",
			mutate:      func(c *Config) { c.ThumbnailJobRetentionPeriod = "-1h" },
			wantFlag:    "thumbnail-job-retention-period",
			wantMessage: "must be positive",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			cfg := Config{}
			cfg.setDefaults()
			tc.mutate(&cfg)
			cmd := &Command{config: cfg}

			got, err := cmd.parseWorkerDurations()
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.wantFlag)
			c.Assert(err.Error(), qt.Contains, tc.wantMessage)
			c.Assert(got, qt.Equals, workerDurations{})
		})
	}
}
