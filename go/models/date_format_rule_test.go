package models_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func TestDateFormat(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		wantErr bool
	}{
		{name: "valid date", value: models.Date("2027-01-31")},
		{name: "empty is for Required to reject", value: models.Date("")},
		{name: "pointer to a valid date", value: new(models.Date("2027-01-31"))},
		{name: "nil pointer", value: (*models.Date)(nil)},
		{name: "plain string", value: "2027-01-31"},
		{name: "not a date", value: models.Date("not-a-date"), wantErr: true},
		{name: "US order", value: models.Date("01/31/2027"), wantErr: true},
		{name: "day out of range", value: models.Date("2027-02-31"), wantErr: true},
		{name: "timestamp, not a date", value: models.Date("2027-01-31T00:00:00Z"), wantErr: true},
		{name: "pointer to an invalid date", value: new(models.Date("nope")), wantErr: true},
		{name: "unrelated type is not this rule's business", value: 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			err := models.DateFormat.Validate(tt.value)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
				return
			}
			c.Assert(err, qt.IsNil)
		})
	}
}
