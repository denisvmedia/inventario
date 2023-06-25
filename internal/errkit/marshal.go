package errkit

import (
	"encoding"
	"encoding/json"
	"fmt"
)

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
