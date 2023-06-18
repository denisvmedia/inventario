package errkit

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
)

type Fields = map[string]any

type stackTrace struct {
	funcName string
	fileName string
	line     int
}

type Error struct {
	error
	msg         string
	stackTrace  stackTrace
	fields      Fields
	equivalents []error
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: %s", e.msg, e.error.Error())
}

func (e *Error) MarshalJSON() ([]byte, error) {
	type jsonError struct {
		Msg        string          `json:"msg"`
		Func       string          `json:"func"`
		FilePos    string          `json:"filepos"`
		Fields     Fields          `json:"fields,omitempty"`
		Error      json.RawMessage `json:"error,omitempty"`
		ErrorExtra any             `json:"error_extra,omitempty"`
	}

	jerr := jsonError{
		Msg:     e.msg,
		Func:    e.stackTrace.funcName,
		FilePos: fmt.Sprintf("%s:%d", e.stackTrace.fileName, e.stackTrace.line),
		Fields:  e.fields,
	}

	if e.error == nil {
		return json.Marshal(jerr)
	}

	jerr.Error = ForceMarshalError(e.error)

	if _, ok := e.error.(json.Unmarshaler); !ok {
		nextErr := errors.Unwrap(e.error)
		jerr.ErrorExtra = nextErr
	}

	return json.Marshal(jerr)
}

func (e *Error) ChainWithFields(fields Fields) *Error {
	tmp := *e
	tmp.fields = make(Fields)
	for k, v := range e.fields {
		tmp.fields[k] = v
	}
	for k, v := range fields {
		tmp.fields[k] = v
	}
	return &tmp
}

func (e *Error) WithFields(fields Fields) error {
	return e.ChainWithFields(fields)
}

func (e *Error) ChainWithField(key string, value any) *Error {
	tmp := *e
	tmp.fields = make(Fields)
	for k, v := range e.fields {
		tmp.fields[k] = v
	}
	tmp.fields[key] = value
	return &tmp
}

func (e *Error) WithField(key string, value any) error {
	return e.ChainWithField(key, value)
}

func (e *Error) Unwrap() error {
	return e.error
}

func (e *Error) ChainWithEquivalent(err error) *Error {
	tmp := *e
	tmp.equivalents = make([]error, len(e.equivalents))
	copy(tmp.equivalents, e.equivalents)
	tmp.equivalents = append(tmp.equivalents, err)
	return &tmp
}

func (e *Error) WithEquivalent(err error) error {
	return e.ChainWithEquivalent(err)
}

func Wrap(err error, msg string) error {
	return newError(err, msg, 2)
}

func ChainWrap(err error, msg string) *Error {
	return newError(err, msg, 2)
}

func ChainWithEquivalent(err error, equivalent error) *Error {
	if err, ok := err.(*Error); ok {
		return err.ChainWithEquivalent(equivalent)
	}

	return ChainWrap(err, "").ChainWithEquivalent(equivalent)
}

func WithEquivalent(err error, equivalent error) error {
	return ChainWithEquivalent(err, equivalent)
}

func ForceMarshalError(e error) json.RawMessage {
	data, err := MarshalError(e)
	if err != nil {
		data, _ = json.Marshal(e.Error())
	}
	return data
}

func MarshalError(err error) (json.RawMessage, error) {
	switch v := err.(type) {
	case json.Marshaler:
		return v.MarshalJSON()
	case encoding.TextMarshaler:
		data, err := v.MarshalText()
		if err != nil {
			return nil, err
		}
		return json.Marshal(string(data))
	case fmt.Stringer:
		data := v.String()
		return json.Marshal(data)
	case nil:
		return json.Marshal(nil)
	default:
		data := err.Error()
		return json.Marshal(data)
	}
}

func newError(err error, msg string, skip int) *Error {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return &Error{
			error: errors.New("failed to retrieve caller information"),
			msg:   "failed to retrieve caller information",
		}
	}

	funcName := runtime.FuncForPC(pc).Name()
	fileName := filepath.Base(file)

	return &Error{
		error: err,
		msg:   msg,
		stackTrace: stackTrace{
			funcName: funcName,
			fileName: fileName,
			line:     line,
		},
	}
}

type fieldError struct {
	fields Fields
	err    error
}

func WithFields(err error, fields Fields) error {
	return &fieldError{
		fields: fields,
		err:    err,
	}
}

// Error implements the error interface and returns the error message.
func (e *fieldError) Error() string {
	return fmt.Sprintf("%s (%+v)", e.err.Error(), e.fields)
}

// Is implements the errors.Is interface and returns true if the target error is found.
func (e *fieldError) Is(target error) bool {
	return errors.Is(e.err, target)
}

// As implements the errors.As interface and returns true if the target error is found.
func (e *fieldError) As(target any) bool {
	return errors.As(e.err, target)
}

// Unwrap implements the errors.Wrapper interface and returns the wrapped error.
func (e *fieldError) Unwrap() error {
	return e.err
}

// MarshalJSON implements the json.Marshaler interface and returns the serialized error.
func (e *fieldError) MarshalJSON() ([]byte, error) {
	type jsonError struct {
		Error  json.RawMessage `json:"error"`
		Fields Fields          `json:"fields,omitempty"`
	}

	errData, err := MarshalError(e.err)
	if err != nil {
		return nil, err
	}

	jerr := jsonError{
		Error:  errData,
		Fields: e.fields,
	}

	return json.Marshal(jerr)
}
