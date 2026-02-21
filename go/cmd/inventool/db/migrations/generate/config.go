package generate

type Config struct {
	GoEntitiesDir string `yaml:"go_entities_dir" env:"GO_ENTITIES_DIR" env-default:"./models"`
	MigrationsDir string `yaml:"migrations_dir" env:"DB_MIGRATIONS_DIR" env-default:"./migrations"`
	Preview       bool   `yaml:"preview" env:"PREVIEW" env-default:"false"`
	Check         bool   `yaml:"check" env:"CHECK" env-default:"false"`
}
