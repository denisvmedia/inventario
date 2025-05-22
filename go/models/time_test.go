package models_test

import (
	"encoding/json"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestDate_Validate_HappyPath(t *testing.T) {
	t.Run("valid date", func(t *testing.T) {
		c := qt.New(t)

		date := models.Date("2023-01-01")
		err := date.Validate()
		c.Assert(err, qt.IsNil)
	})

	t.Run("nil date", func(t *testing.T) {
		c := qt.New(t)

		var date *models.Date
		err := date.Validate()
		c.Assert(err, qt.IsNil)
	})
}

func TestDate_Validate_UnhappyPaths(t *testing.T) {
	testCases := []struct {
		name string
		date models.Date
	}{
		{
			name: "invalid format",
			date: models.Date("01/01/2023"),
		},
		{
			name: "invalid date",
			date: models.Date("2023-13-01"),
		},
		{
			name: "empty date",
			date: models.Date(""),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			err := tc.date.Validate()
			c.Assert(err, qt.Not(qt.IsNil))
		})
	}
}

func TestDate_JSONMarshaling(t *testing.T) {
	c := qt.New(t)

	// Create a date
	date := models.Date("2023-01-01")

	// Marshal the date to JSON
	jsonData, err := json.Marshal(&date)
	c.Assert(err, qt.IsNil)
	c.Assert(string(jsonData), qt.Equals, `"2023-01-01"`)

	// Unmarshal the JSON back to a date
	var unmarshaledDate models.Date
	err = json.Unmarshal(jsonData, &unmarshaledDate)
	c.Assert(err, qt.IsNil)
	c.Assert(string(unmarshaledDate), qt.Equals, string(date))

	// Test nil date marshaling
	var nilDate *models.Date
	jsonData, err = json.Marshal(nilDate)
	c.Assert(err, qt.IsNil)
	c.Assert(string(jsonData), qt.Equals, "null")
}

func TestDate_JSONUnmarshaling_Error(t *testing.T) {
	c := qt.New(t)

	// Test unmarshaling invalid JSON
	var date models.Date
	err := json.Unmarshal([]byte(`"invalid-date"`), &date)
	c.Assert(err, qt.Not(qt.IsNil))

	// Test unmarshaling non-string JSON
	err = json.Unmarshal([]byte(`123`), &date)
	c.Assert(err, qt.Not(qt.IsNil))
}

func TestDate_ToTime(t *testing.T) {
	c := qt.New(t)

	// Test valid date conversion
	date := models.Date("2023-01-01")
	timeValue := date.ToTime()

	expectedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	c.Assert(timeValue.Year(), qt.Equals, expectedTime.Year())
	c.Assert(timeValue.Month(), qt.Equals, expectedTime.Month())
	c.Assert(timeValue.Day(), qt.Equals, expectedTime.Day())

	// Test nil date conversion
	var nilDate *models.Date
	timeValue = nilDate.ToTime()
	c.Assert(timeValue.IsZero(), qt.IsTrue)
}

func TestDate_Comparison(t *testing.T) {
	c := qt.New(t)

	date1 := models.Date("2023-01-01")
	date2 := models.Date("2023-01-02")
	date3 := models.Date("2023-01-01")
	var nilDate *models.Date

	// Test After
	c.Assert(date2.After(&date1), qt.IsTrue)
	c.Assert(date1.After(&date2), qt.IsFalse)
	c.Assert(date1.After(nilDate), qt.IsFalse)
	c.Assert(nilDate.After(&date1), qt.IsFalse)

	// Test Before
	c.Assert(date1.Before(&date2), qt.IsTrue)
	c.Assert(date2.Before(&date1), qt.IsFalse)
	c.Assert(date1.Before(nilDate), qt.IsFalse)
	c.Assert(nilDate.Before(&date1), qt.IsFalse)

	// Test Equal
	c.Assert(date1.Equal(&date3), qt.IsTrue)
	c.Assert(date1.Equal(&date2), qt.IsFalse)
	c.Assert(date1.Equal(nilDate), qt.IsFalse)
	c.Assert(nilDate.Equal(&date1), qt.IsFalse)

	// Test nil == nil case
	var nilDate2 *models.Date
	c.Assert(nilDate.Equal(nilDate2), qt.IsTrue)
}

func TestToPDate(t *testing.T) {
	c := qt.New(t)

	// Test with non-empty date
	date := models.Date("2023-01-01")
	pDate := models.ToPDate(date)
	c.Assert(pDate, qt.IsNotNil)
	c.Assert(string(*pDate), qt.Equals, string(date))

	// Test with empty date
	emptyDate := models.Date("")
	pDate = models.ToPDate(emptyDate)
	c.Assert(pDate, qt.IsNil)
}
