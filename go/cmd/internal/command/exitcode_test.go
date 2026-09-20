package command_test

import (
	"errors"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/cmd/internal/command"
)

func TestExitCodeFor(t *testing.T) {
	c := qt.New(t)
	sentinel := errors.New("dirty")

	c.Run("nil is success", func(c *qt.C) {
		c.Assert(command.ExitCodeFor(nil), qt.Equals, 0)
	})

	c.Run("a plain error is the generic failure", func(c *qt.C) {
		c.Assert(command.ExitCodeFor(errors.New("boom")), qt.Equals, command.ExitCodeFailure)
	})

	c.Run("an exit-coded error keeps its status through wrapping", func(c *qt.C) {
		err := fmt.Errorf("outer: %w", command.WithExitCode(sentinel, 3))
		c.Assert(command.ExitCodeFor(err), qt.Equals, 3)
		c.Assert(err, qt.ErrorIs, sentinel,
			qt.Commentf("the wrapper must not hide the cause"))
	})

	c.Run("a zero code falls back to the generic failure", func(c *qt.C) {
		c.Assert(command.ExitCodeFor(command.WithExitCode(sentinel, 0)), qt.Equals, command.ExitCodeFailure)
	})

	c.Run("wrapping nil stays nil", func(c *qt.C) {
		c.Assert(command.WithExitCode(nil, 3), qt.IsNil)
	})
}
