package up

type Config struct {
	MigrationsDir string `yaml:"migrations_dir" env:"DB_MIGRATIONS_DIR" env-default:"./migrations"`
	DryRun        bool   `yaml:"dry_run" env:"DRY_RUN" env-default:"false"`
}
