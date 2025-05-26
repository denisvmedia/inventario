package models_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestArea_Validate(t *testing.T) {
	c := qt.New(t)

	area := &models.Area{}
	err := area.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorIs, models.ErrMustUseValidateWithContext)
}

func TestArea_ValidateWithContext_HappyPath(t *testing.T) {
	t.Run("valid area", func(t *testing.T) {
		c := qt.New(t)

		area := models.Area{
			Name:       "Test Area",
			LocationID: "location-123",
		}

		ctx := context.Background()
		err := area.ValidateWithContext(ctx)
		c.Assert(err, qt.IsNil)
	})
}

func TestArea_ValidateWithContext_UnhappyPaths(t *testing.T) {
	testCases := []struct {
		name          string
		area          models.Area
		errorContains string
	}{
		{
			name:          "missing name",
			area:          models.Area{LocationID: "location-123"},
			errorContains: "name: cannot be blank",
		},
		{
			name:          "missing location_id",
			area:          models.Area{Name: "Test Area"},
			errorContains: "location_id: cannot be blank",
		},
		{
			name:          "empty area",
			area:          models.Area{},
			errorContains: "location_id: cannot be blank",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			ctx := context.Background()
			err := tc.area.ValidateWithContext(ctx)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestArea_JSONMarshaling(t *testing.T) {
	c := qt.New(t)

	// Create an area with all fields populated
	area := models.Area{
		Name:       "Test Area",
		LocationID: "location-123",
	}
	area.ID = "area-123"

	// Marshal the area to JSON
	jsonData, err := json.Marshal(area)
	c.Assert(err, qt.IsNil)

	// Unmarshal the JSON back to an area
	var unmarshaledArea models.Area
	err = json.Unmarshal(jsonData, &unmarshaledArea)
	c.Assert(err, qt.IsNil)

	// Verify that the unmarshaled area matches the original
	c.Assert(unmarshaledArea.ID, qt.Equals, area.ID)
	c.Assert(unmarshaledArea.Name, qt.Equals, area.Name)
	c.Assert(unmarshaledArea.LocationID, qt.Equals, area.LocationID)
}

func TestArea_IDable(t *testing.T) {
	c := qt.New(t)

	// Create an area
	area := models.Area{
		Name:       "Test Area",
		LocationID: "location-123",
	}
	area.ID = "area-123"

	// Test GetID
	c.Assert(area.GetID(), qt.Equals, "area-123")

	// Test SetID
	area.SetID("new-area-id")
	c.Assert(area.GetID(), qt.Equals, "new-area-id")
}
