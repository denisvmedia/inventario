package bootstrap

import (
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/httpserver"
)

// ProbesHandler builds the chi router served on the workers' probe listener. It
// exposes the same liveness/readiness endpoints as the API server so operators
// see a uniform `/healthz`, `/readyz`, `/metrics` surface regardless of which
// `run` subcommand is deployed, and it reuses apiserver.Health to keep the
// readiness semantics (DB + optional Redis) identical to the API server's
// check.
func ProbesHandler(rs *RuntimeSetup) http.Handler {
	r := chi.NewRouter()
	r.Group(apiserver.Health(rs.FactorySet, rs.Params.RedisPinger))
	r.Method(http.MethodGet, "/metrics", promhttp.Handler())
	return r
}

// StartProbes starts the workers' probe HTTP listener on cfg.ProbeAddr. It
// returns the running server handle plus the error channel produced by
// httpserver.Run. The caller must invoke WaitForWorkersShutdown (or otherwise
// consume errCh and call Shutdown) to terminate the listener cleanly. The
// listener is intentionally independent from the API server so `run workers`
// does not open any application-traffic port.
func StartProbes(cfg *Config, rs *RuntimeSetup) (*httpserver.APIServer, <-chan error) {
	srv := &httpserver.APIServer{}
	errCh := srv.Run(cfg.ProbeAddr, ProbesHandler(rs))
	slog.Info("Worker probes listener started",
		"addr", cfg.ProbeAddr,
		"endpoints", "/healthz,/readyz,/metrics",
	)
	return srv, errCh
}

// WaitForWorkersShutdown blocks until the probe server reports a startup error
// or the process receives SIGINT/SIGTERM, then issues a graceful shutdown of
// the probe listener. It mirrors WaitForShutdown but targets the workers'
// probe server rather than the API server.
func WaitForWorkersShutdown(srv *httpserver.APIServer, errCh <-chan error) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigCh:
	case err := <-errCh:
		slog.Error("Failure during worker probes startup", "error", err)
		return err
	}

	slog.Info("Shutting down worker probes listener")
	if err := srv.Shutdown(); err != nil {
		slog.Error("Failure during worker probes shutdown", "error", err)
		return err
	}
	return nil
}
