package run_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/run"
)

func TestNew_WorkerSelectorFlagsAreBoundOnlyOnWorkersSubcommand(t *testing.T) {
	c := qt.New(t)

	cmd := run.New().Cmd()

	subcommands := map[string]bool{
		"all":       false,
		"apiserver": false,
		"workers":   false,
	}
	for _, sub := range cmd.Commands() {
		if _, ok := subcommands[sub.Name()]; !ok {
			continue
		}
		subcommands[sub.Name()] = true

		if sub.Name() == "workers" {
			c.Assert(sub.Flags().Lookup("workers-only"), qt.IsNotNil)
			c.Assert(sub.Flags().Lookup("workers-exclude"), qt.IsNotNil)
			continue
		}

		c.Assert(sub.Flags().Lookup("workers-only"), qt.IsNil)
		c.Assert(sub.Flags().Lookup("workers-exclude"), qt.IsNil)
	}

	for name, found := range subcommands {
		c.Assert(found, qt.IsTrue, qt.Commentf("subcommand %q should be registered", name))
	}
}
