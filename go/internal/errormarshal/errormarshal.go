package errormarshal

import (
	"encoding"
	"encoding/json"
	"fmt"

	errxjson "github.com/go-extras/errx/json"
)

type jsonError struct {
	Error json.RawMessage `json:"error,omitempty"`
	Type  string          `json:"type,omitempty"`
}

type jsonMinimalError struct {
	Msg  string `json:"msg,omitempty"`
	Type string `json:"type,omitempty"`
}

// MarshalError marshals an error to JSON using a type-switch approach similar to errkit.
// It returns the marshaled bytes and an error if marshaling fails.
//
// The function handles errors in this priority order:
//  1. json.Marshaler - errors implementing MarshalJSON (e.g., validation.Errors)
//  2. encoding.TextMarshaler - errors implementing MarshalText
//  3. fmt.Stringer - errors implementing String()
//  4. nil errors
//  5. errx errors - using errxjson.Marshal (for errx-wrapped errors with attributes)
//  6. Default fallback - uses Error() method
func MarshalError(aerr error) ([]byte, error) {
	switch v := aerr.(type) {
	case json.Marshaler:
		// Use standard JSON marshaling for errors implementing json.Marshaler
		// (e.g., validation.Errors)
		data, err := v.MarshalJSON()
		if err != nil {
			return nil, err
		}
		jsonErr := jsonError{
			Error: data,
			Type:  fmt.Sprintf("%T", aerr),
		}
		return json.Marshal(&jsonErr)

	case encoding.TextMarshaler:
		data, err := v.MarshalText()
		if err != nil {
			return nil, err
		}
		jsonErr := jsonMinimalError{
			Msg:  string(data),
			Type: fmt.Sprintf("%T", aerr),
		}
		return json.Marshal(&jsonErr)

	case fmt.Stringer:
		jsonErr := jsonMinimalError{
			Msg:  v.String(),
			Type: fmt.Sprintf("%T", aerr),
		}
		return json.Marshal(&jsonErr)

	case nil:
		return json.Marshal(nil)

	default:
		// Try errxjson for errx errors (which don't implement json.Marshaler)
		if data, err := errxjson.Marshal(aerr); err == nil {
			jsonErr := jsonError{
				Error: data,
				Type:  fmt.Sprintf("%T", aerr),
			}
			return json.Marshal(&jsonErr)
		}

		// Fallback: minimal error structure
		jsonErr := jsonMinimalError{
			Msg:  aerr.Error(),
			Type: fmt.Sprintf("%T", v),
		}
		return json.Marshal(&jsonErr)
	}
}

// Marshal marshals an error to JSON, replicating the errkit.ForceMarshalError behavior.
// It always succeeds by falling back to a minimal error representation if marshaling fails.
func Marshal(err error) json.RawMessage {
	data, e := MarshalError(err)
	if e != nil {
		// Fallback: create minimal error structure
		minimal := &jsonMinimalError{
			Msg:  err.Error(),
			Type: fmt.Sprintf("%T", err),
		}
		data, _ = json.Marshal(minimal)
	}
	return data
}

// MustMarshal is like Marshal but never fails - it always returns a valid JSON representation.
func MustMarshal(err error) json.RawMessage {
	return Marshal(err)
}
