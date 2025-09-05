package get

// Config holds configuration for user get command
type Config struct {
	// Output options
	Output string `yaml:"output" env:"OUTPUT" env-default:"table"`


}
