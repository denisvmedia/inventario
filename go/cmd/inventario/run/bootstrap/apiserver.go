package bootstrap

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/httpserver"
)

// StartAPIServer starts the HTTP listener on cfg.Addr and returns the server
// handle plus the error channel produced by httpserver.Run. The caller must
// invoke WaitForShutdown (or otherwise consume errCh and call Shutdown) to
// terminate the server cleanly.
func StartAPIServer(cfg *Config, rs *RuntimeSetup, restoreStatus apiserver.RestoreStatusQuerier) (*httpserver.APIServer, <-chan error) {
	srv := &httpserver.APIServer{}
	errCh := srv.Run(cfg.Addr, apiserver.APIServer(rs.Params, restoreStatus))
	return srv, errCh
}

// WaitForShutdown blocks until the API server reports a startup error or the
// process receives SIGINT/SIGTERM, then issues a graceful shutdown.
func WaitForShutdown(srv *httpserver.APIServer, errCh <-chan error) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sigCh:
	case err := <-errCh:
		slog.Error("Failure during server startup", "error", err)
		return err
	}

	slog.Info("Shutting down server")
	if err := srv.Shutdown(); err != nil {
		slog.Error("Failure during server shutdown", "error", err)
		return err
	}
	return nil
}

// WaitForSignal blocks until the process receives SIGINT or SIGTERM. It is
// used by `run workers` where there is no HTTP listener to wait on.
func WaitForSignal() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
}
