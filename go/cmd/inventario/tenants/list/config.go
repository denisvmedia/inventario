package list

// Config holds configuration for tenant list command
type Config struct {
	// Filtering options
	Status string `yaml:"status" env:"TENANT_STATUS" env-default:""`
	Search string `yaml:"search" env:"TENANT_SEARCH" env-default:""`

	// Pagination options
	Limit  int `yaml:"limit" env:"LIMIT" env-default:"50"`
	Offset int `yaml:"offset" env:"OFFSET" env-default:"0"`

	// Output options
	Output string `yaml:"output" env:"OUTPUT" env-default:"table"`


}
