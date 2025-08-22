package data

// Config holds configuration for data migration commands
type Config struct {
	// Default tenant configuration
	DefaultTenantID   string `mapstructure:"default-tenant-id"`
	DefaultTenantName string `mapstructure:"default-tenant-name"`
	DefaultTenantSlug string `mapstructure:"default-tenant-slug"`
	
	// Admin user configuration
	AdminEmail    string `mapstructure:"admin-email"`
	AdminPassword string `mapstructure:"admin-password"`
	AdminName     string `mapstructure:"admin-name"`
	
	// Migration options
	DryRun bool `mapstructure:"dry-run"`
}

// DefaultConfig returns default configuration for data migration
func DefaultConfig() Config {
	return Config{
		DefaultTenantID:   "default-tenant-id",
		DefaultTenantName: "Default Organization",
		DefaultTenantSlug: "default",
		AdminEmail:        "admin@example.com",
		AdminPassword:     "admin123",
		AdminName:         "System Administrator",
		DryRun:            false,
	}
}
