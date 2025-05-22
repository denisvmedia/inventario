package errkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
)

type stackTrace struct {
	funcName string
	fileName string
	line     int
}

var (
	_ error          = (*stackTracedError)(nil)
	_ json.Marshaler = (*stackTracedError)(nil)
)

type stackTracedError struct {
	err        error      // The wrapped error
	stackTrace stackTrace // Stack trace associated with the error
}

// WithStack creates a new error with a stack trace.
// It wraps the given error with the stack trace.
func WithStack(err error, fields ...any) error {
	stack, _ := getStackTrace(1)

	result := &stackTracedError{
		stackTrace: stack,
		err:        err,
	}

	if len(fields) == 0 {
		return result
	}

	return WithFields(result, fields)
}

// Error implements the error interface and returns the error message.
// It includes only the wrapped error message, without the stack trace.
func (e *stackTracedError) Error() string {
	return e.err.Error()
}

// Is implements the errors.Is interface and returns true if the target error is found.
// It checks if the target error matches the wrapped error.
func (e *stackTracedError) Is(target error) bool {
	return errors.Is(e.err, target)
}

// As implements the errors.As interface and returns true if the target error is found.
// It checks if the target error can be assigned to the wrapped error.
func (e *stackTracedError) As(target any) bool {
	return errors.As(e.err, target)
}

// Unwrap implements the errors.Wrapper interface and returns the wrapped error.
// It provides access to the original error.
func (e *stackTracedError) Unwrap() error {
	return e.err
}

// MarshalJSON implements the json.Marshaler interface and returns the serialized error.
// It serializes the error and its associated stack trace to JSON.
func (e *stackTracedError) MarshalJSON() ([]byte, error) {
	type jsonStackTrace struct {
		FuncName string `json:"funcName"`
		FilePos  string `json:"filePos"`
	}
	type jsonError struct {
		Error      json.RawMessage `json:"error"`                // Serialized error
		StackTrace jsonStackTrace  `json:"stackTrace,omitempty"` // Serialized stack trace
	}

	errData, err := MarshalError(e.err) // Assuming MarshalError is a custom function to serialize the error
	if err != nil {
		return nil, err
	}

	jerr := jsonError{
		Error: errData,
		StackTrace: jsonStackTrace{
			FuncName: e.stackTrace.funcName,
			FilePos:  fmt.Sprintf("%s:%d", e.stackTrace.fileName, e.stackTrace.line),
		},
	}

	return json.Marshal(jerr) // Serialize the error and stack trace to JSON
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
