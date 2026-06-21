package errormarshal

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"

	errxjson "github.com/go-extras/errx/json"
	"github.com/jellydator/validation"
)

type jsonError struct {
	Error json.RawMessage `json:"error,omitempty"`
	Type  string          `json:"type,omitempty"`
}

type jsonMinimalError struct {
	Msg  string `json:"msg,omitempty"`
	Type string `json:"type,omitempty"`
}

// jsonValidationError carries the usual rendered-message tree under `error`
// plus a parallel `errorCodes` tree (same shape) whose leaves hold the
// stable {code, params} of each field error, so the frontend can localize
// per-field validation messages by code. #1990
type jsonValidationError struct {
	Error      json.RawMessage `json:"error,omitempty"`
	ErrorCodes json.RawMessage `json:"errorCodes,omitempty"`
	Type       string          `json:"type,omitempty"`
}

// validationCodeTree mirrors a validation.Errors tree but maps each leaf to
// its stable {code, params} instead of the rendered English message. Nested
// validation.Errors (validated sub-structs, e.g. data.attributes.<field>)
// recurse so the shape matches the message tree the FE already walks. Plain
// errors (errors.New/fmt.Errorf returned from a By-validator) carry no stable
// code; their leaf keeps the message so the FE can fall back to it. #1990
func validationCodeTree(errs validation.Errors) map[string]any {
	out := make(map[string]any, len(errs))
	for field, e := range errs {
		var nested validation.Errors
		if errors.As(e, &nested) {
			out[field] = validationCodeTree(nested)
			continue
		}
		var verr validation.Error
		if errors.As(e, &verr) {
			leaf := map[string]any{"code": verr.Code()}
			// Omit empty params (the common case — required/blank errors carry
			// none) rather than emitting "params":null on every field.
			if params := verr.Params(); len(params) > 0 {
				leaf["params"] = params
			}
			out[field] = leaf
			continue
		}
		out[field] = map[string]any{"code": "", "message": e.Error()}
	}
	return out
}

// MarshalError marshals an error to JSON using a type-switch approach.
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
	// validation.Errors carries a stable per-field Code()+Params() that its
	// own MarshalJSON discards (rendering only the English message). Surface
	// them in a parallel `errorCodes` tree alongside the unchanged message
	// tree so the FE can localize field-validation messages by code. #1990
	var verrs validation.Errors
	if errors.As(aerr, &verrs) {
		msgData, err := verrs.MarshalJSON()
		if err != nil {
			return nil, err
		}
		codeData, err := json.Marshal(validationCodeTree(verrs))
		if err != nil {
			return nil, err
		}
		return json.Marshal(&jsonValidationError{
			Error:      msgData,
			ErrorCodes: codeData,
			Type:       fmt.Sprintf("%T", aerr),
		})
	}

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

// Marshal marshals an error to JSON.
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
