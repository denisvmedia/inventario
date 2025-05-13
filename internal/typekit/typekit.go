package typekit

import (
	"errors"
	"fmt"
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

// SetFieldByConfigfieldTag sets the struct field denoted by the "configfield" tag to the provided value if it matches.
// ptr must be a non-nil pointer to a struct, and the function returns an error if the field is not settable or not found.
func SetFieldByConfigfieldTag(ptr any, tag string, value any) error {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return errors.New("ptr must be a non-nil pointer to struct")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return errors.New("ptr must point to a struct")
	}
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Tag.Get("configfield") == tag {
			fieldValue := v.Field(i)
			if !fieldValue.CanSet() {
				return fmt.Errorf("cannot set field %s", field.Name)
			}
			val := reflect.ValueOf(value)
			// Преобразование значений с учетом указателей
			if field.Type.Kind() == reflect.Ptr && val.Kind() != reflect.Ptr {
				valPtr := reflect.New(field.Type.Elem())
				valPtr.Elem().Set(val.Convert(field.Type.Elem()))
				fieldValue.Set(valPtr)
			} else if field.Type.Kind() != reflect.Ptr && val.Kind() == reflect.Ptr {
				fieldValue.Set(val.Elem().Convert(field.Type))
			} else {
				fieldValue.Set(val.Convert(field.Type))
			}
			return nil
		}
	}
	return fmt.Errorf("no field with configfield tag %q found", tag)
}
