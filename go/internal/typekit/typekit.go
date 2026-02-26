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
	if val.Kind() == reflect.Pointer && val.IsNil() {
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
	if v.Kind() != reflect.Pointer || v.IsNil() {
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
			case field.Type.Kind() == reflect.Pointer && val.Kind() != reflect.Pointer:
				valPtr := reflect.New(field.Type.Elem())
				valPtr.Elem().Set(val.Convert(field.Type.Elem()))
				fieldValue.Set(valPtr)
			case field.Type.Kind() != reflect.Pointer && val.Kind() == reflect.Pointer:
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
	if v.Kind() != reflect.Pointer || v.IsNil() {
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
	case reflect.Pointer:
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

// validateAndDereferenceStruct validates that v is a struct or a pointer to a struct,
// and returns the dereferenced value if it's a pointer.
func validateAndDereferenceStruct(v reflect.Value) (reflect.Value, error) {
	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			return reflect.Value{}, errors.New("ptr must be a non-nil pointer to struct")
		}
		v = v.Elem()
		if v.Kind() != reflect.Struct {
			return reflect.Value{}, errors.New("ptr must point to a struct")
		}
	case reflect.Struct:
		// v is already a struct, no need to do anything
	default:
		return reflect.Value{}, errors.New("ptr must be a pointer to struct")
	}
	return v, nil
}

// handleEmbeddedStruct processes an embedded struct field and extracts its DB fields.
func handleEmbeddedStruct(field reflect.StructField, fieldValue reflect.Value, fields, placeholders *[]string, params map[string]any) error {
	// Handle embedded structs (anonymous fields)
	if field.Type.Kind() == reflect.Struct {
		return extractDBFields(field.Type, fieldValue, fields, placeholders, params)
	}

	// Handle pointer to embedded structs
	if field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.Struct && !fieldValue.IsNil() {
		return extractDBFields(field.Type.Elem(), fieldValue.Elem(), fields, placeholders, params)
	}

	return nil
}

// extractDBFields recursively extracts fields with db tags from a struct, including embedded structs.
func extractDBFields(t reflect.Type, v reflect.Value, fields, placeholders *[]string, params map[string]any) error {
	var err error
	v, err = validateAndDereferenceStruct(v)
	if err != nil {
		return err
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Handle embedded structs
		if field.Anonymous {
			if err := handleEmbeddedStruct(field, fieldValue, fields, placeholders, params); err != nil {
				return err
			}
			continue
		}

		// Handle regular fields with db tags
		dbTag := field.Tag.Get("db")
		if dbTag == "" {
			continue
		}

		*fields = append(*fields, dbTag)
		*placeholders = append(*placeholders, ":"+dbTag)
		params[dbTag] = fieldValue.Interface()
	}

	return nil
}

// ExtractDBFields extracts fields with db tags from a struct, including embedded structs.
func ExtractDBFields(entity any, fields, placeholders *[]string, params map[string]any) error {
	t := reflect.TypeOf(entity)
	v := reflect.ValueOf(entity)

	return extractDBFields(t, v, fields, placeholders, params)
}
