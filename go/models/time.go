package models

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jellydator/validation"
)

// PDate is an alias type for a pointer to a Date.
type PDate = *Date

// PTimestamp is an alias type for a pointer to a Timestamp.
type PTimestamp = *Timestamp

var (
	_ validation.Validatable = (*Date)(nil)
	_ json.Marshaler         = (*Date)(nil)
	_ json.Unmarshaler       = (*Date)(nil)
	_ driver.Valuer          = (*Date)(nil)
	_ sql.Scanner            = (*Date)(nil)
	_ validation.Validatable = (*Timestamp)(nil)
	_ json.Marshaler         = (*Timestamp)(nil)
	_ json.Unmarshaler       = (*Timestamp)(nil)
	_ driver.Valuer          = (*Timestamp)(nil)
	_ sql.Scanner            = (*Timestamp)(nil)
)

const dateFormat = "2006-01-02"
const timestampFormat = time.RFC3339

// Date represents a date in the format "YYYY-MM-DD".
type Date string

// Timestamp represents a timestamp in RFC3339 format.
type Timestamp string

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
func (*Date) Validate() error {
	return ErrMustUseValidateWithContext
}

func (d *Date) ValidateWithContext(_ context.Context) error {
	if d == nil {
		return nil
	}

	_, err := time.Parse(dateFormat, string(*d))
	return err
}

// DateFormat checks that a value holds a calendar date in YYYY-MM-DD. An
// empty value passes, so pair it with validation.Required when the field is
// mandatory.
//
// A non-pointer Date field needs this rule to be checked at all. Listing
// the field on its own does nothing: validation.Field dereferences the
// field pointer before asking whether the value satisfies Validatable, and
// Date's validator keeps a pointer receiver so that a nil PDate validates
// instead of panicking.
var DateFormat validation.Rule = dateFormatRule{}

type dateFormatRule struct{}

func (dateFormatRule) Validate(value any) error {
	var raw string
	switch v := value.(type) {
	case Date:
		raw = string(v)
	case *Date:
		if v == nil {
			return nil
		}
		raw = string(*v)
	case string:
		raw = v
	default:
		return nil
	}
	if raw == "" {
		return nil
	}
	_, err := time.Parse(dateFormat, raw)
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

// Scan implements the sql.Scanner interface for Date.
// It can scan from string, []byte, or time.Time values.
func (d *Date) Scan(value any) error {
	if value == nil {
		*d = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*d = Date(v)
	case []byte:
		*d = Date(v)
	case time.Time:
		*d = Date(v.Format(dateFormat))
	default:
		return fmt.Errorf("cannot scan %T into Date", value)
	}

	return nil
}

// Value implements the driver.Valuer interface for Date.
func (d Date) Value() (driver.Value, error) {
	if d == "" {
		return nil, nil
	}
	return string(d), nil
}

// MarshalJSON marshals the Timestamp to JSON.
func (t *Timestamp) MarshalJSON() ([]byte, error) {
	if t == nil {
		return []byte("null"), nil
	}
	return json.Marshal(string(*t))
}

// UnmarshalJSON unmarshals the JSON data into the Timestamp.
// It also validates the timestamp format.
// If the timestamp is not in the correct format, it returns an error.
// If the timestamp is in the correct format, it sets the Timestamp and returns nil.
func (t *Timestamp) UnmarshalJSON(data []byte) error {
	var timestampStr string
	if err := json.Unmarshal(data, &timestampStr); err != nil {
		return err
	}
	*t = Timestamp(timestampStr)
	return t.ValidateWithContext(context.Background())
}

// Validate checks if the timestamp is in the correct format.
func (*Timestamp) Validate() error {
	return ErrMustUseValidateWithContext
}

func (t *Timestamp) ValidateWithContext(_ context.Context) error {
	if t == nil {
		return nil
	}

	_, err := time.Parse(timestampFormat, string(*t))
	return err
}

// ToTime converts the Timestamp to a time.Time. If the Timestamp is nil, it returns a zero time.Time.
func (t *Timestamp) ToTime() time.Time {
	if t == nil {
		return time.Time{}
	}

	result, _ := time.Parse(timestampFormat, string(*t)) // we'll ignore the error here
	return result
}

// After returns true if t is after other. If both are nil, it returns false.
func (t *Timestamp) After(other *Timestamp) bool {
	if t == nil || other == nil {
		return false
	}

	return t.ToTime().After(other.ToTime())
}

// Before returns true if t is before other. If both are nil, it returns false.
func (t *Timestamp) Before(other *Timestamp) bool {
	if t == nil || other == nil {
		return false
	}

	return t.ToTime().Before(other.ToTime())
}

// Equal returns true if both timestamps are equal. If both are nil, it returns true.
func (t *Timestamp) Equal(other *Timestamp) bool {
	switch {
	case t == nil && other == nil: // both are nil
		return true
	case t == nil || other == nil: // one is not nil, but the other is
		return false
	case *t == *other: // both are non-nil and equal
		return true
	default: // both are non-nil and not equal
		return false
	}
}

// ToPTimestamp converts a Timestamp to a PTimestamp. If the Timestamp is empty, it returns nil.
func ToPTimestamp(t Timestamp) PTimestamp {
	if t == "" {
		return nil
	}
	return &t
}

// Scan implements the sql.Scanner interface for Timestamp.
// It can scan from string, []byte, or time.Time values.
func (t *Timestamp) Scan(value any) error {
	if value == nil {
		*t = ""
		return nil
	}

	switch v := value.(type) {
	case string:
		*t = Timestamp(v)
	case []byte:
		*t = Timestamp(v)
	case time.Time:
		*t = Timestamp(v.UTC().Format(timestampFormat))
	default:
		return fmt.Errorf("cannot scan %T into Timestamp", value)
	}

	return nil
}

// Value implements the driver.Valuer interface for Timestamp.
func (t Timestamp) Value() (driver.Value, error) {
	if t == "" {
		return nil, nil
	}
	return string(t), nil
}

// NewTimestamp creates a new Timestamp from a time.Time, in UTC.
//
// The offset does not survive storage: every timestamp column but one is
// `timestamp without time zone`, so PostgreSQL casts the RFC 3339 string and
// keeps the wall clock while dropping the zone. Written from a host at +02:00,
// 20:47+02:00 is stored as 20:47 and reads back as 20:47Z — two hours later
// than the instant it was meant to be, and two hours adrift from the `now()`
// an expiry check in SQL compares it against.
//
// Normalizing here rather than at each call site is what makes a round trip an
// identity. The shipped container is UTC, so stored data does not move. #2594
func NewTimestamp(t time.Time) Timestamp {
	return Timestamp(t.UTC().Format(timestampFormat))
}

// NewPTimestamp creates a new PTimestamp from a time.Time.
func NewPTimestamp(t time.Time) PTimestamp {
	ts := NewTimestamp(t)
	return &ts
}

// Now returns a new Timestamp representing the current time.
func Now() Timestamp {
	return NewTimestamp(time.Now())
}

// PNow returns a new PTimestamp representing the current time.
func PNow() PTimestamp {
	return NewPTimestamp(time.Now())
}
