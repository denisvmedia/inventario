package bootstrap

import (
	"fmt"
	"log/slog"
	"time"
)

// WorkerDurations holds the parsed time.Duration values for every duration-
// valued flag consumed by the background workers. It is produced once during
// bootstrap so that misconfiguration fails fast before any goroutines or the
// HTTP listener are started.
type WorkerDurations struct {
	ExportPollInterval          time.Duration
	ImportPollInterval          time.Duration
	RestorePollInterval         time.Duration
	RefreshTokenCleanupInterval time.Duration
	ThumbnailPollInterval       time.Duration
	ThumbnailCleanupInterval    time.Duration
	ThumbnailJobRetentionPeriod time.Duration
	ThumbnailJobBatchTimeout    time.Duration
	DetachedThumbnailJobTimeout time.Duration
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
	return out, nil
}
