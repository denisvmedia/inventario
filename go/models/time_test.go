package models_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestDate_Validate(t *testing.T) {
	c := qt.New(t)

	date := models.Date("2023-01-01")
	err := date.Validate()
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Equals, "must use validate with context")
}

func TestDate_ValidateWithContext_HappyPath(t *testing.T) {
	t.Run("valid date", func(t *testing.T) {
		c := qt.New(t)

		date := models.Date("2023-01-01")
		ctx := context.Background()
		err := date.ValidateWithContext(ctx)
		c.Assert(err, qt.IsNil)
	})

	t.Run("nil date", func(t *testing.T) {
		c := qt.New(t)

		var date *models.Date
		ctx := context.Background()
		err := date.ValidateWithContext(ctx)
		c.Assert(err, qt.IsNil)
	})
}

func TestDate_ValidateWithContext_UnhappyPaths(t *testing.T) {
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

			ctx := context.Background()
			err := tc.date.ValidateWithContext(ctx)
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

func TestTimestamp_Validate(t *testing.T) {
	c := qt.New(t)

	timestamp := models.Timestamp("2023-01-01T12:00:00Z")
	err := timestamp.Validate()
	c.Assert(err, qt.Not(qt.IsNil))
	c.Assert(err.Error(), qt.Equals, "must use validate with context")
}

func TestTimestamp_ValidateWithContext_HappyPath(t *testing.T) {
	t.Run("valid timestamp", func(t *testing.T) {
		c := qt.New(t)

		timestamp := models.Timestamp("2023-01-01T12:00:00Z")
		ctx := context.Background()
		err := timestamp.ValidateWithContext(ctx)
		c.Assert(err, qt.IsNil)
	})

	t.Run("nil timestamp", func(t *testing.T) {
		c := qt.New(t)

		var timestamp *models.Timestamp
		ctx := context.Background()
		err := timestamp.ValidateWithContext(ctx)
		c.Assert(err, qt.IsNil)
	})
}

func TestTimestamp_ValidateWithContext_UnhappyPaths(t *testing.T) {
	testCases := []struct {
		name      string
		timestamp models.Timestamp
	}{
		{
			name:      "invalid format",
			timestamp: models.Timestamp("01/01/2023 12:00:00"),
		},
		{
			name:      "invalid timestamp",
			timestamp: models.Timestamp("2023-13-01T12:00:00Z"),
		},
		{
			name:      "empty timestamp",
			timestamp: models.Timestamp(""),
		},
		{
			name:      "date only",
			timestamp: models.Timestamp("2023-01-01"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			ctx := context.Background()
			err := tc.timestamp.ValidateWithContext(ctx)
			c.Assert(err, qt.Not(qt.IsNil))
		})
	}
}

func TestTimestamp_JSONMarshaling(t *testing.T) {
	c := qt.New(t)

	// Create a timestamp
	timestamp := models.Timestamp("2023-01-01T12:00:00Z")

	// Marshal the timestamp to JSON
	jsonData, err := json.Marshal(&timestamp)
	c.Assert(err, qt.IsNil)
	c.Assert(string(jsonData), qt.Equals, `"2023-01-01T12:00:00Z"`)

	// Unmarshal the JSON back to a timestamp
	var unmarshaledTimestamp models.Timestamp
	err = json.Unmarshal(jsonData, &unmarshaledTimestamp)
	c.Assert(err, qt.IsNil)
	c.Assert(string(unmarshaledTimestamp), qt.Equals, string(timestamp))

	// Test nil timestamp marshaling
	var nilTimestamp *models.Timestamp
	jsonData, err = json.Marshal(nilTimestamp)
	c.Assert(err, qt.IsNil)
	c.Assert(string(jsonData), qt.Equals, "null")
}

func TestTimestamp_ToTime(t *testing.T) {
	c := qt.New(t)

	// Test valid timestamp conversion
	timestamp := models.Timestamp("2023-01-01T12:30:45Z")
	timeValue := timestamp.ToTime()

	expectedTime := time.Date(2023, 1, 1, 12, 30, 45, 0, time.UTC)
	c.Assert(timeValue.Equal(expectedTime), qt.IsTrue)

	// Test nil timestamp conversion
	var nilTimestamp *models.Timestamp
	timeValue = nilTimestamp.ToTime()
	c.Assert(timeValue.IsZero(), qt.IsTrue)
}

func TestTimestamp_Comparison(t *testing.T) {
	c := qt.New(t)

	timestamp1 := models.Timestamp("2023-01-01T12:00:00Z")
	timestamp2 := models.Timestamp("2023-01-01T13:00:00Z")
	timestamp3 := models.Timestamp("2023-01-01T12:00:00Z")
	var nilTimestamp *models.Timestamp

	// Test After
	c.Assert(timestamp2.After(&timestamp1), qt.IsTrue)
	c.Assert(timestamp1.After(&timestamp2), qt.IsFalse)
	c.Assert(timestamp1.After(nilTimestamp), qt.IsFalse)
	c.Assert(nilTimestamp.After(&timestamp1), qt.IsFalse)

	// Test Before
	c.Assert(timestamp1.Before(&timestamp2), qt.IsTrue)
	c.Assert(timestamp2.Before(&timestamp1), qt.IsFalse)
	c.Assert(timestamp1.Before(nilTimestamp), qt.IsFalse)
	c.Assert(nilTimestamp.Before(&timestamp1), qt.IsFalse)

	// Test Equal
	c.Assert(timestamp1.Equal(&timestamp3), qt.IsTrue)
	c.Assert(timestamp1.Equal(&timestamp2), qt.IsFalse)
	c.Assert(timestamp1.Equal(nilTimestamp), qt.IsFalse)
	c.Assert(nilTimestamp.Equal(&timestamp1), qt.IsFalse)

	// Test nil == nil case
	var nilTimestamp2 *models.Timestamp
	c.Assert(nilTimestamp.Equal(nilTimestamp2), qt.IsTrue)
}

func TestToPTimestamp(t *testing.T) {
	c := qt.New(t)

	// Test with non-empty timestamp
	timestamp := models.Timestamp("2023-01-01T12:00:00Z")
	pTimestamp := models.ToPTimestamp(timestamp)
	c.Assert(pTimestamp, qt.IsNotNil)
	c.Assert(string(*pTimestamp), qt.Equals, string(timestamp))

	// Test with empty timestamp
	emptyTimestamp := models.Timestamp("")
	pTimestamp = models.ToPTimestamp(emptyTimestamp)
	c.Assert(pTimestamp, qt.IsNil)
}

func TestNewTimestamp(t *testing.T) {
	c := qt.New(t)

	testTime := time.Date(2023, 1, 1, 12, 30, 45, 0, time.UTC)
	timestamp := models.NewTimestamp(testTime)

	c.Assert(string(timestamp), qt.Equals, "2023-01-01T12:30:45Z")
}

func TestNewPTimestamp(t *testing.T) {
	c := qt.New(t)

	testTime := time.Date(2023, 1, 1, 12, 30, 45, 0, time.UTC)
	pTimestamp := models.NewPTimestamp(testTime)

	c.Assert(pTimestamp, qt.IsNotNil)
	c.Assert(string(*pTimestamp), qt.Equals, "2023-01-01T12:30:45Z")
}

func TestNow(t *testing.T) {
	c := qt.New(t)

	before := time.Now()
	timestamp := models.Now()
	after := time.Now()

	timestampTime := timestamp.ToTime()
	c.Assert(timestampTime.After(before.Add(-time.Second)), qt.IsTrue)
	c.Assert(timestampTime.Before(after.Add(time.Second)), qt.IsTrue)
}

func TestPNow(t *testing.T) {
	c := qt.New(t)

	before := time.Now()
	pTimestamp := models.PNow()
	after := time.Now()

	c.Assert(pTimestamp, qt.IsNotNil)
	timestampTime := pTimestamp.ToTime()
	c.Assert(timestampTime.After(before.Add(-time.Second)), qt.IsTrue)
	c.Assert(timestampTime.Before(after.Add(time.Second)), qt.IsTrue)
}
