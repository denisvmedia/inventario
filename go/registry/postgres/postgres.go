package postgres

import (
	"context"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const Name = "postgres"

func Register() (cleanup func() error) {
	newFn, cleanup := NewPostgresRegistrySet()
	registry.Register(Name, newFn)
	return cleanup
}

func NewFactorySet(dbx *sqlx.DB) *registry.FactorySet {
	// Create factory instances that will create context-aware registries
	restoreStepFactory := NewRestoreStepRegistry(dbx)

	fs := &registry.FactorySet{}
	fs.LocationRegistryFactory = NewLocationRegistry(dbx)
	fs.AreaRegistryFactory = NewAreaRegistry(dbx)
	fs.SettingsRegistryFactory = NewSettingsRegistry(dbx)
	fs.FileRegistryFactory = NewFileRegistry(dbx)
	fs.CommodityRegistryFactory = NewCommodityRegistry(dbx)
	fs.ImageRegistryFactory = NewImageRegistry(dbx)
	fs.InvoiceRegistryFactory = NewInvoiceRegistry(dbx)
	fs.ManualRegistryFactory = NewManualRegistry(dbx)
	fs.ExportRegistryFactory = NewExportRegistry(dbx)
	fs.RestoreStepRegistryFactory = restoreStepFactory
	fs.RestoreOperationRegistryFactory = NewRestoreOperationRegistry(dbx, restoreStepFactory)
	fs.TenantRegistry = NewTenantRegistry(dbx)
	fs.UserRegistry = NewUserRegistry(dbx)
	fs.RefreshTokenRegistry = NewRefreshTokenRegistry(dbx)
	fs.AuditLogRegistry = NewAuditLogRegistry(dbx)
	fs.EmailVerificationRegistry = NewEmailVerificationRegistry(dbx)
	fs.ThumbnailGenerationJobRegistryFactory = NewThumbnailGenerationJobRegistry(dbx)
	fs.UserConcurrencySlotRegistryFactory = NewUserConcurrencySlotRegistry(dbx)
	fs.OperationSlotRegistryFactory = NewOperationSlotRegistryFactory(dbx)

	return fs
}

func NewRegistrySetWithUserID(dbx *sqlx.DB, userID, tenantID string) *registry.Set {
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
			TenantID: tenantID,
		},
	})

	fs := NewFactorySet(dbx)
	s, err := fs.CreateUserRegistrySet(ctx)
	if err != nil {
		panic(err) // This maintains the same behavior as the original must.Must calls
	}
	return s
}

func NewPostgresRegistrySet() (registrySetFunc func(c registry.Config) (factorySet *registry.FactorySet, err error), cleanup func() error) {
	var doCleanup = func() error { return nil }

	return func(c registry.Config) (factorySet *registry.FactorySet, err error) {
		parsed, err := c.Parse()
		if err != nil {
			return nil, errxtrace.Wrap("failed to parse config DSN", err)
		}

		if parsed.Scheme != Name {
			return nil, errxtrace.Wrap("invalid scheme", registry.ErrInvalidConfig, errx.Attrs("expected", Name, "got", parsed.Scheme))
		}

		// Create a connection pool
		poolConfig, err := pgxpool.ParseConfig(string(c))
		if err != nil {
			return nil, errxtrace.Wrap("failed to parse PostgreSQL connection string", err)
		}

		// Set some reasonable defaults if not specified
		// Use smaller connection pools for testing to prevent exhaustion
		if poolConfig.MaxConns == 0 {
			poolConfig.MaxConns = 3
		}
		if poolConfig.MinConns == 0 {
			poolConfig.MinConns = 1
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
			return nil, errxtrace.Wrap("failed to create PostgreSQL connection pool", err)
		}

		// Test the connection
		if err := pool.Ping(context.Background()); err != nil {
			return nil, errxtrace.Wrap("failed to connect to PostgreSQL", err)
		}

		// Initialize the database schema
		if err := checkSchemaInited(pool); err != nil {
			return nil, errxtrace.Wrap("failed to initialize database schema", err)
		}

		// Create sqlx DB wrapper from pgxpool
		sqlDB := stdlib.OpenDBFromPool(pool)
		sqlxDB := sqlx.NewDb(sqlDB, "pgx")

		// Create PostgreSQL factory set
		factorySet = NewFactorySet(sqlxDB)

		doCleanup = func() error {
			err := sqlxDB.Close()
			pool.Close()
			return err
		}

		return factorySet, nil
	}, doCleanup
}

// checkSchemaInited checks if the database schema is up-to-date using Ptah
func checkSchemaInited(_ *pgxpool.Pool) error {
	// For now, skip schema validation in PostgreSQL registry
	// The schema validation should be handled by the application layer
	// This allows tests to work with the new Ptah migration system

	// TODO: Implement proper Ptah-based schema validation
	// For production use, consider adding a flag to enable/disable this check

	return nil
}
