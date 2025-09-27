package defaults_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/defaults"
)

func TestDefaults(t *testing.T) {
	c := qt.New(t)

	cfg := defaults.New()

	// Test server defaults
	c.Assert(cfg.Server.Addr, qt.Equals, ":3333")
	c.Assert(cfg.Server.UploadLocation, qt.Contains, "uploads?create_dir=1")
	c.Assert(strings.HasPrefix(cfg.Server.UploadLocation, "file://"), qt.IsTrue)
	c.Assert(cfg.Server.JWTSecret, qt.Equals, "") // Empty by default

	// Test database defaults
	c.Assert(cfg.Database.DSN, qt.Equals, "memory://")

	// Test worker defaults
	c.Assert(cfg.Workers.MaxConcurrentExports, qt.Equals, 3)
	c.Assert(cfg.Workers.MaxConcurrentImports, qt.Equals, 1)

	// Test thumbnail generation defaults
	c.Assert(cfg.ThumbnailGeneration.MaxConcurrentPerUser, qt.Equals, 5)
	c.Assert(cfg.ThumbnailGeneration.RateLimitPerMinute, qt.Equals, 50)
	c.Assert(cfg.ThumbnailGeneration.SlotDuration, qt.Equals, "30m")
}

func TestDefaultGetters(t *testing.T) {
	c := qt.New(t)

	// Test individual getter functions
	c.Assert(defaults.GetServerAddr(), qt.Equals, ":3333")
	c.Assert(defaults.GetDatabaseDSN(), qt.Equals, "memory://")
	c.Assert(defaults.GetMaxConcurrentExports(), qt.Equals, 3)
	c.Assert(defaults.GetMaxConcurrentImports(), qt.Equals, 1)
	c.Assert(defaults.GetJWTSecret(), qt.Equals, "") // Empty by default

	// Test upload location getter
	uploadLocation := defaults.GetUploadLocation()
	c.Assert(uploadLocation, qt.Contains, "uploads?create_dir=1")
	c.Assert(strings.HasPrefix(uploadLocation, "file://"), qt.IsTrue)
}

func TestDefaultsConsistency(t *testing.T) {
	c := qt.New(t)

	// Test that the getter functions return the same values as the struct
	cfg := defaults.New()

	c.Assert(defaults.GetServerAddr(), qt.Equals, cfg.Server.Addr)
	c.Assert(defaults.GetDatabaseDSN(), qt.Equals, cfg.Database.DSN)
	c.Assert(defaults.GetMaxConcurrentExports(), qt.Equals, cfg.Workers.MaxConcurrentExports)
	c.Assert(defaults.GetMaxConcurrentImports(), qt.Equals, cfg.Workers.MaxConcurrentImports)
	c.Assert(defaults.GetUploadLocation(), qt.Equals, cfg.Server.UploadLocation)
	c.Assert(defaults.GetJWTSecret(), qt.Equals, cfg.Server.JWTSecret)
}
