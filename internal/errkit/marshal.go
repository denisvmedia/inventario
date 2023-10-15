package errkit

import (
	"encoding"
	"encoding/json"
	"fmt"
)

func ForceMarshalError(e error) []byte {
	data, err := MarshalError(e)
	if err != nil {
		type jsonMinimalError struct {
			Msg  string `json:"msg,omitempty"`
			Type string `json:"type,omitempty"`
		}
		e := &jsonMinimalError{
			Msg:  e.Error(),
			Type: fmt.Sprintf("%T", e),
		}
		data, _ = json.Marshal(e)
	}
	return data
}

func MarshalError(aerr error) ([]byte, error) {
	type jsonError struct {
		Error json.RawMessage `json:"error,omitempty"`
		Type  string          `json:"type,omitempty"`
	}
	type jsonMinimalError struct {
		Msg  string `json:"msg,omitempty"`
		Type string `json:"type,omitempty"`
	}

	switch v := aerr.(type) {
	case *Error:
		return v.MarshalJSON()
	case *multiError:
		return marshalMultiple(v)
	case json.Marshaler:
		data, err := v.MarshalJSON()
		if err != nil {
			return nil, err
		}
		jsonErr := jsonError{
			Error: data,
			Type:  fmt.Sprintf("%T", v),
		}
		return json.Marshal(&jsonErr)
	case multipleErrors:
		return marshalMultiple(v)
	case encoding.TextMarshaler:
		data, err := v.MarshalText()
		if err != nil {
			return nil, err
		}
		jsonErr := jsonMinimalError{
			Msg:  string(data),
			Type: fmt.Sprintf("%T", v),
		}
		return json.Marshal(&jsonErr)
	case fmt.Stringer:
		jsonErr := jsonMinimalError{
			Msg:  v.String(),
			Type: fmt.Sprintf("%T", v),
		}
		return json.Marshal(&jsonErr)
	case nil:
		return json.Marshal(nil)
	default:
		jsonErr := jsonMinimalError{
			Msg:  aerr.Error(),
			Type: fmt.Sprintf("%T", v),
		}
		return json.Marshal(&jsonErr)
	}
}

func marshalMultiple(merrs multipleErrors) ([]byte, error) {
	type jsonError struct {
		Error json.RawMessage `json:"error,omitempty"`
		Type  string          `json:"type,omitempty"`
	}

	jsonErr := jsonError{
		Type: fmt.Sprintf("%T", merrs),
	}

	errs := merrs.Unwrap()
	rawErrs := make([]json.RawMessage, 0, len(errs))
	for _, uerr := range errs {
		data, err := MarshalError(uerr)
		if err != nil {
			return nil, err
		}
		rawErrs = append(rawErrs, data)
	}
	marshalled, err := json.Marshal(rawErrs)
	if err != nil {
		return nil, err
	}
	jsonErr.Error = marshalled

	return json.Marshal(jsonErr)
}
