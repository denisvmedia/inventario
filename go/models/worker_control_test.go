package models_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

// TestWorkerTypeOrphanFileGC_IsPausable is the highest-value guard on the
// #2237 orphan-file GC: workerpause.Controller fails OPEN for worker types
// it does not know about (IsPaused returns false for an unknown key), so a
// constant that is declared but omitted from allWorkerTypes would make the
// only DESTRUCTIVE periodic worker in the tree impossible for an operator to
// stop with `inventario workers pause orphan-file-gc`. Assert the whole
// round-trip: the const is canonical, parses, validates, and is enumerated.
func TestWorkerTypeOrphanFileGC_IsPausable(t *testing.T) {
	c := qt.New(t)

	c.Assert(string(models.WorkerTypeOrphanFileGC), qt.Equals, "orphan-file-gc")
	c.Assert(models.WorkerTypeOrphanFileGC.IsValid(), qt.IsTrue)

	parsed, ok := models.ParseWorkerType("orphan-file-gc")
	c.Assert(ok, qt.IsTrue)
	c.Assert(parsed, qt.Equals, models.WorkerTypeOrphanFileGC)

	c.Assert(models.AllWorkerTypes(), qt.Contains, models.WorkerTypeOrphanFileGC)
}

// TestAllWorkerTypes_AreValidAndParseable keeps the three sources of truth in
// worker_control.go (the const block, allWorkerTypes, IsValid) from drifting:
// every enumerated type must validate and round-trip through ParseWorkerType.
func TestAllWorkerTypes_AreValidAndParseable(t *testing.T) {
	c := qt.New(t)

	all := models.AllWorkerTypes()
	c.Assert(all, qt.Not(qt.HasLen), 0)

	seen := make(map[models.WorkerType]bool, len(all))
	for _, wt := range all {
		c.Assert(wt.IsValid(), qt.IsTrue, qt.Commentf("worker type %q is enumerated but not accepted by IsValid", wt))

		parsed, ok := models.ParseWorkerType(string(wt))
		c.Assert(ok, qt.IsTrue, qt.Commentf("worker type %q does not round-trip through ParseWorkerType", wt))
		c.Assert(parsed, qt.Equals, wt)

		c.Assert(seen[wt], qt.IsFalse, qt.Commentf("worker type %q is listed twice", wt))
		seen[wt] = true
	}
}

func TestParseWorkerType_Unknown(t *testing.T) {
	c := qt.New(t)

	for _, s := range []string{"", "orphan-file-gc ", "orphan_file_gc", "email", "nope"} {
		parsed, ok := models.ParseWorkerType(s)
		c.Assert(ok, qt.IsFalse, qt.Commentf("input %q", s))
		c.Assert(parsed, qt.Equals, models.WorkerType(""))
	}
}
