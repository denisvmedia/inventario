// Package services_test covers factory-level CSRF tests only.
// Implementation-level tests live in the respective sub-packages:
//   - github.com/denisvmedia/inventario/csrf/inmemory
//   - github.com/denisvmedia/inventario/csrf/noop
//   - github.com/denisvmedia/inventario/csrf/redis
package services_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	csrfinmemory "github.com/denisvmedia/inventario/csrf/inmemory"
	"github.com/denisvmedia/inventario/services"
)

func TestNewCSRFService_FallbackToInMemory(t *testing.T) {
	c := qt.New(t)

	// Empty URL should create in-memory service
	svc := services.NewCSRFService("")
	c.Assert(svc, qt.IsNotNil)

	// Should work as an in-memory service
	ctx := context.Background()
	token, err := svc.GenerateToken(ctx, "test-user")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")

	// Clean up if it's an in-memory service
	if memSvc, ok := svc.(*csrfinmemory.Service); ok {
		defer memSvc.Stop()
	}
}

func TestNewCSRFService_InvalidRedisURL(t *testing.T) {
	c := qt.New(t)

	// Invalid Redis URL should fall back to in-memory with error logged
	svc := services.NewCSRFService("invalid://not-a-valid-url")
	c.Assert(svc, qt.IsNotNil)

	// Should still work (fallback to in-memory)
	ctx := context.Background()
	token, err := svc.GenerateToken(ctx, "test-user")
	c.Assert(err, qt.IsNil)
	c.Assert(token, qt.Not(qt.Equals), "")

	// Clean up if it's an in-memory service
	if memSvc, ok := svc.(*csrfinmemory.Service); ok {
		defer memSvc.Stop()
	}
}
