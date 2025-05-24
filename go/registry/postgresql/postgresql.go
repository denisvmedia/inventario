package postgresql

import (
	"context"
	"net/url"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/registry"
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
	if err := initSchema(pool); err != nil {
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

// initSchema initializes the database schema if it doesn't exist
func initSchema(pool *pgxpool.Pool) error {
	return InitSchemaForTesting(pool)
}

// InitSchemaForTesting initializes the database schema for testing
// This is exported for use in tests
func InitSchemaForTesting(pool *pgxpool.Pool) error {
	ctx := context.Background()

	// Create tables if they don't exist
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS locations (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			address TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS areas (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			location_id TEXT NOT NULL REFERENCES locations(id) ON DELETE CASCADE
		);

		CREATE TABLE IF NOT EXISTS commodities (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			short_name TEXT,
			type TEXT NOT NULL,
			area_id TEXT NOT NULL REFERENCES areas(id) ON DELETE CASCADE,
			count INTEGER NOT NULL DEFAULT 1,
			original_price DECIMAL(15,2),
			original_price_currency TEXT,
			converted_original_price DECIMAL(15,2),
			current_price DECIMAL(15,2),
			serial_number TEXT,
			extra_serial_numbers JSONB,
			part_numbers JSONB,
			tags JSONB,
			status TEXT NOT NULL,
			purchase_date TEXT,
			registered_date TEXT,
			last_modified_date TEXT,
			urls JSONB,
			comments TEXT,
			draft BOOLEAN NOT NULL DEFAULT FALSE
		);

		CREATE TABLE IF NOT EXISTS images (
			id TEXT PRIMARY KEY,
			commodity_id TEXT NOT NULL REFERENCES commodities(id) ON DELETE CASCADE,
			path TEXT NOT NULL,
			original_path TEXT NOT NULL,
			ext TEXT NOT NULL,
			mime_type TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS invoices (
			id TEXT PRIMARY KEY,
			commodity_id TEXT NOT NULL REFERENCES commodities(id) ON DELETE CASCADE,
			path TEXT NOT NULL,
			original_path TEXT NOT NULL,
			ext TEXT NOT NULL,
			mime_type TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS manuals (
			id TEXT PRIMARY KEY,
			commodity_id TEXT NOT NULL REFERENCES commodities(id) ON DELETE CASCADE,
			path TEXT NOT NULL,
			original_path TEXT NOT NULL,
			ext TEXT NOT NULL,
			mime_type TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS settings (
			id TEXT PRIMARY KEY DEFAULT 'settings',
			data JSONB NOT NULL
		);

		-- Insert default settings if they don't exist
		INSERT INTO settings (id, data)
		VALUES ('settings', '{}')
		ON CONFLICT (id) DO NOTHING;
	`)

	return err
}

// ParsePostgreSQLURL parses a PostgreSQL URL and returns a connection string
func ParsePostgreSQLURL(u *url.URL) string {
	cp := *u // shallow copy
	cp.Scheme = "postgres"
	return cp.String()
}
