package bootstrap

import (
	"fmt"
	"log/slog"
	"time"

	"github.com/denisvmedia/inventario/services"
)

// WorkerDurations holds the parsed time.Duration values for every duration-
// valued flag consumed by the background workers. It is produced once during
// bootstrap so that misconfiguration fails fast before any goroutines or the
// HTTP listener are started.
type WorkerDurations struct {
	ExportPollInterval               time.Duration
	ImportPollInterval               time.Duration
	RestorePollInterval              time.Duration
	RefreshTokenCleanupInterval      time.Duration
	EmailVerificationCleanupInterval time.Duration
	MagicLinkTokenCleanupInterval    time.Duration
	OperationSlotCleanupInterval     time.Duration
	GroupPurgeInterval               time.Duration
	WarrantyReminderInterval         time.Duration
	StorageQuotaReminderInterval     time.Duration
	LoanReminderInterval             time.Duration
	MaintenanceReminderInterval      time.Duration
	CurrencyMigrationInterval        time.Duration
	BusinessMetricsInterval          time.Duration
	WorkerControlRefreshInterval     time.Duration
	OrphanFileGCInterval             time.Duration
	OrphanFileGCMinAge               time.Duration
	ThumbnailPollInterval            time.Duration
	ThumbnailCleanupInterval         time.Duration
	ThumbnailJobRetentionPeriod      time.Duration
	ThumbnailJobBatchTimeout         time.Duration
	DetachedThumbnailJobTimeout      time.Duration
}

// parseWorkerDuration parses a single duration-valued flag. It enforces that
// the value is both well-formed and strictly positive so that downstream
// workers never receive 0 or negative intervals.
func parseWorkerDuration(flagName, value string) (time.Duration, error) {
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("invalid --%s %q: %w", flagName, value, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("invalid --%s %q: must be positive", flagName, value)
	}
	return d, nil
}

