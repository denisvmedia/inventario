package httpserver

import (
	"context"
	"net/http"
	"time"
)

type APIServer struct {
	server *http.Server
}

func (s *APIServer) Run(addr string, h http.Handler) <-chan error {
	s.server = &http.Server{
		Addr:    addr,
		Handler: h,
	}

	errCh := make(chan error, 2)

	go func() {
		err := s.server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	return errCh
}

func (s *APIServer) Shutdown() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.server.Shutdown(ctx)
}
