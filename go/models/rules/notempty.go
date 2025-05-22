package rules

import (
	"reflect"
	"strings"

	"github.com/jellydator/validation"
)

var (
	// ErrIsEmpty is the error that returns when a value is empty.
	ErrIsEmpty = validation.NewError("validation_is_empty", "cannot be blank")
)

var NotEmpty validation.Rule = notEmptyRule{}

type notEmptyRule struct{}

func (notEmptyRule) Validate(value any) error {
	if isEmpty(value) {
		return ErrIsEmpty
	}
	return nil
}

func isEmpty(value any) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice:
		return v.Len() == 0
	case reflect.String:
		val := v.String()
		return strings.TrimSpace(val) == ""
	case reflect.Ptr:
		return v.IsZero()
	}

	return false
}
