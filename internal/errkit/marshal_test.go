package errkit_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/errkit"
)

func TestForceMarshalError(t *testing.T) {
	t.Run("stderror", func(t *testing.T) {
		c := qt.New(t)

		err := errors.New("test error")
		data := errkit.ForceMarshalError(err)

		expectedJSON := `{"msg":"test error","type":"*errors.errorString"}`
		var expectedData any

		err = json.Unmarshal([]byte(expectedJSON), &expectedData)
		c.Assert(err, qt.IsNil)
		c.Assert(data, qt.JSONEquals, &expectedData)
	})

	t.Run("joined error", func(t *testing.T) {
		c := qt.New(t)

		err1 := errors.New("test error1")
		err2 := errors.New("test error2")
		err3 := errors.Join(err1, err2)
		data := errkit.ForceMarshalError(err3)

		expectedJSON := `{"error":[{"msg": "test error1", "type": "*errors.errorString"}, {"msg": "test error2", "type": "*errors.errorString"}],"type":"*errors.joinError"}`
		var expectedData any

		err := json.Unmarshal([]byte(expectedJSON), &expectedData)
		c.Assert(err, qt.IsNil)
		c.Assert(data, qt.JSONEquals, &expectedData)
	})

	t.Run("joined and appended error", func(t *testing.T) {
		c := qt.New(t)

		err1 := errors.New("test error1")
		err2 := errors.New("test error2")
		err3 := errors.Join(err1, err2)
		err4 := errkit.Append(err3, errors.New("test error3"))
		data, err := json.Marshal(err4)
		c.Assert(err, qt.IsNil)

		expectedJSON := `{"error":[{"msg": "test error1", "type": "*errors.errorString"}, {"msg": "test error2", "type": "*errors.errorString"}, {"msg": "test error3", "type": "*errors.errorString"}],"type":"*errkit.multiError"}`
		var expectedData any

		err = json.Unmarshal([]byte(expectedJSON), &expectedData)
		c.Assert(err, qt.IsNil)
		c.Assert(data, qt.JSONEquals, &expectedData)
	})

	t.Run("unmarshallable error", func(t *testing.T) {
		c := qt.New(t)

		v := &mockJSONMarshaler{
			Data: make(chan []byte, 1),
		}
		data := errkit.ForceMarshalError(v)

		expectedJSON := `{"msg":"error: type: chan []uint8","type":"*errkit_test.mockJSONMarshaler"}`
		var expectedData any

		err := json.Unmarshal([]byte(expectedJSON), &expectedData)
		c.Assert(err, qt.IsNil)
		c.Assert(data, qt.JSONEquals, &expectedData)
	})
}

func TestMarshalError_Error(t *testing.T) {
	type testcases struct {
		name         string
		err          error
		expectedJSON string
	}
	tests := []testcases{
		{
			name:         "stderror",
			err:          errors.New("test error"),
			expectedJSON: `{"msg":"test error","type":"*errors.errorString"}`,
		},
		{
			name:         "errkit.Error",
			err:          errkit.WithMessage(errors.New("test error"), "wrapped error"),
			expectedJSON: `{"msg":"wrapped error","error":{"msg":"test error","type":"*errors.errorString"}}`,
		},
		{
			name:         "errkit.Errors",
			err:          errkit.Append(errkit.WithMessage(errors.New("test error"), "wrapped error")),
			expectedJSON: `{"error":[{"msg":"wrapped error","error":{"msg":"test error","type":"*errors.errorString"}}],"type":"*errkit.multiError"}`,
		},
		{
			name:         "error struct",
			err:          &testError{},
			expectedJSON: `{"msg":"test error","type":"*errkit_test.testError"}`,
		},
		{
			name:         "nil error",
			err:          nil,
			expectedJSON: `null`,
		},
		{
			name:         "stringer",
			err:          &mockStringer{Str: "test error"},
			expectedJSON: `{"msg":"String:test error","type":"*errkit_test.mockStringer"}`,
		},
		{
			name:         "encoding.TextMarshaler",
			err:          &mockTextMarshaler{Data: []byte("test error")},
			expectedJSON: `{"msg":"text:test error","type":"*errkit_test.mockTextMarshaler"}`,
		},
		{
			name: "json.Marshaler",
			err: &mockJSONMarshaler{
				Data: map[string]any{
					"xxx": "yyy",
					"zzz": 123,
					"aaa": []any{"bbb", "ccc"},
				},
			},
			expectedJSON: `{"error":{"aaa":["bbb","ccc"],"xxx":"yyy","zzz":123},"type":"*errkit_test.mockJSONMarshaler"}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			data, err := errkit.MarshalError(tc.err)
			fmt.Println(string(data))
			c.Assert(err, qt.IsNil)
			var expectedData any
			err = json.Unmarshal([]byte(tc.expectedJSON), &expectedData)
			c.Assert(err, qt.IsNil)
			c.Assert(data, qt.JSONEquals, &expectedData)
		})
	}
}

// Mock types for testing MarshalError()

type mockJSONMarshaler struct {
	Data any
}

func (m *mockJSONMarshaler) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.Data)
}

func (m *mockJSONMarshaler) Error() string {
	return fmt.Sprintf("error: type: %T", m.Data)
}

type mockTextMarshaler struct {
	Data []byte
}

func (m *mockTextMarshaler) MarshalText() ([]byte, error) {
	data := []byte("text:" + string(m.Data))
	return data, nil
}

func (m *mockTextMarshaler) Error() string {
	return "error:" + string(m.Data)
}

type mockStringer struct {
	Str string
}

func (m *mockStringer) String() string {
	return "String:" + m.Str
}

func (m *mockStringer) Error() string {
	return "Error:" + m.Str
}
