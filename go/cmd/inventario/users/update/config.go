package update

// Config holds configuration for user update command
type Config struct {
	// User fields to update. Empty / unset values are left unchanged; the
	// command tracks which flags were explicitly provided (see update.go) so
	// that an unset flag never overwrites an existing column.
	Email    string `yaml:"email" env:"USER_EMAIL" env-default:""`
	Name     string `yaml:"name" env:"USER_NAME" env-default:""`
	Active   bool   `yaml:"active" env:"USER_ACTIVE" env-default:"true"`
	Tenant   string `yaml:"tenant" env:"USER_TENANT" env-default:""`
	Password bool   `yaml:"password" env:"USER_PASSWORD" env-default:"false"`

	// UploadLocation points at the object store the server uses, so a tenant
	// move can copy the user's profile photo into the new tenant's storage.
	// When empty it falls back to the configured default upload location.
	UploadLocation string `yaml:"upload_location" env:"UPLOAD_LOCATION" env-default:""`

	// Command options
	DryRun bool `yaml:"dry_run" env:"DRY_RUN" env-default:"false"`
}
