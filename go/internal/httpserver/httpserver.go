package httpserver

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

type APIServer struct {
	server     *http.Server
	ln         net.Listener
	done       chan struct{}
	doneOnce   sync.Once
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
		s.markDone()
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

// Shutdown stops the server and then runs the OnShutdown functions. After a
// Run whose listener could not bind, nothing is serving: Shutdown only runs
// the functions, so the caller can treat both cases alike.
func (s *APIServer) Shutdown() error {
	defer s.markDone()

	var err error
	if s.ln != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err = s.server.Shutdown(ctx)
	}
	for _, f := range s.onShutdown {
		f()
	}
	return err
}

// markDone closes the Done channel. Both a failed Run and Shutdown close it,
// and a caller that sees the bind error still calls Shutdown, so it closes
// only once.
func (s *APIServer) markDone() {
	s.doneOnce.Do(func() {
		if s.done != nil {
			close(s.done)
		}
	})
}

func (s *APIServer) Done() <-chan struct{} {
	return s.done
}
