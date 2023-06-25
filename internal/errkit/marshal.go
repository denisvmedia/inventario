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
	type jsonMinimalError struct {
		Msg  string `json:"msg,omitempty"`
		Type string `json:"type,omitempty"`
	}

	switch v := aerr.(type) {
	case *Error:
		return v.MarshalJSON()
	case json.Marshaler:
		type jsonError struct {
			Error json.RawMessage `json:"error,omitempty"`
			Type  string          `json:"type,omitempty"`
		}

		data, err := v.MarshalJSON()
		if err != nil {
			return nil, err
		}
		jsonErr := jsonError{
			Error: data,
			Type:  fmt.Sprintf("%T", v),
		}
		return json.Marshal(&jsonErr)
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
