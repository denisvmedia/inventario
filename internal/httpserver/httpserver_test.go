package httpserver_test

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/httpserver"
)

func TestAPIServer_Run(t *testing.T) {
	c := qt.New(t)
	apiServer := &httpserver.APIServer{}

	// Create a mock HTTP handler for testing.
	mockHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintln(w, "Hello, client")
		c.Assert(err, qt.IsNil)
	})

	// Start the server in a separate goroutine.
	doneCh, errCh := apiServer.Run("localhost:0", mockHandler)
	select {
	case <-doneCh:
		c.Fatalf("server stopped unexpectedly. Error: %v", <-errCh)
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

	<-doneCh
}
