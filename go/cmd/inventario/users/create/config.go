package create

// Config holds configuration for user creation command
type Config struct {
	// User fields
	Email    string `yaml:"email" env:"USER_EMAIL" env-default:""`
	Password string `yaml:"password" env:"USER_PASSWORD" env-default:""`
	Name     string `yaml:"name" env:"USER_NAME" env-default:""`
	Role     string `yaml:"role" env:"USER_ROLE" env-default:"user"`
	Tenant   string `yaml:"tenant" env:"USER_TENANT" env-default:""`
	Active   bool   `yaml:"active" env:"USER_ACTIVE" env-default:"true"`

	// Command options
	DryRun      bool `yaml:"dry_run" env:"DRY_RUN" env-default:"false"`
	Interactive bool `yaml:"interactive" env:"INTERACTIVE" env-default:"true"`
}
