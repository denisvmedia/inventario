package apiserver_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/apiserver"
)

// #1314: POST /exports/{id}/restores asks the querier unconditionally, so a
// nil one is a panic on the first request that reaches it rather than a
// failure to start. Two integration tests were passing nil.
func TestAPIServer_RejectsANilRestoreStatusQuerier(t *testing.T) {
	c := qt.New(t)

	c.Assert(func() { apiserver.APIServer(apiserver.Params{}, nil) }, qt.PanicMatches,
		".*restoreStatus is required.*")
}
