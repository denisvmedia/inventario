package bootstrap

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/httpserver"
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
	return WaitForShutdownWithProbes(srv, errCh, nil, nil)
}

// WaitForShutdownWithProbes also owns the probe/metrics listener: an error on
// either channel ends the wait, and both are shut down. probeSrv may be nil.
func WaitForShutdownWithProbes(
	srv *httpserver.APIServer,
	errCh <-chan error,
	probeSrv *httpserver.APIServer,
	probeErrCh <-chan error,
) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	// A nil channel blocks forever, which is what an absent listener should do.
	var probeCh <-chan error
	if probeSrv != nil {
		probeCh = probeErrCh
	}

	var startupErr error
	select {
	case <-sigCh:
	case err := <-errCh:
		slog.Error("Failure during server startup", "error", err)
		startupErr = err
	case err := <-probeCh:
		slog.Error("Failure on the probe listener", "error", err)
		startupErr = err
	}

	slog.Info("Shutting down server")
	shutdownErr := srv.Shutdown()
	if shutdownErr != nil {
		slog.Error("Failure during server shutdown", "error", shutdownErr)
	}

	if probeSrv != nil {
		if err := probeSrv.Shutdown(); err != nil {
			slog.Error("Failure during probe listener shutdown", "error", err)
			if shutdownErr == nil {
				shutdownErr = err
			}
		}
	}

	if startupErr != nil {
		return startupErr
	}
	return shutdownErr
}
