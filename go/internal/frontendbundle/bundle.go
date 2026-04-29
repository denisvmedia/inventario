// Package frontendbundle holds the identifiers and validation for the
// "which embedded SPA do we serve at /" knob, shared by the apiserver
// handler and the run/bootstrap config so the two never drift.
//
// The dual-bundle window is part of epic #1397 (Vue → React rewrite); the
// cutover PR (#1423) deletes this package together with the legacy bundle
// itself.
package frontendbundle

import "fmt"

// Bundle identifiers. The active selection is driven by the
// --frontend-bundle CLI flag or the INVENTARIO_FRONTEND env var.
const (
	Legacy = "legacy"
	New    = "new"
)

// Valid enumerates every accepted bundle name. Surfaced for help text and
// validation messages so we don't repeat the list at each call site.
var Valid = []string{Legacy, New}

// Validate returns nil for a recognised bundle name and a helpful error
// otherwise. Called by the bootstrap to fail fast at startup; the apiserver
// handler treats it as defence-in-depth and falls back to the legacy bundle
// rather than 500-ing if validation is somehow bypassed.
func Validate(bundle string) error {
	switch bundle {
	case Legacy, New:
		return nil
	default:
		return fmt.Errorf("invalid frontend bundle %q: must be one of %v (set via --frontend-bundle or INVENTARIO_FRONTEND)", bundle, Valid)
	}
}
