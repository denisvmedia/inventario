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
		result, _ := newVal.Interface().(T)
		return result
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
				return fmt.Errorf("cannot set field %q: %w", field.Name, ErrUnsettableField)
			}
			val := reflect.ValueOf(value)
			switch {
			case field.Type.Kind() == reflect.Ptr && val.Kind() != reflect.Ptr:
				valPtr := reflect.New(field.Type.Elem())
				valPtr.Elem().Set(val.Convert(field.Type.Elem()))
				fieldValue.Set(valPtr)
			case field.Type.Kind() != reflect.Ptr && val.Kind() == reflect.Ptr:
				fieldValue.Set(val.Elem().Convert(field.Type))
			default:
				fieldValue.Set(val.Convert(field.Type))
			}
			return nil
		}
	}
	return fmt.Errorf("cannot set field %q: %w", tag, ErrNoFieldWithTag)
}

func GetFieldByConfigfieldTag[T any](ptr T, tag string) (any, error) {
	v := reflect.ValueOf(ptr)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return nil, errors.New("ptr must be a non-nil pointer to struct")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return nil, errors.New("ptr must point to a struct")
	}
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.Tag.Get("configfield") == tag {
			fieldValue := v.Field(i)
			if !fieldValue.CanInterface() {
				return nil, fmt.Errorf("cannot access field %s", field.Name)
			}
			return fieldValue.Interface(), nil
		}
	}
	return nil, fmt.Errorf("no field with configfield tag %q found", tag)
}

// StructToMap converts a struct to a map.
// Only exported fields are included.
// No deep conversion is done (i.e. if a field is a struct, it's not converted to a map).
func StructToMap[T any](ptr T) (map[string]any, error) {
	v := reflect.ValueOf(ptr)
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return nil, errors.New("ptr must be a non-nil pointer to struct")
		}
		v = v.Elem()
		if v.Kind() != reflect.Struct {
			return nil, errors.New("ptr must point to a struct")
		}
	case reflect.Struct:
		// v is already a struct, no need to do anything
	default:
		return nil, errors.New("ptr must be a pointer to struct")
	}

	t := v.Type()

	result := make(map[string]any)
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue
		}
		fieldValue := v.Field(i)
		if !fieldValue.CanInterface() {
			return nil, fmt.Errorf("cannot access field %s", field.Name)
		}
		result[field.Tag.Get("configfield")] = fieldValue.Interface()
	}
	return result, nil
}
