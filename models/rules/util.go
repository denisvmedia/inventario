package rules

import (
	"reflect"
	"strings"
)

func IsEmpty(value any) bool {
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
	}

	return false
}
