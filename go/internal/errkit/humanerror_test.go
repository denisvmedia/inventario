package errkit_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/errkit"
)

func TestHumanError(t *testing.T) {
	c := qt.New(t)

	msg := "This is a human error"
	details := errkit.NewHumanError("Details of the human error", nil)

	c.Run("Create HumanError", func(c *qt.C) {
		err := errkit.NewHumanError(msg, details)
		c.Assert(err, qt.IsNotNil)
		c.Assert(err.Error(), qt.Equals, msg)
		c.Assert(err.Unwrap(), qt.Equals, details)
	})

	c.Run("MarshalJSON", func(c *qt.C) {
		err := errkit.NewHumanError(msg, details)
		jsonBytes, marshalErr := json.Marshal(err)
		c.Assert(marshalErr, qt.IsNil)
		c.Assert(string(jsonBytes), qt.Equals, `{"msg":"This is a human error","details":{"msg":"Details of the human error"}}`)
	})
}
