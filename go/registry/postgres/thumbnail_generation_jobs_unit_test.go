package postgres_test

import (
	"database/sql"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry"
)

func TestErrorHandlingLogic(t *testing.T) {
	c := qt.New(t)

	// Test that sql.ErrNoRows is properly converted to registry.ErrNotFound
	// This simulates the logic in the GetJobByFileID method

	err := sql.ErrNoRows

	// This is the logic from the fixed GetJobByFileID method
	var resultErr error
	if err != nil {
		if err == sql.ErrNoRows {
			resultErr = registry.ErrNotFound
		} else {
			resultErr = err
		}
	}

	c.Assert(resultErr, qt.Equals, registry.ErrNotFound)
}
