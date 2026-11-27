package httpserver_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/httpserver"
)

func TestAPIServer_Run(t *testing.T) {
	c := qt.New(t)
	apiServer := &httpserver.APIServer{}

	// Create a mock HTTP handler for testing.
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, _r *http.Request) {
		_, err := fmt.Fprintln(w, "Hello, client")
		c.Assert(err, qt.IsNil)
	})

	// Start the server in a separate goroutine.
	errCh := apiServer.Run("localhost:0", mockHandler)
	select {
	case <-apiServer.Done():
		c.Fatal("server stopped unexpectedly")
	case err := <-errCh:
		c.Fatalf("server stopped unexpectedly. Error: %v", err)
	default:
		c.Log("Server running on", apiServer.Port())
	}

	r, err := http.NewRequest("GET", fmt.Sprintf("http://localhost:%d", apiServer.Port()), nil)
	c.Assert(err, qt.IsNil)
	resp, err := http.DefaultClient.Do(r)
	c.Assert(err, qt.IsNil)
	c.Assert(resp.StatusCode, qt.Equals, http.StatusOK)
	body, err := io.ReadAll(resp.Body)
	c.Assert(err, qt.IsNil)
	err = resp.Body.Close()
	c.Assert(err, qt.IsNil)
	c.Assert(string(body), qt.Equals, "Hello, client\n")

	// Shutdown the server.
	err = apiServer.Shutdown()

	// Assert the shutdown error, if any.
	c.Assert(err, qt.IsNil)

	// Wait for the server to stop.
	select {
	case err := <-errCh:
		c.Assert(err, qt.IsNil)
	case <-time.After(5 * time.Second):
		c.Fatal("server did not stop in time")
	}

	<-apiServer.Done()
}

// Resources the handler holds are released once the server has stopped.
func TestAPIServer_OnShutdownRunsAfterShutdown(t *testing.T) {
	c := qt.New(t)
	apiServer := &httpserver.APIServer{}
	var calls []string
	apiServer.OnShutdown(func() { calls = append(calls, "first") })
	apiServer.OnShutdown(func() { calls = append(calls, "second") })

	errCh := apiServer.Run("localhost:0", http.NotFoundHandler())
	c.Assert(calls, qt.HasLen, 0)

	c.Assert(apiServer.Shutdown(), qt.IsNil)
	c.Assert(calls, qt.DeepEquals, []string{"first", "second"})
	c.Assert(<-errCh, qt.IsNil)
}

// A listener that cannot bind reports the error, and the Shutdown that follows
// still releases what the handler holds.
func TestAPIServer_ShutdownAfterFailedBind(t *testing.T) {
	c := qt.New(t)

	taken, err := net.Listen("tcp", "localhost:0")
	c.Assert(err, qt.IsNil)
	defer taken.Close()

	apiServer := &httpserver.APIServer{}
	released := false
	apiServer.OnShutdown(func() { released = true })

	errCh := apiServer.Run(taken.Addr().String(), http.NotFoundHandler())
	c.Assert(<-errCh, qt.IsNotNil)
	<-apiServer.Done()

	c.Assert(apiServer.Shutdown(), qt.IsNil)
	c.Assert(released, qt.IsTrue)
}
