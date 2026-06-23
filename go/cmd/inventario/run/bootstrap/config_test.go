package bootstrap_test

import (
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/run/bootstrap"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
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

func TestConfigSetDefaults_MaxUploadBytesDefaultsAppliedWhenZero(t *testing.T) {
	c := qt.New(t)

	// #2101: a YAML config that omits max_upload_bytes leaves the field at 0
	// (cleanenv applies env-default only on env reads), which would silently
	// disable the upload cap. SetDefaults must backfill the 1 GiB default.
	cfg := bootstrap.Config{MaxUploadBytes: 0}
	cfg.SetDefaults()

	c.Assert(cfg.MaxUploadBytes, qt.Equals, int64(1<<30))
}

func TestConfigSetDefaults_MaxUploadBytesPreservesExplicitPositive(t *testing.T) {
	c := qt.New(t)

	cfg := bootstrap.Config{MaxUploadBytes: 5 << 20}
	cfg.SetDefaults()

	c.Assert(cfg.MaxUploadBytes, qt.Equals, int64(5<<20))
}

func TestConfigSetDefaults_MaxUploadBytesPreservesNegativeAsDisabled(t *testing.T) {
	c := qt.New(t)

	// A negative value is the explicit opt-out ("no limit") — the enforcement
	// path treats <= 0 as "no cap", so it must NOT be backfilled.
	cfg := bootstrap.Config{MaxUploadBytes: -1}
	cfg.SetDefaults()

	c.Assert(cfg.MaxUploadBytes, qt.Equals, int64(-1))
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
	c.Assert(cfg.EmailVerificationCleanupInterval, qt.Equals, "1h")
	c.Assert(cfg.OperationSlotCleanupInterval, qt.Equals, "5m")
	c.Assert(cfg.ThumbnailBatchSize, qt.Equals, 10)
	c.Assert(cfg.ThumbnailPollInterval, qt.Equals, "5s")
	c.Assert(cfg.ThumbnailCleanupInterval, qt.Equals, "5m")
	c.Assert(cfg.ThumbnailJobRetentionPeriod, qt.Equals, "24h")
	c.Assert(cfg.ThumbnailJobBatchTimeout, qt.Equals, "30s")
	c.Assert(cfg.DetachedThumbnailJobTimeout, qt.Equals, "2m")
}

// TestConfig_FeatureCurrencyMigration_DefaultsOnWithoutEnvOrYAML pins the #1618
// part-2 finding: env-default:"true" on the FeatureCurrencyMigration bool DOES
// fire through shared.ReadSection's INVENTARIO_RUN_-prefixed wrapper on a fresh
// run (no env var, no config file), so the worker is enabled by default. The
// reporter's "disabled on a fresh docker run" was a pre-fix state. This guards
// against re-introducing a cobra flag with a hardcoded `false` default, which
// would clobber the loaded value (RegisterFlags binds flags AFTER ReadSection).
// EnableAPIDocs is pinned alongside because it shares the same default-true-bool
// mechanism (and the "swagger off by default" misconception is security-relevant).
func TestConfig_FeatureCurrencyMigration_DefaultsOnWithoutEnvOrYAML(t *testing.T) {
	c := qt.New(t)

	restore := shared.GetConfigFile()
	t.Cleanup(func() { shared.SetConfigFile(restore) })
	// Point at a non-existent file so ReadSection takes the env-only path and the
	// test never picks up a stray config.yaml from the working directory.
	shared.SetConfigFile(filepath.Join(t.TempDir(), "no-such-config.yaml"))
	// Clear any ambient values so the env-default path fires deterministically
	// regardless of the runner environment (t.Setenv restores originals on
	// cleanup; the immediate Unsetenv makes the vars absent during the test).
	t.Setenv("INVENTARIO_RUN_FEATURE_CURRENCY_MIGRATION", "")
	c.Assert(os.Unsetenv("INVENTARIO_RUN_FEATURE_CURRENCY_MIGRATION"), qt.IsNil)
	t.Setenv("INVENTARIO_RUN_ENABLE_API_DOCS", "")
	c.Assert(os.Unsetenv("INVENTARIO_RUN_ENABLE_API_DOCS"), qt.IsNil)

	var cfg bootstrap.Config
	shared.TryReadSection("run", &cfg)

	c.Assert(cfg.FeatureCurrencyMigration, qt.IsTrue)
	c.Assert(cfg.EnableAPIDocs, qt.IsTrue)
}

// TestConfig_FeatureCurrencyMigration_DisabledViaPrefixedEnv pins that the only
// working off-switch for a default-true bool is the prefixed env var
// (INVENTARIO_RUN_*) — the exact name the corrected workers.go log line now
// points operators to (#1618). A YAML `feature_currency_migration: false` would
// NOT disable it (false is the zero value and cleanenv re-applies env-default to
// zero fields), so the env var is the documented kill-switch.
func TestConfig_FeatureCurrencyMigration_DisabledViaPrefixedEnv(t *testing.T) {
	c := qt.New(t)

	restore := shared.GetConfigFile()
	t.Cleanup(func() { shared.SetConfigFile(restore) })
	shared.SetConfigFile(filepath.Join(t.TempDir(), "no-such-config.yaml"))

	t.Setenv("INVENTARIO_RUN_FEATURE_CURRENCY_MIGRATION", "false")

	var cfg bootstrap.Config
	shared.TryReadSection("run", &cfg)

	c.Assert(cfg.FeatureCurrencyMigration, qt.IsFalse)
}

func TestConfigSetDefaults_WorkerTunablesPreserveExplicitValues(t *testing.T) {
	c := qt.New(t)

	cfg := bootstrap.Config{
		MaxConcurrentExports:             7,
		MaxConcurrentImports:             4,
		MaxConcurrentRestores:            2,
		ExportPollInterval:               "20s",
		ImportPollInterval:               "25s",
		RestorePollInterval:              "30s",
		RefreshTokenCleanupInterval:      "15m",
		EmailVerificationCleanupInterval: "20m",
		ThumbnailBatchSize:               25,
		ThumbnailPollInterval:            "2s",
		ThumbnailCleanupInterval:         "10m",
		ThumbnailJobRetentionPeriod:      "48h",
		ThumbnailJobBatchTimeout:         "45s",
		DetachedThumbnailJobTimeout:      "4m",
	}
	cfg.SetDefaults()

	c.Assert(cfg.MaxConcurrentExports, qt.Equals, 7)
	c.Assert(cfg.MaxConcurrentImports, qt.Equals, 4)
	c.Assert(cfg.MaxConcurrentRestores, qt.Equals, 2)
	c.Assert(cfg.ExportPollInterval, qt.Equals, "20s")
	c.Assert(cfg.ImportPollInterval, qt.Equals, "25s")
	c.Assert(cfg.RestorePollInterval, qt.Equals, "30s")
	c.Assert(cfg.RefreshTokenCleanupInterval, qt.Equals, "15m")
	c.Assert(cfg.EmailVerificationCleanupInterval, qt.Equals, "20m")
	c.Assert(cfg.ThumbnailBatchSize, qt.Equals, 25)
	c.Assert(cfg.ThumbnailPollInterval, qt.Equals, "2s")
	c.Assert(cfg.ThumbnailCleanupInterval, qt.Equals, "10m")
	c.Assert(cfg.ThumbnailJobRetentionPeriod, qt.Equals, "48h")
	c.Assert(cfg.ThumbnailJobBatchTimeout, qt.Equals, "45s")
	c.Assert(cfg.DetachedThumbnailJobTimeout, qt.Equals, "4m")
}
