package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/schema/migrations/migrator"
	"github.com/denisvmedia/inventario/services/workerpause"
)

// Mode identifies which `run` subcommand is driving the bootstrap. It selects
// the startup log banner (so operators see "Starting API server", "Starting
// workers" or "Starting server" accordingly) and lets subsystems make
// mode-aware decisions without reaching back into cobra.
type Mode string

const (
	// ModeAll is the combined "API server + every worker" single-process mode
	// invoked by bare `inventario run` and `inventario run all`.
	ModeAll Mode = "all"
	// ModeAPIServer is the HTTP-only mode invoked by `inventario run apiserver`.
	ModeAPIServer Mode = "apiserver"
	// ModeWorkers is the workers-only mode invoked by `inventario run workers`.
	ModeWorkers Mode = "workers"
)

// RuntimeSetup aggregates the shared state produced by Build: registry factory,
// API server parameters, email lifecycle and validated worker duration flags.
// It is built once and fed into the per-subsystem Start* helpers so that the
// all/apiserver/workers subcommands can compose the same primitives.
type RuntimeSetup struct {
	DSN                       string
	FactorySet                *registry.FactorySet
	Params                    apiserver.Params
	EmailLifecycle            EmailServiceLifecycle
	WorkerDurations           WorkerDurations
	CloseReadinessRedisPinger func()

	// PauseController polls the worker_control rows and exposes the
	// soft-pause check the workers consult each tick (#1308). It is built
	// only in worker-bearing modes (ModeAll / ModeWorkers); it stays nil in
	// ModeAPIServer, where no worker run loop exists to gate.
	PauseController *workerpause.Controller
}

// Build performs the non-goroutine preamble of `inventario run`: it logs the
// startup context, resolves the registry factory, seeds the in-memory default
// tenant, builds the API server parameters and validates every duration-valued
// worker flag. On any failure, previously allocated external resources (Redis
// readiness clients) are released before the error is returned.
func Build(cfg *Config, dbConfig *shared.DatabaseConfig, mode Mode) (*RuntimeSetup, error) {
	dsn := dbConfig.DBDSN

	logInventarioEnv()
	logStartupInfo(mode, cfg.Addr, dsn)

	if err := verifySchemaMatchesBinary(dsn); err != nil {
		return nil, err
	}

	factorySet, err := resolveFactorySet(dsn)
	if err != nil {
		return nil, err
	}

	// Inject the operator-supplied currency-migration HMAC key into the
	// factory so preview tokens are verifiable across replicas / survive
	// restarts. Empty config value is a no-op (per-process random key).
	// Warn loudly (but accept) on shorter-than-recommended keys —
	// preview tokens gate a destructive operation, and a short key
	// weakens the signature without alerting the operator. 32 bytes
	// matches the SHA-256 block / output size; nothing larger gives
	// extra security.
	if cfg.CurrencyMigrationHMACKey != "" && factorySet.CurrencyMigrationRegistryFactory != nil {
		const recommendedHMACKeyLen = 32
		if len(cfg.CurrencyMigrationHMACKey) < recommendedHMACKeyLen {
			slog.Warn("CurrencyMigrationHMACKey is shorter than the recommended length; preview-token security is weakened",
				"configured_bytes", len(cfg.CurrencyMigrationHMACKey),
				"recommended_min_bytes", recommendedHMACKeyLen,
			)
		}
		factorySet.CurrencyMigrationRegistryFactory.SetHMACKey([]byte(cfg.CurrencyMigrationHMACKey))
	}

	seedMemoryDBDefaultTenant(dsn, factorySet)

	serverSetup, err := buildServerParams(cfg, factorySet, dsn)
	if err != nil {
		return nil, err
	}

	// Validate duration-valued worker flags up front so misconfiguration fails
	// fast without starting any background goroutines or the HTTP listener.
	durations, err := ParseWorkerDurations(cfg)
	if err != nil {
		serverSetup.closeReadinessRedisPinger()
		return nil, err
	}

	rs := &RuntimeSetup{
		DSN:                       dsn,
		FactorySet:                factorySet,
		Params:                    serverSetup.params,
		EmailLifecycle:            serverSetup.emailLifecycle,
		WorkerDurations:           durations,
		CloseReadinessRedisPinger: serverSetup.closeReadinessRedisPinger,
	}

	// Build the soft-pause controller only in worker-bearing modes (#1308).
	// ModeAPIServer has no worker run loops to gate, so it stays nil there
	// and the workers' nil-safe pause field keeps them running.
	if mode == ModeAll || mode == ModeWorkers {
		rs.PauseController = workerpause.NewController(
			factorySet.WorkerControlRegistry,
			workerpause.WithRefreshInterval(durations.WorkerControlRefreshInterval),
		)
	}

	return rs, nil
}

