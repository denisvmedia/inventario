package get

// Config holds configuration for tenant get command
type Config struct {
	// Output options
	Output string `yaml:"output" env:"OUTPUT" env-default:"table"`
}
