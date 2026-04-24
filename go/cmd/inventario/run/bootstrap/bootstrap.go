package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
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

	factorySet, err := resolveFactorySet(dsn)
	if err != nil {
		return nil, err
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

	return &RuntimeSetup{
		DSN:                       dsn,
		FactorySet:                factorySet,
		Params:                    serverSetup.params,
		EmailLifecycle:            serverSetup.emailLifecycle,
		WorkerDurations:           durations,
		CloseReadinessRedisPinger: serverSetup.closeReadinessRedisPinger,
	}, nil
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
func seedMemoryDBDefaultTenant(dsn string, factorySet *registry.FactorySet) {
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(dsn)), "memory://") {
		return
	}
	defaultTenant := models.Tenant{
		Name:             "Default Tenant",
		Slug:             "default",
		Status:           models.TenantStatusActive,
		IsDefault:        true,
		RegistrationMode: models.RegistrationModeClosed,
	}
	if _, err := factorySet.TenantRegistry.Create(context.Background(), defaultTenant); err != nil {
		slog.Warn("Failed to seed default tenant in memory-db mode", "error", err)
	}
}
