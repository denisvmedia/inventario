package deletecmd

// Config holds configuration for user delete command
type Config struct {
	// UploadLocation points at the object store the server uses, so the
	// deletion can delete the user's profile photo. When empty it falls back
	// to the configured default upload location.
	UploadLocation string `yaml:"upload_location" env:"UPLOAD_LOCATION" env-default:""`

	// Command options
	Force  bool `yaml:"force" env:"FORCE" env-default:"false"`
	DryRun bool `yaml:"dry_run" env:"DRY_RUN" env-default:"false"`
}
