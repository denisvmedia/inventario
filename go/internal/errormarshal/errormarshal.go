package errormarshal

import (
	"encoding/json"
	"errors"
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

// Marshal marshals an error to JSON, replicating the errkit.ForceMarshalError structure.
// It wraps the error in {"error": {...}, "type": "..."} to match the previous JSON API format.
//
// The function attempts multiple strategies in order:
//  1. errx JSON marshaling (for errx-wrapped errors with attributes)
//  2. Standard JSON marshaling (only if error implements json.Marshaler)
//  3. Minimal error structure fallback (always succeeds)
//
// The function handles edge cases:
//   - Panics from errxjson.Marshal (e.g., validation.Errors with unhashable map types)
//   - Errors without MarshalJSON that would serialize to `{}`
//   - Unexpected marshaling failures of minimal structures (panics to surface issues)
func Marshal(err error) json.RawMessage {
	if err == nil {
		return json.RawMessage(`{"error":null,"type":"<nil>"}`)
	}

	// Try errx JSON marshaling first, but catch panics (e.g., from validation.Errors with unhashable types)
	var errxResult json.RawMessage
	func() {
		defer func() {
			recover() // Silently catch panic from errxjson.Marshal
		}()
		if data, e := errxjson.Marshal(err); e == nil {
			wrapped := jsonError{
				Error: data,
				Type:  fmt.Sprintf("%T", err),
			}
			if result, e := json.Marshal(wrapped); e == nil {
				errxResult = result
			}
		}
	}()
	if errxResult != nil {
		return errxResult
	}

	// Try standard JSON marshaling only if error implements json.Marshaler
	// This avoids marshaling errors to `{}` for standard errors without MarshalJSON
	if _, ok := err.(json.Marshaler); ok {
		if data, e := json.Marshal(err); e == nil {
			wrapped := jsonError{
				Error: data,
				Type:  fmt.Sprintf("%T", err),
			}
			if result, e := json.Marshal(wrapped); e == nil {
				return result
			}
		}
	}

	// Final fallback: minimal error structure (this should always succeed)
	minimal := jsonMinimalError{
		Msg:  err.Error(),
		Type: fmt.Sprintf("%T", err),
	}
	data, e := json.Marshal(minimal)
	if e != nil {
		// This is an unexpected situation - marshaling a simple struct failed
		// Panic to surface the issue rather than silently returning invalid JSON
		panic(fmt.Sprintf("failed to marshal minimal error structure: %v", e))
	}
	return data
}

// MustMarshal is like Marshal but panics if marshaling fails.
// This should never happen in practice as Marshal has a final fallback.
func MustMarshal(err error) json.RawMessage {
	result := Marshal(err)
	if result == nil {
		panic("errormarshal.Marshal returned nil")
	}
	return result
}

var (
	// ErrMarshalFailed is returned when marshaling fails unexpectedly
	ErrMarshalFailed = errors.New("failed to marshal error")
)
