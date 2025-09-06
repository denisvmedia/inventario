package update

// Config holds configuration for user update command
type Config struct {
	// User fields to update
	Email    string `yaml:"email" env:"USER_EMAIL" env-default:""`
	Name     string `yaml:"name" env:"USER_NAME" env-default:""`
	Role     string `yaml:"role" env:"USER_ROLE" env-default:""`
	Active   string `yaml:"active" env:"USER_ACTIVE" env-default:""`
	Tenant   string `yaml:"tenant" env:"USER_TENANT" env-default:""`
	Password bool   `yaml:"password" env:"USER_PASSWORD" env-default:"false"`

	// Command options
	DryRun      bool `yaml:"dry_run" env:"DRY_RUN" env-default:"false"`
	Interactive bool `yaml:"interactive" env:"INTERACTIVE" env-default:"false"`
}
