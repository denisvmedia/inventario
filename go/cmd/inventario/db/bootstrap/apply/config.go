package apply

type Config struct {
	Username              string `yaml:"username" env:"DB_USERNAME" env-default:"inventario"`
	UsernameForMigrations string `yaml:"username_for_migrations" env:"DB_USERNAME_FOR_MIGRATIONS"`
	DryRun                bool   `yaml:"dry_run" env:"DRY_RUN" env-default:"false"`
}

func (*Config) Validate() error {
	// TODO: implement me
	return nil
}
