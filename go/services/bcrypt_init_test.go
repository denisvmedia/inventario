package services_test

import (
	"os"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services"
)

// TestMain lowers the bcrypt cost factor used inside this package for
// the full test-binary run. The MFA service hashes MFABackupCodeCount
// (10) codes serially per call at bcrypt.DefaultCost (~80ms each); under
// `go test -race` that scales to ~8 seconds per setup. Lowering to
// bcrypt.MinCost cuts the wall-clock by ~10x and keeps the wider
// `go test -race ./...` run inside the per-binary 10-minute panic
// timeout. Production callers are unaffected — the override is scoped
// to this test binary's process.
func TestMain(m *testing.M) {
	models.SetBcryptCostForTesting(nil, bcrypt.MinCost)
	services.SetBackupCodeBcryptCostForTesting(nil, bcrypt.MinCost)
	os.Exit(m.Run())
}
