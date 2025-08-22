package data

// Config holds configuration for initial dataset setup commands
type Config struct {
	// Default tenant configuration
	DefaultTenantID   string `yaml:"default_tenant_id" env:"DEFAULT_TENANT_ID" env-default:"default-tenant-id"`
	DefaultTenantName string `yaml:"default_tenant_name" env:"DEFAULT_TENANT_NAME" env-default:"Default Organization"`
	DefaultTenantSlug string `yaml:"default_tenant_slug" env:"DEFAULT_TENANT_SLUG" env-default:"default"`

	// Admin user configuration
	AdminEmail    string `yaml:"admin_email" env:"ADMIN_EMAIL" env-default:"admin@example.com"`
	AdminPassword string `yaml:"admin_password" env:"ADMIN_PASSWORD" env-default:"admin123"`
	AdminName     string `yaml:"admin_name" env:"ADMIN_NAME" env-default:"System Administrator"`

	// Setup options
	DryRun bool `yaml:"dry_run" env:"DRY_RUN" env-default:"false"`
}
