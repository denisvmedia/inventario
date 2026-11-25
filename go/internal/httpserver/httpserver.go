package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"
)

type APIServer struct {
	server     *http.Server
	ln         net.Listener
	done       chan struct{}
	onShutdown []func()
}

// OnShutdown registers f to run once Shutdown has stopped the server, for
// resources the handler holds. Register before calling Shutdown.
func (s *APIServer) OnShutdown(f func()) {
	s.onShutdown = append(s.onShutdown, f)
}

func (s *APIServer) Run(addr string, h http.Handler) <-chan error {
	s.done = make(chan struct{})
	s.server = &http.Server{
		Addr:              addr,
		Handler:           h,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 2)

	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		errCh <- err
		close(errCh)
		close(s.done)
		return errCh
	}
	s.ln = ln

	go func() {
		err := s.server.Serve(ln)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	return errCh
}

func (s *APIServer) Port() int {
	return (any)(s.ln.Addr()).(*net.TCPAddr).Port
}

func (s *APIServer) Shutdown() error {
	defer func() {
		close(s.done)
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := s.server.Shutdown(ctx)
	for _, f := range s.onShutdown {
		f()
	}
	return err
}

func (s *APIServer) Done() <-chan struct{} {
	return s.done
}
