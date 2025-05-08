package apiserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/checkers"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func TestLocationsUpdate_WithNestedData(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{
		RegistrySet: &registry.Set{
			LocationRegistry: newLocationRegistry(),
		},
	}
	locations := must.Must(params.RegistrySet.LocationRegistry.List())
	location := locations[0]

	// This test simulates the issue where the frontend sends a nested data structure
	// The attributes field contains a nested data structure instead of just the location attributes
	nestedUpdateObj := map[string]interface{}{
		"data": map[string]interface{}{
			"id":   location.ID,
			"type": "locations",
			"attributes": map[string]interface{}{
				"data": map[string]interface{}{
					"id":   location.ID,
					"type": "locations",
					"attributes": map[string]interface{}{
						"id":      location.ID,
						"name":    "Nested Update Name",
						"address": "Nested Update Address",
					},
				},
			},
		},
	}

	nestedUpdateData := must.Must(json.Marshal(nestedUpdateObj))
	nestedUpdateBuf := bytes.NewReader(nestedUpdateData)

	req, err := http.NewRequest("PUT", "/api/v1/locations/"+location.ID, nestedUpdateBuf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	// This should fail with a 422 Unprocessable Entity
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestLocationsUpdate_WithCorrectData(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{
		RegistrySet: &registry.Set{
			LocationRegistry: newLocationRegistry(),
		},
	}
	locations := must.Must(params.RegistrySet.LocationRegistry.List())
	location := locations[0]

	// This is the correct structure
	updateObj := &jsonapi.LocationRequest{
		Data: &jsonapi.LocationData{
			ID:   location.ID,
			Type: "locations",
			Attributes: models.WithID(location.ID, &models.Location{
				Name:    "Correct Update Name",
				Address: "Correct Update Address",
			}),
		},
	}
	updateData := must.Must(json.Marshal(updateObj))
	updateBuf := bytes.NewReader(updateData)

	req, err := http.NewRequest("PUT", "/api/v1/locations/"+location.ID, updateBuf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathEquals("$.data.id"), location.ID)
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "locations")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.name"), "Correct Update Name")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.address"), "Correct Update Address")
}
