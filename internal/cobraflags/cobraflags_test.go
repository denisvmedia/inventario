package cobraflags_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/internal/cobraflags"
)

func TestStringFlag_Register(t *testing.T) {
	c := qt.New(t)

	cmd := &cobra.Command{}
	flag := &cobraflags.StringFlag{
		Name:  "name",
		Value: "default",
		Usage: "usage",
	}

	flag.Register(cmd)

	expectedValue := "test"
	cmd.SetArgs([]string{"--name", expectedValue})
	err := cmd.Execute()

	c.Assert(err, qt.IsNil)
	c.Assert(flag.GetString(), qt.Equals, expectedValue)
}

func TestRegister(t *testing.T) {
	c := qt.New(t)

	cmd := &cobra.Command{}
	flag := &cobraflags.StringFlag{
		Name:  "name",
		Value: "default",
		Usage: "usage",
	}

	cobraflags.Register(cmd, flag)

	expectedValue := "test"
	cmd.SetArgs([]string{"--name", expectedValue})
	err := cmd.Execute()

	c.Assert(err, qt.IsNil)
	c.Assert(flag.GetString(), qt.Equals, expectedValue)
}

func TestRegisterMap(t *testing.T) {
	c := qt.New(t)

	cmd := &cobra.Command{}
	flags := map[string]cobraflags.Flag{
		"name": &cobraflags.StringFlag{
			Name:  "name",
			Value: "default",
			Usage: "usage",
		},
	}

	cobraflags.RegisterMap(cmd, flags)

	expectedValue := "test"
	cmd.SetArgs([]string{"--name", expectedValue})
	err := cmd.Execute()

	c.Assert(err, qt.IsNil)
	c.Assert(flags["name"].GetString(), qt.Equals, expectedValue)
}
