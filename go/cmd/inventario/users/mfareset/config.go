package mfareset

// Config holds configuration for the user mfa-reset command.
type Config struct {
	Force  bool `yaml:"force" env:"FORCE" env-default:"false"`
	DryRun bool `yaml:"dry_run" env:"DRY_RUN" env-default:"false"`
}
