package create

// Config holds configuration for tenant creation command
type Config struct {
	// Tenant fields
	Name     string `yaml:"name" env:"TENANT_NAME" env-default:""`
	Slug     string `yaml:"slug" env:"TENANT_SLUG" env-default:""`
	Domain   string `yaml:"domain" env:"TENANT_DOMAIN" env-default:""`
	Status   string `yaml:"status" env:"TENANT_STATUS" env-default:"active"`
	Settings string `yaml:"settings" env:"TENANT_SETTINGS" env-default:""`

	// Command options
	DryRun      bool `yaml:"dry_run" env:"DRY_RUN" env-default:"false"`
	Interactive bool `yaml:"interactive" env:"INTERACTIVE" env-default:"true"`
	Default     bool `yaml:"default" env:"DEFAULT_TENANT" env-default:"false"`
}
