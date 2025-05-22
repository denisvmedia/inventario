package errkit_test

import (
	"encoding/json"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/errkit"
)

func TestError_Error(t *testing.T) {
	c := qt.New(t)

	err := errors.New("some error")
	e := errkit.Wrap(err, "wrapped error")

	c.Assert(e.Error(), qt.Equals, "wrapped error: some error")
}

func TestError_MarshalJSON(t *testing.T) {
	c := qt.New(t)

	err := errors.New("inner error")
	e := errkit.WithMessage(err, "wrapped error")

	jsonBytes, err := json.Marshal(e)
	c.Assert(err, qt.IsNil)
	expectedJSON := `{"msg":"wrapped error","error":{"msg":"inner error","type":"*errors.errorString"}}`
	var expectedData map[string]any
	err = json.Unmarshal([]byte(expectedJSON), &expectedData)
	c.Assert(err, qt.IsNil)
	c.Assert(jsonBytes, qt.JSONEquals, &expectedData)
}

func TestError_WithFields(t *testing.T) {
	c := qt.New(t)

	err := errors.New("some error")
	e := errkit.WithMessage(err, "wrapped error")

	fields := errkit.Fields{
		"key1": "value1",
		"key2": 2,
	}

	newErr := e.WithFields(fields)

	c.Assert(newErr.Error(), qt.Equals, "wrapped error: some error (fields: map[key1:value1 key2:2])")
	c.Assert(newErr.WithField("key3", true).Error(), qt.Equals, "wrapped error: some error (fields: map[key1:value1 key2:2 key3:true])")
	c.Assert(newErr.WithField("key1", "updated").Error(), qt.Equals, "wrapped error: some error (fields: map[key1:updated key2:2])")

	data, err := json.Marshal(newErr)
	c.Assert(err, qt.IsNil)
	expectedJSON := `{
  "msg": "wrapped error",
  "error": {
    "msg": "some error",
    "type": "*errors.errorString"
  },
  "fields": {
    "key1": "value1",
    "key2": 2
  }
}`
	var expectedData map[string]any
	err = json.Unmarshal([]byte(expectedJSON), &expectedData)
	c.Assert(err, qt.IsNil)
	c.Assert(data, qt.JSONEquals, &expectedData)
}

func TestWrap(t *testing.T) {
	c := qt.New(t)

	c.Run("wrap error", func(c *qt.C) {
		err := errors.New("some error")
		e := errkit.Wrap(err, "wrapped error")

		c.Assert(e.Error(), qt.Equals, "wrapped error: some error")
	})

	c.Run("wrap with fields", func(c *qt.C) {
		err := errors.New("some error")
		e := errkit.Wrap(err, "wrapped error", "aaa", 123, "bbb", "test")

		c.Assert(e.Error(), qt.Equals, "wrapped error: some error (fields: map[aaa:123 bbb:test])")
	})
}

func TestWrapWithFields(t *testing.T) {
	c := qt.New(t)

	err := errors.New("some error")
	fields := errkit.Fields{
		"key1": "value1",
		"key2": 2,
	}

	e := errkit.Wrap(err, "wrapped error", fields)

	c.Assert(e.Error(), qt.Equals, "wrapped error: some error (fields: map[key1:value1 key2:2])")
}

func TestWithMessage(t *testing.T) {
	c := qt.New(t)

	err := errors.New("some error")
	e := errkit.WithMessage(err, "new message")

	c.Assert(e.Error(), qt.Equals, "new message: some error")
}

func TestWithFields(t *testing.T) {
	c := qt.New(t)

	err := errors.New("some error")
	fields := errkit.Fields{
		"key1": "value1",
		"key2": 2,
	}

	e := errkit.WithFields(err, fields)

	c.Assert(e.Error(), qt.Equals, "some error (fields: map[key1:value1 key2:2])")
}

func TestError_Unwrap(t *testing.T) {
	c := qt.New(t)

	err := errors.New("inner error")
	e := errkit.Wrap(err, "wrapped error")

	innerErr := e.Unwrap()

	c.Assert(innerErr, qt.ErrorIs, err)
}

func TestError_WithEquivalents(t *testing.T) {
	c := qt.New(t)

	err := errors.New("inner error")
	e := errkit.WithMessage(err, "wrapped error")

	err1 := errors.New("equivalent error 1")
	err2 := errors.New("equivalent error 2")

	newErr := e.WithEquivalents(err1, err2)

	c.Assert(newErr, qt.ErrorIs, err)
	c.Assert(newErr, qt.ErrorIs, newErr)
	c.Assert(newErr, qt.ErrorIs, err1)
	c.Assert(newErr, qt.ErrorIs, err2)

	_, err = json.Marshal(newErr)
	c.Assert(err, qt.IsNil)
	// fmt.Println(string(data))
}

type testError struct{}

func (*testError) Error() string {
	return "test error"
}
