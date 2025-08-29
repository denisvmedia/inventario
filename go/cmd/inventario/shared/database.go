package shared

import (
	"errors"
	"strings"

	"github.com/spf13/cobra"
)

type DatabaseConfig struct {
	DBDSN string `yaml:"db_dsn" env:"DB_DSN" mapstructure:"db_dsn" env-default:"memory://"`
}

func (d *DatabaseConfig) Validate() error {
	if d.DBDSN == "" {
		return errors.New("database DSN is required for this command")
	}
	// Validate that this is a PostgreSQL DSN
	if !strings.HasPrefix(d.DBDSN, "postgres://") && !strings.HasPrefix(d.DBDSN, "postgresql://") {
		return errors.New("config: bootstrap migrations only support PostgreSQL databases")
	}
	return nil
}

func RegisterDatabaseFlags(cmd *cobra.Command, cfg *DatabaseConfig) {
	TryReadVirtualSection(".", cfg)
	cmd.PersistentFlags().StringVar(&cfg.DBDSN, "db-dsn", cfg.DBDSN, "Database DSN")
}

func RegisterLocalDatabaseFlags(cmd *cobra.Command, cfg *DatabaseConfig) {
	TryReadVirtualSection(".", cfg)
	cmd.Flags().StringVar(&cfg.DBDSN, "db-dsn", cfg.DBDSN, "Database DSN")
}
