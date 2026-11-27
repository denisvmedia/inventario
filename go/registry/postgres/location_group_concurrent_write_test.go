package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

type groupWriteFixture struct {
	dbx     *sqlx.DB
	set     *registry.Set
	groups  registry.LocationGroupRegistry
	service *services.GroupService
	group   *models.LocationGroup
}

// newGroupWriteFixture returns an active group named Books in a fresh
// database, the group service that writes it, and a handle the tests use to
// hold the group's row.
func newGroupWriteFixture(t *testing.T) groupWriteFixture {
	t.Helper()
	c := qt.New(t)
	set, cleanup := setupTestRegistrySet(t)
	t.Cleanup(cleanup)

	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)

	user := getTestUser(c, set)
	seed := newTestGroup(c, user.TenantID, user.ID, "Books")
	seed.GroupCurrency = "USD"
	group, err := set.LocationGroupRegistry.Create(context.Background(), seed)
	c.Assert(err, qt.IsNil)

	return groupWriteFixture{
		dbx:     sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx"),
		set:     set,
		groups:  set.LocationGroupRegistry,
		service: services.NewGroupService(set.LocationGroupRegistry, set.GroupMembershipRegistry, set.GroupInviteRegistry),
		group:   group,
	}
}

// An admin renames a group while another admin starts deleting it. A third
// transaction holds the group's row, so the deletion queues on it first and
// the rename second. The deletion must stay: the rename is refused and the
// group is still pending deletion under its old name (#2918).
func TestGroupRename_Postgres_KeepsADeletionStartedMeanwhile(t *testing.T) {
	c := qt.New(t)
	fx := newGroupWriteFixture(t)
	ctx := context.Background()

	release := holdRowForUpdate(c, fx.dbx, "location_groups", fx.group.ID)

	deleteDone := make(chan error, 1)
	go func() {
		deleteDone <- fx.service.InitiateGroupDeletion(ctx, fx.group.ID, fx.group.Name, fx.group.Name)
	}()
	waitForLockWaiters(c, fx.dbx, 1, deleteDone)

	renameDone := make(chan error, 1)
	go func() {
		_, err := fx.service.UpdateGroup(ctx, fx.group.ID, "Book collection", fx.group.Icon, fx.group.Description)
		renameDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 2, deleteDone, renameDone)

	release()
	c.Assert(<-deleteDone, qt.IsNil)
	c.Assert(<-renameDone, qt.ErrorIs, services.ErrGroupNotActive)

	got, err := fx.groups.Get(ctx, fx.group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)
	c.Assert(got.Name, qt.Equals, "Books")
}

// An admin hides a section while another renames the group. The hidden
// sections reach the row first; the rename, which leaves them out, must not
// write back the list it read (#2918).
func TestGroupRename_Postgres_KeepsHiddenSectionsSetMeanwhile(t *testing.T) {
	c := qt.New(t)
	fx := newGroupWriteFixture(t)
	ctx := context.Background()

	release := holdRowForUpdate(c, fx.dbx, "location_groups", fx.group.ID)

	hideDone := make(chan error, 1)
	go func() {
		_, err := fx.service.SetHiddenSections(ctx, fx.group.ID, models.GroupSectionWarranties)
		hideDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, hideDone)

	renameDone := make(chan error, 1)
	go func() {
		_, err := fx.service.UpdateGroup(ctx, fx.group.ID, "Book collection", fx.group.Icon, fx.group.Description)
		renameDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 2, hideDone, renameDone)

	release()
	c.Assert(<-hideDone, qt.IsNil)
	c.Assert(<-renameDone, qt.IsNil)

	got, err := fx.groups.Get(ctx, fx.group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Name, qt.Equals, "Book collection")
	c.Assert(got.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties})
}

// A currency migration finishes while an admin renames the group: the
// worker's transaction has switched the group to EUR and not yet committed
// when the rename starts. The rename must keep the new currency, or the
// group would be back on USD with its prices already in EUR (#2918).
func TestGroupRename_Postgres_KeepsACurrencyMigrationFinishedMeanwhile(t *testing.T) {
	c := qt.New(t)
	fx := newGroupWriteFixture(t)
	ctx := context.Background()

	worker, err := fx.dbx.BeginTxx(ctx, nil)
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() { _ = worker.Rollback() })
	_, err = worker.ExecContext(ctx,
		`UPDATE location_groups SET group_currency = 'EUR', currency_migration_id = NULL, updated_at = $2 WHERE id = $1`,
		fx.group.ID, time.Now())
	c.Assert(err, qt.IsNil)

	renameDone := make(chan error, 1)
	go func() {
		_, err := fx.service.UpdateGroup(ctx, fx.group.ID, "Book collection", fx.group.Icon, fx.group.Description)
		renameDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, renameDone)

	c.Assert(worker.Commit(), qt.IsNil)
	c.Assert(<-renameDone, qt.IsNil)

	got, err := fx.groups.Get(ctx, fx.group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Name, qt.Equals, "Book collection")
	c.Assert(got.GroupCurrency, qt.Equals, models.Currency("EUR"))
}

// A currency migration finishes while the request that started it records
// its id on the group: the worker's transaction has cleared the id and
// marked the migration completed, and not yet committed. The id must not be
// written back, or the group would show as migrating with no migration left
// to finish (#2918).
func TestGroupCurrencyMigrationID_Postgres_NotWrittenBackOnceTheWorkerFinished(t *testing.T) {
	c := qt.New(t)
	fx := newGroupWriteFixture(t)
	ctx := context.Background()

	migration, err := fx.set.CurrencyMigrationRegistry.Create(ctx, models.CurrencyMigration{
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.NewFromFloat(0.92),
	})
	c.Assert(err, qt.IsNil)

	worker, err := fx.dbx.BeginTxx(ctx, nil)
	c.Assert(err, qt.IsNil)
	c.Cleanup(func() { _ = worker.Rollback() })
	now := time.Now()
	_, err = worker.ExecContext(ctx,
		`UPDATE location_groups SET group_currency = 'EUR', currency_migration_id = NULL, updated_at = $2 WHERE id = $1`,
		migration.GroupID, now)
	c.Assert(err, qt.IsNil)
	_, err = worker.ExecContext(ctx,
		`UPDATE currency_migrations SET status = 'completed', started_at = $2, completed_at = $2 WHERE id = $1`,
		migration.ID, now)
	c.Assert(err, qt.IsNil)

	setDone := make(chan error, 1)
	go func() {
		applied, err := fx.groups.SetCurrencyMigrationID(ctx, migration.GroupID, migration.ID)
		if err == nil && applied {
			err = errors.New("the id of a finished migration was written")
		}
		setDone <- err
	}()
	waitForLockWaiters(c, fx.dbx, 1, setDone)

	c.Assert(worker.Commit(), qt.IsNil)
	c.Assert(<-setDone, qt.IsNil)

	got, err := fx.groups.Get(ctx, migration.GroupID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.CurrencyMigrationID, qt.IsNil)
	c.Assert(got.GroupCurrency, qt.Equals, models.Currency("EUR"))
}