// logInventarioEnv emits every INVENTARIO_-prefixed environment variable name
// (values are intentionally omitted to avoid leaking secrets) to aid
// configuration troubleshooting.
func logInventarioEnv() {
	for _, e := range os.Environ() {
		name, _, _ := strings.Cut(e, "=")
		if strings.HasPrefix(name, "INVENTARIO_") {
			slog.Info("Environment variable", "name", name)
		}
	}
}

// logStartupInfo prints a mode-specific startup banner with the DSN credentials
// masked. ModeWorkers omits --addr since no HTTP listener is opened.
func logStartupInfo(mode Mode, addr, dsn string) {
	parsedDSN := must.Must(registry.Config(dsn).Parse())
	if parsedDSN.User != nil {
		parsedDSN.User = url.UserPassword("xxxxxx", "xxxxxx")
	}
	switch mode {
	case ModeAPIServer:
		slog.Info("Starting API server", "addr", addr, "db-dsn", parsedDSN.String())
	case ModeWorkers:
		slog.Info("Starting workers", "db-dsn", parsedDSN.String())
	default:
		slog.Info("Starting server", "addr", addr, "db-dsn", parsedDSN.String())
	}
}

// verifySchemaMatchesBinary refuses to start the app when the database has
// fewer migrations applied than this binary's embed.FS expects. The bug it
// catches (issue #1655) is the docker-compose footgun where four services
// each owned their own image tag, and a rebuild of one left the migrate
// container shipping a stale binary. Result: the app boots happily, points
// at a half-migrated schema, and queries fail with "column does not exist"
// from the request path.
//
// Memory and other in-process registries don't run real migrations — skip
// the check there so dev workflows and unit tests stay friction-free.
//
// The check is bounded by schemaVerifyTimeout so a slow / unreachable DB
// can't block startup indefinitely; orchestrators see a deterministic boot
// failure they can retry rather than a hung pod.
func verifySchemaMatchesBinary(dsn string) error {
	lowered := strings.ToLower(strings.TrimSpace(dsn))
	if !strings.HasPrefix(lowered, "postgres://") && !strings.HasPrefix(lowered, "postgresql://") {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), schemaVerifyTimeout)
	defer cancel()

	m := migrator.NewWithFallback(dsn, "")
	if err := m.VerifySchemaUpToDate(ctx); err != nil {
		if errors.Is(err, migrator.ErrSchemaLagsBinary) {
			slog.Error("refusing to start: database schema lags the binary's embedded migrations — re-run migrate from the current image (see #1655)",
				"error", err,
			)
		}
		return err
	}
	return nil
}

// schemaVerifyTimeout caps the startup pre-flight schema check. Long enough
// to absorb a cold connection pool + first query on a freshly-started
// postgres, short enough that an orchestrator's readiness probe can
// distinguish a hung DB from a missing migration.
const schemaVerifyTimeout = 30 * time.Second

// resolveFactorySet selects the registry implementation that matches the DSN
// scheme and instantiates its factory set.
func resolveFactorySet(dsn string) (*registry.FactorySet, error) {
	registrySetFn, ok := registry.GetRegistry(dsn)
	if !ok {
		slog.Error("Unknown registry", "dsn", dsn)
		return nil, errors.New("unknown registry")
	}
	slog.Info("Selected database registry", "registry_type", fmt.Sprintf("%T", registrySetFn))

	factorySet, err := registrySetFn(registry.Config(dsn))
	if err != nil {
		slog.Error("Failed to setup registry", "error", err)
		return nil, err
	}
	return factorySet, nil
}

// seedMemoryDBDefaultTenant creates a default tenant in memory-db mode so
// PublicTenantMiddleware can resolve it without manual setup steps. Other
// backends are expected to have a tenant seeded via migrations or the CLI.
//
// The slug + name are overridable via INVENTARIO_RUN_MEMORY_TENANT_SLUG /
// INVENTARIO_RUN_MEMORY_TENANT_NAME so the e2e harness can run against the
// canonical `test-org` tenant (which the seed code requires before it
// provisions the orphan user used by the no-group-redirect tests).
func seedMemoryDBDefaultTenant(dsn string, factorySet *registry.FactorySet) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(dsn)), "memory://") {
		return
	}
	slug := os.Getenv("INVENTARIO_RUN_MEMORY_TENANT_SLUG")
	if slug == "" {
		slug = "default"
	}
	name := os.Getenv("INVENTARIO_RUN_MEMORY_TENANT_NAME")
	if name == "" {
		name = "Default Tenant"
	}
	defaultTenant := models.Tenant{
		Name:             name,
		Slug:             slug,
		Status:           models.TenantStatusActive,
		IsDefault:        true,
		RegistrationMode: models.RegistrationModeClosed,
	}
	if _, err := factorySet.TenantRegistry.Create(context.Background(), defaultTenant); err != nil {
		slog.Warn("Failed to seed default tenant in memory-db mode", "error", err)
	}
}
