package errkit_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/errkit"
)

func TestError_Wrap_MarshalJSON(t *testing.T) {
	c := qt.New(t)

	err := errors.New("some error")
	e := errkit.Wrap(err, "wrapped error")

	jsonBytes, err := json.Marshal(e)
	c.Assert(err, qt.IsNil)
	fmt.Println(string(jsonBytes))
	expectedJSON := `{
  "msg": "wrapped error",
  "error": {
    "error": {
      "error": {
        "msg": "some error",
        "type": "*errors.errorString"
      },
      "stackTrace": {
        "funcName": "github.com/denisvmedia/inventario/internal/errkit.WithStack",
        "filePos": "stacktracederr.go:30"
      }
    },
    "type": "*errkit.stackTracedError"
  }
}
`
	var expectedData map[string]any
	err = json.Unmarshal([]byte(expectedJSON), &expectedData)
	c.Assert(err, qt.IsNil)
	c.Assert(jsonBytes, qt.JSONEquals, &expectedData)
}
