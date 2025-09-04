package delete

// Config holds configuration for tenant delete command
type Config struct {
	// Command options
	Force  bool `yaml:"force" env:"FORCE" env-default:"false"`
	DryRun bool `yaml:"dry_run" env:"DRY_RUN" env-default:"false"`
}
