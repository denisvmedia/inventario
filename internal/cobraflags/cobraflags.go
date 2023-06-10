package cobraflags

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/denisvmedia/inventario/internal/assert"
)

type flagGetter interface {
	GetString() string
	GetBool() bool
	GetInt() int
}

type Flag interface {
	Register(*cobra.Command)

	flagGetter
}

func Register(cmd *cobra.Command, flags ...Flag) {
	for _, flag := range flags {
		flag.Register(cmd)
	}
}

func RegisterMap(cmd *cobra.Command, flags map[string]Flag) {
	for _, flag := range flags {
		flag.Register(cmd)
	}
}

var _ Flag = (*StringFlag)(nil)

type StringFlag struct {
	Name  string
	Value string
	Usage string

	flagGetter
}

func (s *StringFlag) Register(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.String(s.Name, s.Value, s.Usage)
	assert.NoError(viper.BindPFlag(s.Name, flags.Lookup(s.Name)))
}

func (s *StringFlag) GetString() string {
	return viper.GetString(s.Name)
}
