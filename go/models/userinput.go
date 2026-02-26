package models

import (
	"errors"
	"reflect"
)

// SanitizeUserInput sets zero values for all struct fields that have userinput:"false" tag.
// This function processes embedded structs and child structs recursively, ensuring that
// system-controlled fields cannot be set by user input.
//
// The function handles:
//   - Embedded structs (anonymous fields)
//   - Child structs (non-anonymous struct fields)
//   - Pointers to structs (both embedded and child)
//   - Deeply nested structures
//
// Fields with userinput:"false" tag are set to their zero values:
//   - strings become ""
//   - numbers become 0
//   - booleans become false
//   - pointers become nil
//   - etc.
//
// Usage example:
//
//	type User struct {
//	    Name     string `json:"name"`                    // user can set
//	    Email    string `json:"email"`                   // user can set
//	    ID       string `json:"id" userinput:"false"`    // system-controlled
//	    Created  time.Time `userinput:"false"`           // system-controlled
//	}
//
//	user := &User{Name: "John", Email: "john@example.com", ID: "malicious-id"}
//	SanitizeUserInput(user)
//	// Result: user.Name="John", user.Email="john@example.com", user.ID="", user.Created=zero
//
// entity must be a non-nil pointer to a struct.
func SanitizeUserInput[T any](entity *T) {
	v := reflect.ValueOf(entity)
	if v.IsNil() {
		panic("entity must be a non-nil pointer to struct")
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		panic("entity must be a non-nil pointer to struct")
	}

	sanitizeUserInputRecursive(v.Type(), v)
}

// sanitizeUserInputRecursive recursively processes struct fields and sets zero values
// for fields with userinput:"false" tag, including embedded structs and child structs.
func sanitizeUserInputRecursive(t reflect.Type, v reflect.Value) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldValue := v.Field(i)

		// Handle embedded structs
		if field.Anonymous {
			if err := handleEmbeddedStructForUserInput(field, fieldValue); err != nil {
				continue // skip fields that can't be processed
			}
			continue
		}

		// Check if field has userinput:"false" tag
		userInputTag := field.Tag.Get("userinput")
		if userInputTag == "false" {
			// Set field to zero value if it's settable
			if fieldValue.CanSet() {
				zeroValue := reflect.Zero(field.Type)
				fieldValue.Set(zeroValue)
			}
		} else {
			// Process child structs recursively (non-embedded structs)
			if err := handleChildStructForUserInput(field, fieldValue); err != nil {
				continue // skip fields that can't be processed
			}
		}
	}
}

// handleEmbeddedStructForUserInput processes an embedded struct field for user input processing.
func handleEmbeddedStructForUserInput(field reflect.StructField, fieldValue reflect.Value) error {
	// Handle embedded structs (anonymous fields)
	if field.Type.Kind() == reflect.Struct {
		sanitizeUserInputRecursive(field.Type, fieldValue)
		return nil
	}

	// Handle pointer to embedded structs
	if field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.Struct && !fieldValue.IsNil() {
		sanitizeUserInputRecursive(field.Type.Elem(), fieldValue.Elem())
		return nil
	}

	return errors.New("not a struct or pointer to struct")
}

// handleChildStructForUserInput processes a child struct field (non-embedded) for user input processing.
func handleChildStructForUserInput(field reflect.StructField, fieldValue reflect.Value) error {
	// Handle child structs (non-anonymous fields)
	if field.Type.Kind() == reflect.Struct {
		sanitizeUserInputRecursive(field.Type, fieldValue)
		return nil
	}

	// Handle pointer to child structs
	if field.Type.Kind() == reflect.Pointer && field.Type.Elem().Kind() == reflect.Struct && !fieldValue.IsNil() {
		sanitizeUserInputRecursive(field.Type.Elem(), fieldValue.Elem())
		return nil
	}

	return errors.New("not a struct or pointer to struct")
}
