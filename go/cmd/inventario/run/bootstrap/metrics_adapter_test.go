package bootstrap_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/run/bootstrap"
	"github.com/denisvmedia/inventario/registry"
)

// TestSystemStatsToBusinessStats asserts the registry.SystemStats →
// metrics.BusinessStats adapter copies every field into the matching
// slot. Distinct values per field guard against a transposed
// assignment (e.g. swapping StorageImages and StorageDocuments).
func TestSystemStatsToBusinessStats(t *testing.T) {
	c := qt.New(t)

	in := registry.SystemStats{
		Tenants:        1,
		Users:          2,
		LocationGroups: 3,
		Locations:      4,
		Areas:          5,
		Commodities:    6,
		Files:          7,

		StorageImages:    8,
		StorageDocuments: 9,
		StorageOther:     10,
		StorageExports:   11,
	}

	got := bootstrap.SystemStatsToBusinessStats(in)

	c.Assert(got.Tenants, qt.Equals, int64(1))
	c.Assert(got.Users, qt.Equals, int64(2))
	c.Assert(got.LocationGroups, qt.Equals, int64(3))
	c.Assert(got.Locations, qt.Equals, int64(4))
	c.Assert(got.Areas, qt.Equals, int64(5))
	c.Assert(got.Commodities, qt.Equals, int64(6))
	c.Assert(got.Files, qt.Equals, int64(7))
	c.Assert(got.StorageImages, qt.Equals, int64(8))
	c.Assert(got.StorageDocuments, qt.Equals, int64(9))
	c.Assert(got.StorageOther, qt.Equals, int64(10))
	c.Assert(got.StorageExports, qt.Equals, int64(11))
}
