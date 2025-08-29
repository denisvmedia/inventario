package printcmd

type Config struct {
	Username                    string `yaml:"username" env:"DB_USERNAME" env-default:"inventario"`
	UsernameForMigrations       string `yaml:"username_for_migrations" env:"DB_USERNAME_FOR_MIGRATIONS"`
	UsernameForBackgroundWorker string `yaml:"username_for_background_worker" env:"DB_USERNAME_FOR_BACKGROUND_WORKER"`
}

func (*Config) Validate() error {
	// TODO: implement me
	return nil
}
