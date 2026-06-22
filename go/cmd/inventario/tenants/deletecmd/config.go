package deletecmd

// Config holds configuration for tenant delete command
type Config struct {
	// Command options
	Force  bool `yaml:"force" env:"FORCE" env-default:"false"`
	DryRun bool `yaml:"dry_run" env:"DRY_RUN" env-default:"false"`

	// UploadLocation points at the same object store the server uses, so the
	// tenant hard-delete can remove the tenant's physical blobs before purging
	// its DB rows. When empty it falls back to the configured default; if that
	// does not match the running deployment the blobs are orphaned (the DB
	// purge still proceeds with a warning).
	UploadLocation string `yaml:"upload_location" env:"UPLOAD_LOCATION" env-default:""`
}
