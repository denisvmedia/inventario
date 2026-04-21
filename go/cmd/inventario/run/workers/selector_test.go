package workers_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/run/workers"
)

func TestParseSelector_DefaultsToAll(t *testing.T) {
	c := qt.New(t)

	set, err := workers.ParseSelector("", "")

	c.Assert(err, qt.IsNil)
	c.Assert(set.Sorted(), qt.DeepEquals, workers.AllWorkerIDs())
}

func TestParseSelector_ExplicitAllKeyword(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{name: "lower case", input: "all"},
		{name: "upper case", input: "ALL"},
		{name: "whitespace padded", input: "  all  "},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			set, err := workers.ParseSelector(tc.input, "")

			c.Assert(err, qt.IsNil)
			c.Assert(set.Sorted(), qt.DeepEquals, workers.AllWorkerIDs())
		})
	}
}

func TestParseSelector_OnlySelectsSubset(t *testing.T) {
	c := qt.New(t)

	set, err := workers.ParseSelector("thumbnails,exports", "")

	c.Assert(err, qt.IsNil)
	c.Assert(set.Sorted(), qt.DeepEquals, []workers.WorkerID{
		workers.WorkerExports,
		workers.WorkerThumbnails,
	})
}

func TestParseSelector_OnlyNormalizesCaseAndWhitespace(t *testing.T) {
	c := qt.New(t)

	set, err := workers.ParseSelector("  THUMBNAILS , Exports ", "")

	c.Assert(err, qt.IsNil)
	c.Assert(set.Has(workers.WorkerExports), qt.IsTrue)
	c.Assert(set.Has(workers.WorkerThumbnails), qt.IsTrue)
	c.Assert(set.Has(workers.WorkerEmails), qt.IsFalse)
}

func TestParseSelector_ExcludeDropsIDs(t *testing.T) {
	c := qt.New(t)

	set, err := workers.ParseSelector("", "emails,token-cleanup")

	c.Assert(err, qt.IsNil)
	c.Assert(set.Has(workers.WorkerEmails), qt.IsFalse)
	c.Assert(set.Has(workers.WorkerTokenCleanup), qt.IsFalse)
	c.Assert(set.Has(workers.WorkerExports), qt.IsTrue)
	c.Assert(set.Has(workers.WorkerImports), qt.IsTrue)
	c.Assert(set.Has(workers.WorkerRestores), qt.IsTrue)
	c.Assert(set.Has(workers.WorkerThumbnails), qt.IsTrue)
}

func TestParseSelector_MutuallyExclusive(t *testing.T) {
	c := qt.New(t)

	_, err := workers.ParseSelector("thumbnails", "emails")

	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "mutually exclusive")
}

func TestParseSelector_UnknownIdentifier(t *testing.T) {
	cases := []struct {
		name string
		only string
		excl string
		flag string
	}{
		{name: "unknown in --workers-only", only: "thumbnails,bogus", excl: "", flag: "--workers-only"},
		{name: "unknown in --workers-exclude", only: "", excl: "bogus", flag: "--workers-exclude"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			_, err := workers.ParseSelector(tc.only, tc.excl)

			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.flag)
			c.Assert(err.Error(), qt.Contains, `"bogus"`)
		})
	}
}

func TestParseSelector_EmptyIdentifierInList(t *testing.T) {
	c := qt.New(t)

	_, err := workers.ParseSelector("thumbnails,,exports", "")

	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "empty identifier")
}

func TestParseSelector_ExcludingEveryIDProducesEmptySet(t *testing.T) {
	c := qt.New(t)

	all := workers.AllWorkerIDs()
	excludeParts := make([]string, len(all))
	for i, id := range all {
		excludeParts[i] = string(id)
	}

	set, err := workers.ParseSelector("", strings.Join(excludeParts, ","))

	c.Assert(err, qt.IsNil)
	c.Assert(set, qt.HasLen, 0)
}

func TestAllWorkerIDs_StableOrder(t *testing.T) {
	c := qt.New(t)

	c.Assert(workers.AllWorkerIDs(), qt.DeepEquals, []workers.WorkerID{
		workers.WorkerEmails,
		workers.WorkerExports,
		workers.WorkerImports,
		workers.WorkerRestores,
		workers.WorkerThumbnails,
		workers.WorkerTokenCleanup,
	})
}
