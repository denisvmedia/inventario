package typekit

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// IsNil reports whether v is nil, including the case of an interface that
// carries a type but holds a nil pointer.
//
// `v == nil` answers false for that one: the value is not the nil interface,
// it is a non-nil interface describing a nil pointer. A pointer-receiver
// method called on it runs with a nil receiver and panics at the first field
// access — a long way from where the nil was handed over, and with a message
// that says nothing about which dependency was missing.
func IsNil(v any) bool {
	if v == nil {
		return true
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Map, reflect.Pointer, reflect.Slice, reflect.UnsafePointer:
		return rv.IsNil()
	default:
		return false
	}
}

func ZeroOfType[T any](t T) (zero T) {
	// Use reflection to create a new instance of the type
	val := reflect.ValueOf(t)

	// If it's a nil pointer, we need to create a new instance
	if val.Kind() == reflect.Pointer && val.IsNil() {
		// Create a new instance of the pointed-to type
		newVal := reflect.New(val.Type().Elem())
		result, _ := reflect.TypeAssert[T](newVal)
		return result
	}

	// For non-nil values, return a zero value of the same type
	return zero
}

// SetFieldByConfigfieldTag sets the struct field denoted by the "configfield" tag to the provided value if it matches.
// ptr must be a non-nil pointer to a struct, and the function returns an error if the field is not settable or not found,
// or when the value does not fit the field type (see assignValue).
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
			if err := assignValue(fieldValue, value); err != nil {
				return fmt.Errorf("cannot set field %q: %w", field.Name, err)
			}
			return nil
		}
	}
	return fmt.Errorf("cannot set field %q: %w", tag, ErrNoFieldWithTag)
}

// assignValue stores value in dst. A nil value clears dst. A value of the
// same kind is assigned or converted (a string into a named string type), a
// pointer field takes a fresh pointer to the value, and a pointer value
// fills a plain field. Anything else goes through a JSON round trip into
// dst's type: a float64 decoded from JSON fills an int field when it is a
// whole number, and the []any that encoding/json produces for a list fills
// a []int field. A value that does not fit returns ErrIncompatibleValue
// instead of panicking in reflect.Value.Convert.
func assignValue(dst reflect.Value, value any) error {
	target := dst.Type()
	if IsNil(value) {
		dst.Set(reflect.Zero(target))
		return nil
	}
	val := reflect.ValueOf(value)
	switch {
	case val.Type().AssignableTo(target):
		dst.Set(val)
		return nil
	case target.Kind() == reflect.Pointer && val.Kind() != reflect.Pointer:
		elem := reflect.New(target.Elem())
		if err := assignValue(elem.Elem(), value); err != nil {
			return err
		}
		dst.Set(elem)
		return nil
	case target.Kind() != reflect.Pointer && val.Kind() == reflect.Pointer:
		return assignValue(dst, val.Elem().Interface())
	case val.Kind() == target.Kind() && isScalarKind(target.Kind()) && val.Type().ConvertibleTo(target):
		dst.Set(val.Convert(target))
		return nil
	}

	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("%T into %s: %w", value, target, ErrIncompatibleValue)
	}
	out := reflect.New(target)
	if err := json.Unmarshal(raw, out.Interface()); err != nil {
		return fmt.Errorf("%T into %s: %w", value, target, ErrIncompatibleValue)
	}
	dst.Set(out.Elem())
	return nil
}

func isScalarKind(k reflect.Kind) bool {
	switch k {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
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

		// Handle regular fields with db tags. Skip fields with no tag
		// (`db` absent) and fields explicitly opted out via `db:"-"` —
		// the latter is the conventional sqlx/gorp marker for "this is
		// a relation/transient field, do not include in INSERT".
		// Without the `"-"` check, the column ends up in the query
		// with name `-` and the named-param binding either can't find
		// it or trips over `-` as a non-identifier character (e.g.
		// `RestoreOperation.Steps`, which surfaced on PR #1494 as
		// `failed to insert entity: could not find name`).
		dbTag := field.Tag.Get("db")
		if dbTag == "" || dbTag == "-" {
			continue
		}
		// `dbwrite:"false"` marks a column that SELECT reads back but no
		// INSERT or UPDATE may name: a column the database computes (a
		// generated column, which PostgreSQL refuses to have written), or
		// a legacy column the code must keep working without.
		if field.Tag.Get("dbwrite") == "false" {
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
