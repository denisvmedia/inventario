package workers_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/run/workers"
)

func TestParseSelector_DefaultsToAll(t *testing.T) {
	c := qt.New(t)

	set, deprecated, err := workers.ParseSelector("", "")

	c.Assert(err, qt.IsNil)
	c.Assert(deprecated, qt.HasLen, 0)
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

			set, deprecated, err := workers.ParseSelector(tc.input, "")

			c.Assert(err, qt.IsNil)
			c.Assert(deprecated, qt.HasLen, 0)
			c.Assert(set.Sorted(), qt.DeepEquals, workers.AllWorkerIDs())
		})
	}
}

func TestParseSelector_OnlySelectsSubset(t *testing.T) {
	c := qt.New(t)

	set, deprecated, err := workers.ParseSelector("media,archive", "")

	c.Assert(err, qt.IsNil)
	c.Assert(deprecated, qt.HasLen, 0)
	c.Assert(set.Sorted(), qt.DeepEquals, []workers.WorkerID{
		workers.WorkerArchive,
		workers.WorkerMedia,
	})
}

func TestParseSelector_OnlyNormalizesCaseAndWhitespace(t *testing.T) {
	c := qt.New(t)

	set, deprecated, err := workers.ParseSelector("  MEDIA , Archive ", "")

	c.Assert(err, qt.IsNil)
	c.Assert(deprecated, qt.HasLen, 0)
	c.Assert(set.Has(workers.WorkerArchive), qt.IsTrue)
	c.Assert(set.Has(workers.WorkerMedia), qt.IsTrue)
	c.Assert(set.Has(workers.WorkerEmails), qt.IsFalse)
}

func TestParseSelector_ExcludeDropsIDs(t *testing.T) {
	c := qt.New(t)

	set, deprecated, err := workers.ParseSelector("", "emails,housekeeping")

	c.Assert(err, qt.IsNil)
	c.Assert(deprecated, qt.HasLen, 0)
	c.Assert(set.Has(workers.WorkerEmails), qt.IsFalse)
	c.Assert(set.Has(workers.WorkerHousekeeping), qt.IsFalse)
	c.Assert(set.Has(workers.WorkerArchive), qt.IsTrue)
	c.Assert(set.Has(workers.WorkerMedia), qt.IsTrue)
}

func TestParseSelector_MutuallyExclusive(t *testing.T) {
	c := qt.New(t)

	_, _, err := workers.ParseSelector("media", "emails")

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
		{name: "unknown in --workers-only", only: "media,bogus", excl: "", flag: "--workers-only"},
		{name: "unknown in --workers-exclude", only: "", excl: "bogus", flag: "--workers-exclude"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			_, _, err := workers.ParseSelector(tc.only, tc.excl)

			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.flag)
			c.Assert(err.Error(), qt.Contains, `"bogus"`)
		})
	}
}

func TestParseSelector_EmptyIdentifierInList(t *testing.T) {
	c := qt.New(t)

	_, _, err := workers.ParseSelector("media,,archive", "")

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

	set, deprecated, err := workers.ParseSelector("", strings.Join(excludeParts, ","))

	c.Assert(err, qt.IsNil)
	c.Assert(deprecated, qt.HasLen, 0)
	c.Assert(set, qt.HasLen, 0)
}

func TestParseSelector_LegacyAliasesMapToGroups(t *testing.T) {
	cases := []struct {
		name      string
		only      string
		wantGroup workers.WorkerID
	}{
		{name: "exports -> archive", only: "exports", wantGroup: workers.WorkerArchive},
		{name: "imports -> archive", only: "imports", wantGroup: workers.WorkerArchive},
		{name: "restores -> archive", only: "restores", wantGroup: workers.WorkerArchive},
		{name: "thumbnails -> media", only: "thumbnails", wantGroup: workers.WorkerMedia},
		{name: "token-cleanup -> housekeeping", only: "token-cleanup", wantGroup: workers.WorkerHousekeeping},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			set, deprecated, err := workers.ParseSelector(tc.only, "")

			c.Assert(err, qt.IsNil)
			c.Assert(set.Sorted(), qt.DeepEquals, []workers.WorkerID{tc.wantGroup})
			c.Assert(deprecated, qt.HasLen, 1)
			c.Assert(deprecated[0].Alias, qt.Equals, tc.only)
			c.Assert(deprecated[0].Canonical, qt.Equals, tc.wantGroup)
		})
	}
}

func TestParseSelector_LegacyAliasesCollapseToSingleGroup(t *testing.T) {
	c := qt.New(t)

	set, deprecated, err := workers.ParseSelector("exports,imports,restores", "")

	c.Assert(err, qt.IsNil)
	c.Assert(set.Sorted(), qt.DeepEquals, []workers.WorkerID{workers.WorkerArchive})
	c.Assert(deprecated, qt.HasLen, 3)
	for _, d := range deprecated {
		c.Assert(d.Canonical, qt.Equals, workers.WorkerArchive)
	}
}

func TestParseSelector_LegacyAliasesInExcludeAreReported(t *testing.T) {
	c := qt.New(t)

	set, deprecated, err := workers.ParseSelector("", "thumbnails,token-cleanup")

	c.Assert(err, qt.IsNil)
	c.Assert(set.Has(workers.WorkerMedia), qt.IsFalse)
	c.Assert(set.Has(workers.WorkerHousekeeping), qt.IsFalse)
	c.Assert(set.Has(workers.WorkerArchive), qt.IsTrue)
	c.Assert(set.Has(workers.WorkerEmails), qt.IsTrue)
	c.Assert(deprecated, qt.HasLen, 2)
}

func TestAllWorkerIDs_StableOrder(t *testing.T) {
	c := qt.New(t)

	c.Assert(workers.AllWorkerIDs(), qt.DeepEquals, []workers.WorkerID{
		workers.WorkerArchive,
		workers.WorkerEmails,
		workers.WorkerHousekeeping,
		workers.WorkerMedia,
	})
}
