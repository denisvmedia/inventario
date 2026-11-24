package models_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func TestFormatAssetID(t *testing.T) {
	testCases := []struct {
		n    int
		want string
	}{
		{1, "A-0001"},
		{42, "A-0042"},
		{9999, "A-9999"},
		{12345, "A-12345"},
		{0, ""},
		{-3, ""},
	}
	for _, tc := range testCases {
		t.Run(tc.want, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(models.FormatAssetID(tc.n), qt.Equals, tc.want)
		})
	}
}

func TestParseAssetID_Accepted(t *testing.T) {
	testCases := []struct {
		in   string
		want int
	}{
		{"A-0042", 42},
		{"a-0042", 42},
		{"A-42", 42},
		{"A42", 42},
		{"A 42", 42},
		{"#42", 42},
		{"42", 42},
		{"  A-0042  ", 42},
		{"A-12345", 12345},
	}
	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			c := qt.New(t)
			got, ok := models.ParseAssetID(tc.in)
			c.Assert(ok, qt.IsTrue)
			c.Assert(got, qt.Equals, tc.want)
		})
	}
}

func TestParseAssetID_Rejected(t *testing.T) {
	for _, in := range []string{"", "A", "A-", "#", "0", "A-0000", "Apple", "A-42b", "4 2", "-42", "B-0042", "1234567890"} {
		t.Run(in, func(t *testing.T) {
			c := qt.New(t)
			_, ok := models.ParseAssetID(in)
			c.Assert(ok, qt.IsFalse)
		})
	}
}

func TestParseAssetID_RoundTripsFormat(t *testing.T) {
	c := qt.New(t)
	for _, n := range []int{1, 7, 42, 999, 9999, 10000, 123456} {
		got, ok := models.ParseAssetID(models.FormatAssetID(n))
		c.Assert(ok, qt.IsTrue)
		c.Assert(got, qt.Equals, n)
	}
}
