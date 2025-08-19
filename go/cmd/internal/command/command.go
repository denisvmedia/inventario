package command

import (
	"github.com/spf13/cobra"
)

type Command interface {
	Cmd() *cobra.Command
}

type Base struct {
	cmd *cobra.Command
}

func NewBase(cmd *cobra.Command) Base {
	return Base{cmd: cmd}
}

func (b *Base) Cmd() *cobra.Command {
	return b.cmd
}
