package list

type Config struct {
	MigrationsDir string `yaml:"migrations_dir" env:"DB_MIGRATIONS_DIR" env-default:"./migrations"`
}
