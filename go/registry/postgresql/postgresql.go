package postgresql

import (
	"context"
	"errors"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/registry"
	pgmigrations "github.com/denisvmedia/inventario/registry/postgresql/migrations"
)

const Name = "postgresql"

func Register() {
	registry.Register(Name, NewRegistrySet)
}

func NewRegistrySet(c registry.Config) (*registry.Set, error) {
	parsed, err := c.Parse()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to parse config DSN")
	}

	if parsed.Scheme != Name {
		return nil, errkit.Wrap(registry.ErrInvalidConfig, "invalid scheme")
	}

	// Create a connection pool
	poolConfig, err := pgxpool.ParseConfig(string(c))
	if err != nil {
		return nil, errkit.Wrap(err, "failed to parse PostgreSQL connection string")
	}

	// Set some reasonable defaults if not specified
	if poolConfig.MaxConns == 0 {
		poolConfig.MaxConns = 10
	}
	if poolConfig.MinConns == 0 {
		poolConfig.MinConns = 2
	}
	if poolConfig.MaxConnLifetime == 0 {
		poolConfig.MaxConnLifetime = 1 * time.Hour
	}
	if poolConfig.MaxConnIdleTime == 0 {
		poolConfig.MaxConnIdleTime = 30 * time.Minute
	}

	// Create the connection pool
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create PostgreSQL connection pool")
	}

	// Test the connection
	if err := pool.Ping(context.Background()); err != nil {
		return nil, errkit.Wrap(err, "failed to connect to PostgreSQL")
	}

	// Initialize the database schema
	if err := checkSchemaInited(pool); err != nil {
		return nil, errkit.Wrap(err, "failed to initialize database schema")
	}

	s := &registry.Set{}
	s.LocationRegistry = NewLocationRegistry(pool)
	s.AreaRegistry = NewAreaRegistry(pool, s.LocationRegistry)
	s.SettingsRegistry = NewSettingsRegistry(pool)
	s.CommodityRegistry = NewCommodityRegistry(pool, s.AreaRegistry)
	s.ImageRegistry = NewImageRegistry(pool, s.CommodityRegistry)
	s.InvoiceRegistry = NewInvoiceRegistry(pool, s.CommodityRegistry)
	s.ManualRegistry = NewManualRegistry(pool, s.CommodityRegistry)

	return s, nil
}

// checkSchemaInited checks if the database schema is up-to-date
func checkSchemaInited(pool *pgxpool.Pool) error {
	upToDate, err := pgmigrations.CheckMigrationsApplied(context.Background(), pool)
	if err != nil {
		return errkit.Wrap(err, "failed to check migrations")
	}

	if !upToDate {
		return errors.New("database schema is not up-to-date, please run migrations")
	}

	return nil
}

// ParsePostgreSQLURL parses a PostgreSQL URL and returns a connection string
func ParsePostgreSQLURL(u *url.URL) string {
	cp := *u // shallow copy
	cp.Scheme = "postgres"
	return cp.String()
}
