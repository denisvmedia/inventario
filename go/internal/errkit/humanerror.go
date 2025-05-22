package errkit

import (
	"encoding/json"
)

type HumanError struct {
	msg     string
	details error
}

func NewHumanError(msg string, details error) *HumanError {
	return &HumanError{
		msg:     msg,
		details: details,
	}
}

func (e *HumanError) Error() string {
	return e.msg
}

func (e *HumanError) Unwrap() error {
	return e.details
}

func (e *HumanError) MarshalJSON() ([]byte, error) {
	type jsonError struct {
		Msg     string `json:"msg,omitempty"`
		Details error  `json:"details,omitempty"`
	}

	jsonErr := jsonError{
		Msg:     e.msg,
		Details: e.details,
	}

	return json.Marshal(jsonErr)
}