// ParseWorkerDurations validates every duration-valued worker flag on cfg and
// returns the parsed values. The first failing flag is logged and returned as
// an error; the caller is expected to abort startup in that case.
func ParseWorkerDurations(cfg *Config) (WorkerDurations, error) {
	var out WorkerDurations
	specs := []struct {
		flag  string
		value string
		dst   *time.Duration
	}{
		{"export-poll-interval", cfg.ExportPollInterval, &out.ExportPollInterval},
		{"import-poll-interval", cfg.ImportPollInterval, &out.ImportPollInterval},
		{"restore-poll-interval", cfg.RestorePollInterval, &out.RestorePollInterval},
		{"refresh-token-cleanup-interval", cfg.RefreshTokenCleanupInterval, &out.RefreshTokenCleanupInterval},
		{"email-verification-cleanup-interval", cfg.EmailVerificationCleanupInterval, &out.EmailVerificationCleanupInterval},
		{"magic-link-token-cleanup-interval", cfg.MagicLinkTokenCleanupInterval, &out.MagicLinkTokenCleanupInterval},
		{"operation-slot-cleanup-interval", cfg.OperationSlotCleanupInterval, &out.OperationSlotCleanupInterval},
		{"group-purge-interval", cfg.GroupPurgeInterval, &out.GroupPurgeInterval},
		{"warranty-reminder-interval", cfg.WarrantyReminderInterval, &out.WarrantyReminderInterval},
		{"storage-quota-reminder-interval", cfg.StorageQuotaReminderInterval, &out.StorageQuotaReminderInterval},
		{"loan-reminder-interval", cfg.LoanReminderInterval, &out.LoanReminderInterval},
		{"maintenance-reminder-interval", cfg.MaintenanceReminderInterval, &out.MaintenanceReminderInterval},
		{"currency-migration-interval", cfg.CurrencyMigrationInterval, &out.CurrencyMigrationInterval},
		{"business-metrics-interval", cfg.BusinessMetricsInterval, &out.BusinessMetricsInterval},
		{"orphan-file-gc-interval", cfg.OrphanFileGCInterval, &out.OrphanFileGCInterval},
		{"orphan-file-gc-min-age", cfg.OrphanFileGCMinAge, &out.OrphanFileGCMinAge},
		{"thumbnail-poll-interval", cfg.ThumbnailPollInterval, &out.ThumbnailPollInterval},
		{"thumbnail-cleanup-interval", cfg.ThumbnailCleanupInterval, &out.ThumbnailCleanupInterval},
		{"thumbnail-job-retention-period", cfg.ThumbnailJobRetentionPeriod, &out.ThumbnailJobRetentionPeriod},
		{"thumbnail-job-batch-timeout", cfg.ThumbnailJobBatchTimeout, &out.ThumbnailJobBatchTimeout},
		{"detached-thumbnail-job-timeout", cfg.DetachedThumbnailJobTimeout, &out.DetachedThumbnailJobTimeout},
	}
	for _, spec := range specs {
		d, err := parseWorkerDuration(spec.flag, spec.value)
		if err != nil {
			slog.Error("Failed to parse worker duration", "flag", spec.flag, "error", err)
			return WorkerDurations{}, err
		}
		*spec.dst = d
	}

	// The orphan-file GC min-age (#2237) has a HARD FLOOR on top of the
	// positive-value check every other duration gets. The GC is the only
	// destructive periodic worker in the tree and the age gate is one of its
	// load-bearing safety properties, so a too-short window is a startup
	// failure, not a warning an operator can miss in a log.
	if out.OrphanFileGCMinAge < services.MinOrphanFileGCMinAge {
		err := fmt.Errorf("invalid --orphan-file-gc-min-age %q: must be at least %s",
			cfg.OrphanFileGCMinAge, services.MinOrphanFileGCMinAge)
		slog.Error("Refusing to start the orphan-file GC with an unsafe min-age", "error", err)
		return WorkerDurations{}, err
	}

	// The orphan-file GC mode (#2237) is validated here too, alongside the
	// durations, so every knob the destructive worker reads is checked before
	// a single goroutine starts.
	if _, ok := services.ParseOrphanFileGCMode(cfg.OrphanFileGCMode); !ok {
		err := fmt.Errorf("invalid --orphan-file-gc-mode %q: must be one of off, report, delete", cfg.OrphanFileGCMode)
		slog.Error("Refusing to start with an unknown orphan-file GC mode", "error", err)
		return WorkerDurations{}, err
	}

	// The soft-pause refresh interval (#1308) is parsed tolerantly rather
	// than fail-fast: an unset or non-positive value falls back to the 10s
	// default instead of aborting startup. The controller would clamp it
	// anyway, and this keeps callers that build a bare Config (tests) from
	// having to populate a knob whose default is always safe.
	out.WorkerControlRefreshInterval = parseWorkerDurationOrDefault(cfg.WorkerControlRefreshInterval, defaultWorkerControlRefreshInterval)

	return out, nil
}

// defaultWorkerControlRefreshInterval mirrors the controller's own
// fallback and the defaults package value so a missing/invalid config
// resolves to the same 10s cadence everywhere.
const defaultWorkerControlRefreshInterval = 10 * time.Second

// parseWorkerDurationOrDefault parses value, returning fallback when value
// is empty, malformed, or non-positive. Used for the soft-pause refresh
// interval where an always-safe default is preferable to a hard startup
// failure.
func parseWorkerDurationOrDefault(value string, fallback time.Duration) time.Duration {
	d, err := time.ParseDuration(value)
	if err != nil || d <= 0 {
		// An unset value is a silent, always-safe default; but a non-empty
		// value that we couldn't honour is operator intent we're ignoring,
		// so surface it before falling back.
		if value != "" {
			slog.Warn("Ignoring invalid worker-control refresh interval; using default",
				"value", value, "default", fallback)
		}
		return fallback
	}
	return d
}
