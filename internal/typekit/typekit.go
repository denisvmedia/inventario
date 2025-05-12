package typekit

import (
	"reflect"
)

func ZeroOfType[T any](t T) (zero T) {
	// Use reflection to create a new instance of the type
	val := reflect.ValueOf(t)

	// If it's a nil pointer, we need to create a new instance
	if val.Kind() == reflect.Ptr && val.IsNil() {
		// Create a new instance of the pointed-to type
		newVal := reflect.New(val.Type().Elem())
		return newVal.Interface().(T)
	}

	// For non-nil values, return a zero value of the same type
	return zero
}
