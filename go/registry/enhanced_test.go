package registry_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry"
)

func TestCapabilityMatrix(t *testing.T) {
	t.Run("PostgreSQL capabilities", func(t *testing.T) {
		c := qt.New(t)

		caps, exists := registry.GetCapabilities("postgres")
		c.Assert(exists, qt.IsTrue)
		c.Assert(caps.FullTextSearch, qt.IsTrue)
		c.Assert(caps.JSONBOperators, qt.IsTrue)
		c.Assert(caps.AdvancedIndexing, qt.IsTrue)
		c.Assert(caps.Triggers, qt.IsTrue)
		c.Assert(caps.StoredProcedures, qt.IsTrue)
		c.Assert(caps.BulkOperations, qt.IsTrue)
		c.Assert(caps.Transactions, qt.IsTrue)
		c.Assert(caps.ArrayOperations, qt.IsTrue)
	})

	t.Run("BoltDB capabilities", func(t *testing.T) {
		c := qt.New(t)

		caps, exists := registry.GetCapabilities("boltdb")
		c.Assert(exists, qt.IsTrue)
		c.Assert(caps.FullTextSearch, qt.IsFalse)
		c.Assert(caps.JSONBOperators, qt.IsFalse)
		c.Assert(caps.AdvancedIndexing, qt.IsFalse)
		c.Assert(caps.Triggers, qt.IsFalse)
		c.Assert(caps.StoredProcedures, qt.IsFalse)
		c.Assert(caps.BulkOperations, qt.IsFalse)
		c.Assert(caps.Transactions, qt.IsTrue) // BoltDB supports transactions
		c.Assert(caps.ArrayOperations, qt.IsFalse)
	})

	t.Run("Memory capabilities", func(t *testing.T) {
		c := qt.New(t)

		caps, exists := registry.GetCapabilities("memory")
		c.Assert(exists, qt.IsTrue)
		c.Assert(caps.FullTextSearch, qt.IsFalse)
		c.Assert(caps.JSONBOperators, qt.IsFalse)
		c.Assert(caps.AdvancedIndexing, qt.IsFalse)
		c.Assert(caps.Triggers, qt.IsFalse)
		c.Assert(caps.StoredProcedures, qt.IsFalse)
		c.Assert(caps.BulkOperations, qt.IsFalse)
		c.Assert(caps.Transactions, qt.IsFalse)
		c.Assert(caps.ArrayOperations, qt.IsFalse)
	})

	t.Run("Unknown database", func(t *testing.T) {
		c := qt.New(t)

		_, exists := registry.GetCapabilities("unknown")
		c.Assert(exists, qt.IsFalse)
	})
}

func TestSupportsFeature(t *testing.T) {
	t.Run("PostgreSQL supports full-text search", func(t *testing.T) {
		c := qt.New(t)

		supports := registry.SupportsFeature("postgres", func(caps registry.DatabaseCapabilities) bool {
			return caps.FullTextSearch
		})
		c.Assert(supports, qt.IsTrue)
	})

	t.Run("BoltDB does not support full-text search", func(t *testing.T) {
		c := qt.New(t)

		supports := registry.SupportsFeature("boltdb", func(caps registry.DatabaseCapabilities) bool {
			return caps.FullTextSearch
		})
		c.Assert(supports, qt.IsFalse)
	})

	t.Run("Unknown database does not support any features", func(t *testing.T) {
		c := qt.New(t)

		supports := registry.SupportsFeature("unknown", func(caps registry.DatabaseCapabilities) bool {
			return caps.FullTextSearch
		})
		c.Assert(supports, qt.IsFalse)
	})
}

func TestFeatureMatrix(t *testing.T) {
	c := qt.New(t)

	features := registry.GetFeatureMatrix()
	c.Assert(len(features), qt.Equals, 8) // We have 8 features defined

	// Check that all expected features are present
	featureNames := make(map[string]bool)
	for _, feature := range features {
		featureNames[feature.Feature] = true
	}

	expectedFeatures := []string{
		"FullTextSearch",
		"JSONBOperators",
		"AdvancedIndexing",
		"Triggers",
		"StoredProcedures",
		"BulkOperations",
		"Transactions",
		"ArrayOperations",
	}

	for _, expected := range expectedFeatures {
		c.Assert(featureNames[expected], qt.IsTrue, qt.Commentf("Feature %s should be present", expected))
	}

	// Check that PostgreSQL supports all features
	for _, feature := range features {
		c.Assert(feature.Backends["postgres"], qt.IsTrue, qt.Commentf("PostgreSQL should support %s", feature.Feature))
	}

	// Check that memory supports no advanced features
	for _, feature := range features {
		if feature.Feature != "Transactions" { // Memory doesn't even support transactions
			c.Assert(feature.Backends["memory"], qt.IsFalse, qt.Commentf("Memory should not support %s", feature.Feature))
		}
	}
}

func TestSearchOptions(t *testing.T) {
	t.Run("WithLimit option", func(t *testing.T) {
		c := qt.New(t)

		opts := &registry.SearchOptions{}
		registry.WithLimit(50)(opts)
		c.Assert(opts.Limit, qt.Equals, 50)
	})

	t.Run("WithOffset option", func(t *testing.T) {
		c := qt.New(t)

		opts := &registry.SearchOptions{}
		registry.WithOffset(100)(opts)
		c.Assert(opts.Offset, qt.Equals, 100)
	})

	t.Run("WithSort option", func(t *testing.T) {
		c := qt.New(t)

		opts := &registry.SearchOptions{}
		registry.WithSort("name", "DESC")(opts)
		c.Assert(opts.SortBy, qt.Equals, "name")
		c.Assert(opts.Order, qt.Equals, "DESC")
	})

	t.Run("Multiple options", func(t *testing.T) {
		c := qt.New(t)

		opts := &registry.SearchOptions{}
		registry.WithLimit(25)(opts)
		registry.WithOffset(50)(opts)
		registry.WithSort("created_at", "ASC")(opts)

		c.Assert(opts.Limit, qt.Equals, 25)
		c.Assert(opts.Offset, qt.Equals, 50)
		c.Assert(opts.SortBy, qt.Equals, "created_at")
		c.Assert(opts.Order, qt.Equals, "ASC")
	})
}

func TestTagOperators(t *testing.T) {
	c := qt.New(t)

	c.Assert(string(registry.TagOperatorAND), qt.Equals, "AND")
	c.Assert(string(registry.TagOperatorOR), qt.Equals, "OR")
}

func TestIndexSpec(t *testing.T) {
	c := qt.New(t)

	spec := registry.IndexSpec{
		Name:      "test_idx",
		Table:     "test_table",
		Column:    "test_column",
		Type:      "gin_jsonb",
		Condition: "test_column IS NOT NULL",
	}

	c.Assert(spec.Name, qt.Equals, "test_idx")
	c.Assert(spec.Table, qt.Equals, "test_table")
	c.Assert(spec.Column, qt.Equals, "test_column")
	c.Assert(spec.Type, qt.Equals, "gin_jsonb")
	c.Assert(spec.Condition, qt.Equals, "test_column IS NOT NULL")
}
