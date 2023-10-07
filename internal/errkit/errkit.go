package errkit

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
)

type Fields = map[string]any

var (
	_ error          = (*Error)(nil)
	_ json.Marshaler = (*Error)(nil)
)

type Error struct {
	error
	msg    string
	fields Fields
	prev   Errors
}

func (e *Error) Error() string {
	switch {
	case e.msg != "" && len(e.fields) > 0:
		return fmt.Sprintf("%s: %s (fields: %+v)", e.msg, e.error.Error(), e.fields)
	case e.msg == "" && len(e.fields) > 0:
		return fmt.Sprintf("%s (fields: %+v)", e.error.Error(), e.fields)
	case e.msg != "" && len(e.fields) == 0:
		return fmt.Sprintf("%s: %s", e.msg, e.error.Error())
	default:
		return e.error.Error()
	}
}

func (e *Error) Is(target error) bool {
	if target == nil {
		return false
	}

	if target == e {
		return true
	}

	if e.prev.Is(target) {
		return true
	}

	return errors.Is(e.error, target)
}

func (e *Error) As(target any) bool {
	if target == nil {
		return false
	}

	if e.prev.As(target) {
		return true
	}

	return errors.As(e.error, target)
}

func (e *Error) MarshalJSON() ([]byte, error) {
	type jsonError struct {
		Msg        string          `json:"msg,omitempty"`
		Func       string          `json:"func,omitempty"`
		FilePos    string          `json:"filepos,omitempty"`
		Fields     Fields          `json:"fields,omitempty"`
		Error      json.RawMessage `json:"error,omitempty"`
		ErrorExtra any             `json:"error_extra,omitempty"`
	}

	jerr := jsonError{
		Msg:    e.msg,
		Fields: e.fields,
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
	tmp.prev = append(tmp.prev, e)
	return &tmp
}

func (e *Error) WithField(key string, value any) *Error {
	tmp := *e
	tmp.fields = make(Fields)
	for k, v := range e.fields {
		tmp.fields[k] = v
	}
	tmp.fields[key] = value
	tmp.prev = append(tmp.prev, e)
	return &tmp
}

func (e *Error) WithEquivalents(errs ...error) *Error {
	tmp := *e
	tmp.error = WithEquivalents(e.error, errs...)
	tmp.prev = append(tmp.prev, e)
	return &tmp
}

func (e *Error) Unwrap() error {
	return e.error
}

func NewEquivalent(msg string, errs ...error) error {
	return WithEquivalents(errors.New(msg), errs...)
}

func Wrap(err error, msg string, fields ...any) *Error {
	return &Error{
		error:  WithStack(err),
		msg:    msg,
		fields: ToFields(fields),
	}
}

func WrapWithFields(err error, msg string, fields Fields) *Error {
	result := &Error{
		error:  WithStack(err),
		msg:    msg,
		fields: make(Fields, len(fields)),
	}
	for k, v := range fields {
		result.fields[k] = v
	}
	return result
}

func WithMessage(err error, msg string) *Error {
	return &Error{
		error: err,
		msg:   msg,
	}
}

func WithFieldMap(err error, fields Fields) *Error {
	result := &Error{
		error:  err,
		fields: make(Fields, len(fields)),
	}
	for k, v := range fields {
		result.fields[k] = v
	}
	return result
}

func WithFields(err error, fields ...any) *Error {
	result := &Error{
		error:  err,
		fields: ToFields(fields),
	}
	return result
}
