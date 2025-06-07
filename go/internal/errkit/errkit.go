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
	prev   *multiError
}

func (e *Error) Error() string {
	switch {
	case e.msg != "" && len(e.fields) > 0:
		return fmt.Sprintf("%s: %s (%+v)", e.msg, e.error.Error(), mapToString(e.fields))
	case e.msg == "" && len(e.fields) > 0:
		return fmt.Sprintf("%s (%+v)", e.error.Error(), mapToString(e.fields))
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

	if e.prev != nil && e.prev.Is(target) {
		return true
	}

	return errors.Is(e.error, target)
}

func (e *Error) As(target any) bool {
	if target == nil {
		return false
	}

	if e.prev != nil && e.prev.As(target) {
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

func (e *Error) WithFields(fields ...any) *Error {
	tmp := *e

	if len(fields) == 0 {
		return &tmp
	}

	tmp.fields = make(Fields)

	for k, v := range e.fields {
		tmp.fields[k] = v
	}
	for k, v := range ToFields(fields) {
		tmp.fields[k] = v
	}
	if tmp.prev == nil {
		tmp.prev = &multiError{}
	}
	tmp.prev.errs = append(tmp.prev.errs, e)

	return &tmp
}

func (e *Error) WithField(key string, value any) *Error {
	tmp := *e
	tmp.fields = make(Fields)
	for k, v := range e.fields {
		tmp.fields[k] = v
	}
	tmp.fields[key] = value
	if tmp.prev == nil {
		tmp.prev = &multiError{}
	}
	tmp.prev.errs = append(tmp.prev.errs, e)
	return &tmp
}

func (e *Error) WithEquivalents(errs ...error) *Error {
	tmp := *e
	tmp.error = WithEquivalents(e.error, errs...)
	if tmp.prev == nil {
		tmp.prev = &multiError{}
	}
	tmp.prev.errs = append(tmp.prev.errs, e)
	return &tmp
}

func (e *Error) Unwrap() error {
	return e.error
}

func NewEquivalent(msg string, errs ...error) error {
	// remove nil errors from the list
	newErrs := make([]error, 0, len(errs))
	for _, err := range errs {
		if err != nil {
			newErrs = append(newErrs, err)
		}
	}
	errs = newErrs

	return WithEquivalents(errors.New(msg), errs...)
}

func Wrap(err error, msg string, fields ...any) *Error {
	return &Error{
		error:  WithStack(err),
		msg:    msg,
		fields: ToFields(fields),
	}
}

func WithMessage(err error, msg string, fields ...any) *Error {
	return &Error{
		error:  err,
		msg:    msg,
		fields: ToFields(fields),
	}
}

func WithFields(err error, fields ...any) *Error {
	result := &Error{
		error:  err,
		fields: ToFields(fields),
	}
	return result
}
