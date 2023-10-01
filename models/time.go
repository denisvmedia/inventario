package models

import (
	"encoding/json"
	"time"

	"github.com/jellydator/validation"
)

type PDate = *Date

var (
	_ validation.Validatable = (*Date)(nil)
	_ json.Marshaler         = (*Date)(nil)
	_ json.Unmarshaler       = (*Date)(nil)
)

const dateFormat = "2006-01-02"

type Date string

func (d *Date) MarshalJSON() ([]byte, error) {
	if d == nil {
		return []byte("null"), nil
	}
	return json.Marshal(string(*d))
}

func (d *Date) UnmarshalJSON(data []byte) error {
	var dateStr string
	if err := json.Unmarshal(data, &dateStr); err != nil {
		return err
	}
	*d = Date(dateStr)
	return d.Validate()
}

func (d *Date) Validate() error {
	if d == nil {
		return nil
	}

	_, err := time.Parse(dateFormat, string(*d))
	return err
}

func (d *Date) ToTime() time.Time {
	if d == nil {
		return time.Time{}
	}

	result, _ := time.Parse(dateFormat, string(*d)) // we'll ignore the error here
	return result
}

func (d *Date) After(other *Date) bool {
	if d == nil || other == nil {
		return false
	}

	return *d > *other
}

func (d *Date) Before(other *Date) bool {
	if d == nil || other == nil {
		return false
	}

	return *d < *other
}

func (d *Date) Equal(other *Date) bool {
	if d == nil && other != nil {
		return false
	}

	if d != nil && other == nil {
		return false
	}

	return *d == *other
}

func ToPDate(d Date) PDate {
	if d == "" {
		return nil
	}
	return &d
}
