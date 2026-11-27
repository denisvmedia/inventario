package postgres_test

import (
	"context"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/pgxpool"
)

// The schema every test in this package runs against carries
// newerReleaseColumn on every table (see buildSchema), so reads through the
// registries are covered by the whole suite. These tests check the writes: a
// row an older binary inserts gets the column's default, and a row it updates
// keeps what the newer release stored.

const newerReleaseValue = "written by the newer release"

func TestRegistryWrites_LeaveAColumnTheyDoNotKnow_Commodity(t *testing.T) {
	c := qt.New(t)
	ctx := c.Context()

	set, cleanup := setupTestRegistrySet(t)
	t.Cleanup(cleanup)
	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)

	location := createTestLocation(c, set)
	area := createTestArea(c, set, location.ID)
	commodity := createTestCommodity(c, set, area.ID)
	c.Assert(newerReleaseColumnValue(c, pool, "commodities", commodity.ID), qt.Equals, newerReleaseDefault)

	setNewerReleaseColumn(c, pool, "commodities", commodity.ID)

	got, err := set.CommodityRegistry.Get(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	got.Name = "Renamed by the older binary"
	_, err = set.CommodityRegistry.Update(ctx, *got)
	c.Assert(err, qt.IsNil)

	got, err = set.CommodityRegistry.Get(ctx, commodity.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Name, qt.Equals, "Renamed by the older binary")
	c.Assert(newerReleaseColumnValue(c, pool, "commodities", commodity.ID), qt.Equals, newerReleaseValue)
}

func TestRegistryWrites_LeaveAColumnTheyDoNotKnow_Location(t *testing.T) {
	c := qt.New(t)
	ctx := c.Context()

	set, cleanup := setupTestRegistrySet(t)
	t.Cleanup(cleanup)
	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)

	location := createTestLocation(c, set)
	c.Assert(newerReleaseColumnValue(c, pool, "locations", location.ID), qt.Equals, newerReleaseDefault)

	setNewerReleaseColumn(c, pool, "locations", location.ID)

	location.Name = "Renamed by the older binary"
	_, err = set.LocationRegistry.Update(ctx, *location)
	c.Assert(err, qt.IsNil)

	locations, err := set.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locations, qt.HasLen, 1)
	c.Assert(locations[0].Name, qt.Equals, "Renamed by the older binary")
	c.Assert(newerReleaseColumnValue(c, pool, "locations", location.ID), qt.Equals, newerReleaseValue)
}

// newerReleaseColumnValue reads the column directly, as the newer release
// would. The pool connects as the schema owner, so row-level security does not
// hide the row.
func newerReleaseColumnValue(c *qt.C, pool *pgxpool.Pool, table, id string) string {
	c.Helper()

	var value string
	err := pool.QueryRow(context.Background(),
		fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", newerReleaseColumn, table), id).Scan(&value)
	c.Assert(err, qt.IsNil)
	return value
}

func setNewerReleaseColumn(c *qt.C, pool *pgxpool.Pool, table, id string) {
	c.Helper()

	tag, err := pool.Exec(context.Background(),
		fmt.Sprintf("UPDATE %s SET %s = $1 WHERE id = $2", table, newerReleaseColumn), newerReleaseValue, id)
	c.Assert(err, qt.IsNil)
	c.Assert(tag.RowsAffected(), qt.Equals, int64(1))
}
