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

var (
	_ error          = (*Error)(nil)
	_ json.Marshaler = (*Error)(nil)
)

type Error struct {
	error
	msg        string
	stackTrace stackTrace
	fields     Fields
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

	switch (e.error).(type) {
	case json.Marshaler, encoding.TextMarshaler:
		// skip inner error, it should be marshaled by itself
	default:
		// try to marshal inner error, if any
		jerr.ErrorExtra = errors.Unwrap(e.error)
	}

	return json.Marshal(jerr)
}

func (e *Error) WithFields(fields Fields) *Error {
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

func (e *Error) WithField(key string, value any) *Error {
	tmp := *e
	tmp.fields = make(Fields)
	for k, v := range e.fields {
		tmp.fields[k] = v
	}
	tmp.fields[key] = value
	return &tmp
}

func (e *Error) Unwrap() error {
	return e.error
}

func Wrap(err error, msg string) *Error {
	return newError(err, msg, 2)
}

func getStackTrace(skip int) (stackTrace, error) {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return stackTrace{}, &Error{
			error: errors.New("failed to retrieve caller information"),
			msg:   "failed to retrieve caller information",
		}
	}

	funcName := runtime.FuncForPC(pc).Name()
	fileName := filepath.Base(file)

	return stackTrace{
		funcName: funcName,
		fileName: fileName,
		line:     line,
	}, nil
}

func newError(err error, msg string, skip int) *Error {
	stack, _ := getStackTrace(skip + 1)

	return &Error{
		error:      err,
		msg:        msg,
		stackTrace: stack,
	}
}
