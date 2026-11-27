package registrytest

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// CurrencyMigrationEnv is one group's currency migrations on a backend.
// Migrations and Groups are scoped to the group of Ctx; Service is the
// service registry the worker uses. The backend holds no other group's
// migrations.
type CurrencyMigrationEnv struct {
	Ctx        context.Context
	GroupID    string
	Migrations registry.CurrencyMigrationRegistry
	Service    registry.CurrencyMigrationRegistry
	Groups     registry.LocationGroupRegistry
}

// RunCurrencyMigrationCleanupContract checks DeleteFinishedBefore (#2944):
// it deletes old completed and failed migrations with their audit rows, and
// never a pending or running one or one a group references.
func RunCurrencyMigrationCleanupContract(t *testing.T, newEnv func(t *testing.T) CurrencyMigrationEnv) {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	cutoff := now.Add(-90 * 24 * time.Hour)
	old := now.Add(-100 * 24 * time.Hour)

	t.Run("deletes completed and failed migrations that finished before the cutoff, with their audit rows", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		completed := finishedMigration(c, env, models.CurrencyMigrationStatusCompleted, old)
		writeMigrationAuditRows(c, env, completed.ID, 2)
		failed := finishedMigration(c, env, models.CurrencyMigrationStatusFailed, old)
		recent := finishedMigration(c, env, models.CurrencyMigrationStatusCompleted, now.Add(-24*time.Hour))
		writeMigrationAuditRows(c, env, recent.ID, 1)

		n, err := env.Service.DeleteFinishedBefore(env.Ctx, cutoff, 0)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 2)

		for _, gone := range []string{completed.ID, failed.ID} {
			_, err := env.Migrations.Get(env.Ctx, gone)
			c.Assert(err, qt.ErrorIs, registry.ErrNotFound, qt.Commentf("migration %s", gone))
		}
		rows, err := env.Migrations.ListAuditRows(env.Ctx, completed.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(rows, qt.HasLen, 0)

		kept, err := env.Migrations.Get(env.Ctx, recent.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(kept.Status, qt.Equals, models.CurrencyMigrationStatusCompleted)
		rows, err = env.Migrations.ListAuditRows(env.Ctx, recent.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(rows, qt.HasLen, 1)

		n, err = env.Service.DeleteFinishedBefore(env.Ctx, cutoff, 0)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 0)
	})

	t.Run("keeps pending and running migrations and one a group references", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		// Every migration of the group was created before this cutoff.
		later := now.Add(time.Hour)

		locked := finishedMigration(c, env, models.CurrencyMigrationStatusFailed, old)
		setGroupCurrencyMigration(c, env, new(locked.ID))
		inFlight, err := env.Migrations.Create(env.Ctx, newCurrencyMigration())
		c.Assert(err, qt.IsNil)

		n, err := env.Service.DeleteFinishedBefore(env.Ctx, later, 0)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 0, qt.Commentf("pending, and a failed one the group references"))

		c.Assert(env.Service.UpdateStatus(env.Ctx, inFlight.ID, registry.CurrencyMigrationStatusPatch{
			Status:    models.CurrencyMigrationStatusRunning,
			StartedAt: new(old),
		}), qt.IsNil)
		n, err = env.Service.DeleteFinishedBefore(env.Ctx, later, 0)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 0, qt.Commentf("running"))

		setGroupCurrencyMigration(c, env, nil)
		n, err = env.Service.DeleteFinishedBefore(env.Ctx, later, 0)
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 1)
		_, err = env.Migrations.Get(env.Ctx, locked.ID)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		running, err := env.Migrations.Get(env.Ctx, inFlight.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(running.Status, qt.Equals, models.CurrencyMigrationStatusRunning)
	})

	t.Run("deletes at most limit migrations per call", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		for range 3 {
			finishedMigration(c, env, models.CurrencyMigrationStatusCompleted, old)
		}

		for _, want := range []int{2, 1, 0} {
			n, err := env.Service.DeleteFinishedBefore(env.Ctx, cutoff, 2)
			c.Assert(err, qt.IsNil)
			c.Assert(n, qt.Equals, want)
		}
	})

	t.Run("needs the service registry", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		_, err := env.Migrations.DeleteFinishedBefore(env.Ctx, cutoff, 0)
		c.Assert(err, qt.ErrorIs, registry.ErrUserContextRequired)
	})
}

func newCurrencyMigration() models.CurrencyMigration {
	return models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.RequireFromString("0.9"),
	}
}

// finishedMigration creates a migration and ends it with the status at the
// time given.
func finishedMigration(c *qt.C, env CurrencyMigrationEnv, status models.CurrencyMigrationStatus, at time.Time) *models.CurrencyMigration {
	c.Helper()
	op, err := env.Migrations.Create(env.Ctx, newCurrencyMigration())
	c.Assert(err, qt.IsNil)
	c.Assert(env.Service.UpdateStatus(env.Ctx, op.ID, registry.CurrencyMigrationStatusPatch{
		Status:      status,
		StartedAt:   new(at),
		CompletedAt: new(at),
	}), qt.IsNil)
	return op
}

func writeMigrationAuditRows(c *qt.C, env CurrencyMigrationEnv, migrationID string, n int) {
	c.Helper()
	for range n {
		_, err := env.Migrations.WriteAuditRow(env.Ctx, models.CurrencyMigrationAuditRow{MigrationID: migrationID})
		c.Assert(err, qt.IsNil)
	}
}

func setGroupCurrencyMigration(c *qt.C, env CurrencyMigrationEnv, migrationID *string) {
	c.Helper()
	group, err := env.Groups.Get(env.Ctx, env.GroupID)
	c.Assert(err, qt.IsNil)
	group.CurrencyMigrationID = migrationID
	_, err = env.Groups.Update(env.Ctx, *group)
	c.Assert(err, qt.IsNil)
}
