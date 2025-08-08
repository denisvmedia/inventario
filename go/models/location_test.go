package models_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
)

func TestLocation_Validate(t *testing.T) {
	c := qt.New(t)

	location := &models.Location{}
	err := location.Validate()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorIs, models.ErrMustUseValidateWithContext)
}

func TestLocation_ValidateWithContext_HappyPath(t *testing.T) {
	t.Run("valid location", func(t *testing.T) {
		c := qt.New(t)

		location := models.Location{
			Name:    "Test Location",
			Address: "123 Test Street",
		}

		ctx := context.Background()
		err := location.ValidateWithContext(ctx)
		c.Assert(err, qt.IsNil)
	})
}

func TestLocation_ValidateWithContext_UnhappyPaths(t *testing.T) {
	testCases := []struct {
		name          string
		location      models.Location
		errorContains string
	}{
		{
			name:          "missing name",
			location:      models.Location{Address: "123 Test Street"},
			errorContains: "name: cannot be blank",
		},
		{
			name:          "missing address",
			location:      models.Location{Name: "Test Location"},
			errorContains: "address: cannot be blank",
		},
		{
			name:          "empty location",
			location:      models.Location{},
			errorContains: "name: cannot be blank",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			ctx := context.Background()
			err := tc.location.ValidateWithContext(ctx)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.errorContains)
		})
	}
}

func TestLocation_JSONMarshaling(t *testing.T) {
	c := qt.New(t)

	// Create a location with all fields populated
	location := models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{
				ID: "location-123",
			},
			TenantID: "test-tenant",
		},
		Name:    "Test Location",
		Address: "123 Test Street",
	}

	// Marshal the location to JSON
	jsonData, err := json.Marshal(location)
	c.Assert(err, qt.IsNil)

	// Unmarshal the JSON back to a location
	var unmarshaledLocation models.Location
	err = json.Unmarshal(jsonData, &unmarshaledLocation)
	c.Assert(err, qt.IsNil)

	// Verify that the unmarshaled location matches the original
	c.Assert(unmarshaledLocation.ID, qt.Equals, location.ID)
	c.Assert(unmarshaledLocation.Name, qt.Equals, location.Name)
	c.Assert(unmarshaledLocation.Address, qt.Equals, location.Address)
}

func TestLocation_IDable(t *testing.T) {
	c := qt.New(t)

	// Create a location
	location := models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{
				ID: "location-123",
			},
			TenantID: "test-tenant",
		},
		Name:    "Test Location",
		Address: "123 Test Street",
	}

	// Test GetID
	c.Assert(location.GetID(), qt.Equals, "location-123")

	// Test SetID
	location.SetID("new-location-id")
	c.Assert(location.GetID(), qt.Equals, "new-location-id")
}
