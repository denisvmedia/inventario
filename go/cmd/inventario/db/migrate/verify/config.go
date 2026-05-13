package verify

import "time"

type Config struct {
	MigrationsDir string        `yaml:"migrations_dir" env:"DB_MIGRATIONS_DIR" env-default:"./migrations"`
	Timeout       time.Duration `yaml:"timeout" env:"DB_VERIFY_TIMEOUT" env-default:"30s"`
}
