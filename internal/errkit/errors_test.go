package errkit_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/errkit"
)

func TestErrors_Error(t *testing.T) {
	c := qt.New(t)

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errors.New("error 3")

	errs := errkit.Errors{err1, err2, err3}

	expectedMessage := strings.Join([]string{err1.Error(), err2.Error(), err3.Error()}, "\n")
	c.Assert(errs.Error(), qt.Equals, expectedMessage)
}

func TestErrors_Is(t *testing.T) {
	c := qt.New(t)

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errors.New("error 3")

	errs := errkit.Errors{err1, err2, err3}

	c.Assert(errs, qt.ErrorIs, err1)
	c.Assert(errs, qt.ErrorIs, err2)
	c.Assert(errs, qt.ErrorIs, err3)
	c.Assert(errs, qt.Not(qt.ErrorIs), errors.New("non-existent error"))
}

func TestErrors_As(t *testing.T) {
	c := qt.New(t)

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errkit.Wrap(errors.New("error 3"), "wrapped error 3")
	var targetErr = errkit.Errors{}
	var wrongTargetErr = &testError{}
	var targetErrkitErr = &errkit.Error{}

	errs := errkit.Errors{err1, err2, err3}

	c.Assert(errs, qt.ErrorAs, &targetErr)
	c.Assert(errs, qt.ErrorAs, &targetErrkitErr)
	c.Assert(errs, qt.Not(qt.ErrorAs), &wrongTargetErr)
}

func TestErrors_MarshalJSON(t *testing.T) {
	c := qt.New(t)

	err1 := errors.New("error 1")
	err2 := &testError{}
	err3 := errkit.WithMessage(errors.New("error 3"), "wrapped error 3")

	errs := errkit.Errors{err1, err2, err3}

	expectedJSON := `[
  {
    "msg": "error 1",
    "type": "*errors.errorString"
  },
  {
    "msg": "test error",
    "type": "*errkit_test.testError"
  },
  {
    "msg": "wrapped error 3",
    "error": {
      "msg": "error 3",
      "type": "*errors.errorString"
    }
  }
]`
	var expectedData []map[string]any
	err := json.Unmarshal([]byte(expectedJSON), &expectedData)
	c.Assert(err, qt.IsNil)
	data, err := errs.MarshalJSON()
	c.Assert(err, qt.IsNil)
	c.Assert(string(data), qt.JSONEquals, &expectedData)
}

func TestAppend(t *testing.T) {
	c := qt.New(t)

	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errors.New("error 3")
	err4 := errors.New("error 4")
	err5 := errors.New("error 5")

	// Append to existing Errors slice
	existingErrs := errkit.Errors{err1, err2}
	result := errkit.Append(existingErrs, err3, err4).(errkit.Errors)
	c.Assert(result, qt.HasLen, 4)
	c.Assert(result[0], qt.Equals, err1)
	c.Assert(result[1], qt.Equals, err2)
	c.Assert(result[2], qt.Equals, err3)
	c.Assert(result[3], qt.Equals, err4)

	// Append to non-Errors error
	result = errkit.Append(err5, err1, err2).(errkit.Errors)
	c.Assert(result, qt.HasLen, 3)
	c.Assert(result[0], qt.Equals, err5)
	c.Assert(result[1], qt.Equals, err1)
	c.Assert(result[2], qt.Equals, err2)
}
