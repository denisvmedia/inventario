package list

// Config holds configuration for user list command
type Config struct {
	// Filtering options
	Tenant string `yaml:"tenant" env:"USER_TENANT" env-default:""`
	Role   string `yaml:"role" env:"USER_ROLE" env-default:""`
	Active string `yaml:"active" env:"USER_ACTIVE" env-default:""`
	Search string `yaml:"search" env:"USER_SEARCH" env-default:""`

	// Pagination options
	Limit  int `yaml:"limit" env:"LIMIT" env-default:"50"`
	Offset int `yaml:"offset" env:"OFFSET" env-default:"0"`

	// Output options
	Output string `yaml:"output" env:"OUTPUT" env-default:"table"`


}
