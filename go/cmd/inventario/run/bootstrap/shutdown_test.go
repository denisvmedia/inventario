package bootstrap_test

import (
	"net"
	"net/http"
	"regexp"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/cmd/inventario/run/bootstrap"
	"go.5x5.cz/inventario/internal/httpserver"
)

// takenAddr returns an address another listener holds for the rest of the
// test, so binding to it fails.
func takenAddr(c *qt.C) string {
	c.Helper()

	ln, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() { _ = ln.Close() })
	return ln.Addr().String()
}

func TestWaitForShutdownWithProbes_ReturnsTheAPIBindError(t *testing.T) {
	c := qt.New(t)
	addr := takenAddr(c)

	srv := &httpserver.APIServer{}
	errCh := srv.Run(addr, http.NotFoundHandler())
	probeSrv := &httpserver.APIServer{}
	probeErrCh := probeSrv.Run("localhost:0", http.NotFoundHandler())

	err := bootstrap.WaitForShutdownWithProbes(srv, errCh, probeSrv, probeErrCh)
	c.Assert(err, qt.ErrorMatches, ".*"+regexp.QuoteMeta(addr)+".*")
}

func TestWaitForShutdownWithProbes_ReturnsTheProbeBindError(t *testing.T) {
	c := qt.New(t)
	addr := takenAddr(c)

	srv := &httpserver.APIServer{}
	errCh := srv.Run("localhost:0", http.NotFoundHandler())
	probeSrv := &httpserver.APIServer{}
	probeErrCh := probeSrv.Run(addr, http.NotFoundHandler())

	err := bootstrap.WaitForShutdownWithProbes(srv, errCh, probeSrv, probeErrCh)
	c.Assert(err, qt.ErrorMatches, ".*"+regexp.QuoteMeta(addr)+".*")
}
