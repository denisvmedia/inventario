package bootstrap_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/run/bootstrap"
)

func TestConfigSetDefaults_PreservesExplicitZeroEmailQueueMaxRetries(t *testing.T) {
	c := qt.New(t)

	cfg := bootstrap.Config{EmailQueueMaxRetries: 0}
	cfg.SetDefaults()

	c.Assert(cfg.EmailQueueMaxRetries, qt.Equals, 0)
}

func TestConfigSetDefaults_DefaultsNegativeEmailQueueMaxRetries(t *testing.T) {
	c := qt.New(t)

	cfg := bootstrap.Config{EmailQueueMaxRetries: -1}
	cfg.SetDefaults()

	c.Assert(cfg.EmailQueueMaxRetries, qt.Equals, 5)
}

func TestConfigSetDefaults_GlobalRateLimitDefaultsApplied(t *testing.T) {
	c := qt.New(t)

	cfg := bootstrap.Config{GlobalRateLimit: 0, GlobalRateWindow: ""}
	cfg.SetDefaults()

	c.Assert(cfg.GlobalRateLimit, qt.Equals, 1000)
	c.Assert(cfg.GlobalRateWindow, qt.Equals, "1h")
}

func TestConfigSetDefaults_GlobalRateLimitPreservesExplicitValues(t *testing.T) {
	c := qt.New(t)

	cfg := bootstrap.Config{GlobalRateLimit: 250, GlobalRateWindow: "30m"}
	cfg.SetDefaults()

	c.Assert(cfg.GlobalRateLimit, qt.Equals, 250)
	c.Assert(cfg.GlobalRateWindow, qt.Equals, "30m")
}

func TestConfigSetDefaults_GlobalRateLimitDisabledPreservesZero(t *testing.T) {
	c := qt.New(t)

	cfg := bootstrap.Config{
		GlobalRateLimitDisabled: true,
		GlobalRateLimit:         0,
		GlobalRateWindow:        "",
	}
	cfg.SetDefaults()

	c.Assert(cfg.GlobalRateLimit, qt.Equals, 0)
	c.Assert(cfg.GlobalRateWindow, qt.Equals, "1h")
}

func TestConfigSetDefaults_WorkerTunablesAppliedFromDefaults(t *testing.T) {
	c := qt.New(t)

	cfg := bootstrap.Config{}
	cfg.SetDefaults()

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

	cfg := bootstrap.Config{
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
	cfg.SetDefaults()

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

