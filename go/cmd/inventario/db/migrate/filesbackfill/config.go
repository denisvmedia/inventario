package filesbackfill

// Config holds configuration for the files backfill command.
type Config struct {
	DryRun bool `yaml:"dry_run" env:"DRY_RUN" env-default:"false"`
}
