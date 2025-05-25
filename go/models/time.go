package models

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jellydator/validation"
)

// PDate is an alias type for a pointer to a Date.
type PDate = *Date

var (
	_ validation.Validatable = (*Date)(nil)
	_ json.Marshaler         = (*Date)(nil)
	_ json.Unmarshaler       = (*Date)(nil)
)

const dateFormat = "2006-01-02"

// Date represents a date in the format "YYYY-MM-DD".
type Date string

// MarshalJSON marshals the Date to JSON.
func (d *Date) MarshalJSON() ([]byte, error) {
	if d == nil {
		return []byte("null"), nil
	}
	return json.Marshal(string(*d))
}

// UnmarshalJSON unmarshals the JSON data into the Date.
// It also validates the date format.
// If the date is not in the correct format, it returns an error.
// If the date is in the correct format, it sets the Date and returns nil.
func (d *Date) UnmarshalJSON(data []byte) error {
	var dateStr string
	if err := json.Unmarshal(data, &dateStr); err != nil {
		return err
	}
	*d = Date(dateStr)
	return d.ValidateWithContext(context.Background())
}

// Validate checks if the date is in the correct format.
func (d *Date) Validate() error {
	return ErrMustUseValidateWithContext
}

func (d *Date) ValidateWithContext(_ context.Context) error {
	if d == nil {
		return nil
	}

	_, err := time.Parse(dateFormat, string(*d))
	return err
}

// ToTime converts the Date to a time.Time. If the Date is nil, it returns a zero time.Time.
func (d *Date) ToTime() time.Time {
	if d == nil {
		return time.Time{}
	}

	result, _ := time.Parse(dateFormat, string(*d)) // we'll ignore the error here
	return result
}

// After returns true if d is after other. If both are nil, it returns false.
func (d *Date) After(other *Date) bool {
	if d == nil || other == nil {
		return false
	}

	return *d > *other
}

// Before returns true if d is before other. If both are nil, it returns false.
func (d *Date) Before(other *Date) bool {
	if d == nil || other == nil {
		return false
	}

	return *d < *other
}

// Equal returns true if both dates are equal. If both are nil, it returns true.
func (d *Date) Equal(other *Date) bool {
	switch {
	case d == nil && other == nil: // both are nil
		return true
	case d == nil || other == nil: // one is not nil, but the other is
		return false
	case *d == *other: // both are non-nil and equal
		return true
	default: // both are non-nil and not equal
		return false
	}
}

// ToPDate converts a Date to a PDate. If the Date is empty, it returns nil.
func ToPDate(d Date) PDate {
	if d == "" {
		return nil
	}
	return &d
}
