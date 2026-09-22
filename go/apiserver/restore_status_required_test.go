package apiserver_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/backup/restore"
	"go.5x5.cz/inventario/registry"
)

// #1314: POST /exports/{id}/restores asks the querier unconditionally, so a
// nil one is a panic on the first request that reaches it rather than a
// failure to start. Two integration tests were passing nil.
func TestAPIServer_RejectsANilRestoreStatusQuerier(t *testing.T) {
	c := qt.New(t)

	c.Assert(func() { apiserver.APIServer(apiserver.Params{}, nil) }, qt.PanicMatches,
		".*restoreStatus is required.*")
}

// The shape a wiring mistake actually takes: a *RegistryStatusQuerier declared
// and left unset on some branch. That is not the nil interface, so a bare
// `== nil` waves it through, and the failure arrives later as a nil
// dereference inside HasRunningRestores that names nothing.
func TestAPIServer_RejectsATypedNilRestoreStatusQuerier(t *testing.T) {
	c := qt.New(t)

	// The pointer is nil; the interface carrying it is not. staticcheck will
	// tell you the comparison `RestoreStatusQuerier(typedNil) == nil` is never
	// true, which is the defect stated as a proof.
	var typedNil *restore.RegistryStatusQuerier

	c.Assert(func() { apiserver.APIServer(apiserver.Params{}, typedNil) }, qt.PanicMatches,
		".*restoreStatus is required.*")
}

// A real querier is still accepted — the guard must reject a missing
// dependency, not every dependency.
func TestAPIServer_AcceptsARealRestoreStatusQuerier(t *testing.T) {
	c := qt.New(t)

	params, _, _ := newParams()
	c.Assert(func() {
		_ = apiserver.APIServer(params, restore.NewRegistryStatusQuerier(&registry.Set{}))
	}, qt.Not(qt.PanicMatches), ".*")

	c.Assert(func() {
		_ = apiserver.APIServer(params, restore.NoopStatusQuerier{})
	}, qt.Not(qt.PanicMatches), ".*")
}
