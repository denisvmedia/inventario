package models_test

import (
	"database/sql/driver"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestTimestamp_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected models.Timestamp
		wantErr  bool
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: "",
			wantErr:  false,
		},
		{
			name:     "string value",
			input:    "2023-01-01T12:30:45Z",
			expected: "2023-01-01T12:30:45Z",
			wantErr:  false,
		},
		{
			name:     "byte slice value",
			input:    []byte("2023-01-01T12:30:45Z"),
			expected: "2023-01-01T12:30:45Z",
			wantErr:  false,
		},
		{
			name:     "time.Time value",
			input:    time.Date(2023, 1, 1, 12, 30, 45, 0, time.UTC),
			expected: "2023-01-01T12:30:45Z",
			wantErr:  false,
		},
		{
			name:     "invalid type",
			input:    123,
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			var ts models.Timestamp
			err := ts.Scan(tt.input)

			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(ts, qt.Equals, tt.expected)
			}
		})
	}
}

func TestTimestamp_Value(t *testing.T) {
	tests := []struct {
		name     string
		input    models.Timestamp
		expected driver.Value
	}{
		{
			name:     "empty timestamp",
			input:    "",
			expected: nil,
		},
		{
			name:     "valid timestamp",
			input:    "2023-01-01T12:30:45Z",
			expected: "2023-01-01T12:30:45Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			value, err := tt.input.Value()
			c.Assert(err, qt.IsNil)
			c.Assert(value, qt.Equals, tt.expected)
		})
	}
}

func TestDate_Scan(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected models.Date
		wantErr  bool
	}{
		{
			name:     "nil value",
			input:    nil,
			expected: "",
			wantErr:  false,
		},
		{
			name:     "string value",
			input:    "2023-01-01",
			expected: "2023-01-01",
			wantErr:  false,
		},
		{
			name:     "byte slice value",
			input:    []byte("2023-01-01"),
			expected: "2023-01-01",
			wantErr:  false,
		},
		{
			name:     "time.Time value",
			input:    time.Date(2023, 1, 1, 12, 30, 45, 0, time.UTC),
			expected: "2023-01-01",
			wantErr:  false,
		},
		{
			name:     "invalid type",
			input:    123,
			expected: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			var d models.Date
			err := d.Scan(tt.input)

			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
				c.Assert(d, qt.Equals, tt.expected)
			}
		})
	}
}

func TestDate_Value(t *testing.T) {
	tests := []struct {
		name     string
		input    models.Date
		expected driver.Value
	}{
		{
			name:     "empty date",
			input:    "",
			expected: nil,
		},
		{
			name:     "valid date",
			input:    "2023-01-01",
			expected: "2023-01-01",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			value, err := tt.input.Value()
			c.Assert(err, qt.IsNil)
			c.Assert(value, qt.Equals, tt.expected)
		})
	}
}
