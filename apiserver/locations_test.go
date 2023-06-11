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

func TestLocationsDelete(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{
		LocationRegistry: newLocationRegistry(),
	}
	locations := must.Must(params.LocationRegistry.List())

	req, err := http.NewRequest("DELETE", "/api/v1/locations/"+locations[0].ID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	expectedCount := must.Must(params.LocationRegistry.Count()) - 1

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	cnt, err := params.LocationRegistry.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(cnt, qt.Equals, expectedCount)
}

func TestLocationsCreate(t *testing.T) {
	c := qt.New(t)

	obj := &jsonapi.LocationRequest{
		Data: &models.Location{
			Name:    "LocationResponse New",
			Address: "Address New",
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("POST", "/api/v1/locations", buf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()
	params := apiserver.Params{
		LocationRegistry: newLocationRegistry(),
	}
	expectedCount := must.Must(params.LocationRegistry.Count()) + 1

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusCreated)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathEquals("$.type"), "locations")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.name"), "LocationResponse New")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.address"), "Address New")
	c.Assert(body, checkers.JSONPathMatches("$.id", qt.Matches), "^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$")

	cnt, err := params.LocationRegistry.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(cnt, qt.Equals, expectedCount)
}

func TestLocationsGet(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{}
	params.LocationRegistry = newLocationRegistry()
	params.AreaRegistry = newAreaRegistry(params.LocationRegistry)
	locations := must.Must(params.LocationRegistry.List())
	location := locations[0]

	req, err := http.NewRequest("GET", "/api/v1/locations/"+location.ID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.type"), "locations")
	c.Assert(body, checkers.JSONPathEquals("$.id"), location.ID)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.name"), location.Name)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.address"), location.Address)

	areas := params.LocationRegistry.GetAreas(location.ID)
	c.Assert(body, checkers.JSONPathMatches("$.attributes.areas", qt.HasLen), len(areas))
	c.Assert(body, checkers.JSONPathEquals("$.attributes.areas[0]"), areas[0])
	c.Assert(body, checkers.JSONPathEquals("$.attributes.areas[1]"), areas[1])
}

func TestLocationsList(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{
		LocationRegistry: newLocationRegistry(),
	}
	expectedLocations := must.Must(params.LocationRegistry.List())

	req, err := http.NewRequest("GET", "/api/v1/locations", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathMatches("$.data", qt.HasLen), len(expectedLocations))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].id"), expectedLocations[0].ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].name"), expectedLocations[0].Name)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].address"), expectedLocations[0].Address)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].id"), expectedLocations[1].ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].name"), expectedLocations[1].Name)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].address"), expectedLocations[1].Address)
}

func TestLocationsUpdate(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{
		LocationRegistry: newLocationRegistry(),
	}
	locations := must.Must(params.LocationRegistry.List())
	location := locations[0]

	updateObj := &jsonapi.LocationRequest{
		Data: &models.Location{
			ID:      location.ID,
			Name:    "Updated Name",
			Address: "Updated Address",
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
	c.Assert(body, checkers.JSONPathEquals("$.id"), location.ID)
	c.Assert(body, checkers.JSONPathEquals("$.type"), "locations")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.name"), "Updated Name")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.address"), "Updated Address")
}

func TestLocationsList_EmptyRegistry(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{
		LocationRegistry: registry.NewMemoryLocationRegistry(), // Empty registry
	}

	req, err := http.NewRequest("GET", "/api/v1/locations", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.data"), []any{})
}

func TestLocationsGet_InvalidID(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{
		LocationRegistry: newLocationRegistry(),
	}

	invalidID := "invalid-id"

	req, err := http.NewRequest("GET", "/api/v1/locations/"+invalidID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestLocationsUpdate_PartialData(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{
		LocationRegistry: newLocationRegistry(),
	}
	locations := must.Must(params.LocationRegistry.List())
	location := locations[0]

	updateObj := &jsonapi.LocationRequest{
		Data: &models.Location{
			ID:   location.ID,
			Name: "Updated Name",
			// Address field is not provided
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
	c.Assert(body, checkers.JSONPathEquals("$.id"), location.ID)
	c.Assert(body, checkers.JSONPathEquals("$.type"), "locations")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.name"), "Updated Name")

	// Assert that the address field is unchanged
	// c.Assert(body, checkers.JSONPathEquals("$.attributes.address"), location.Address)
	// As we are not supporting partial updates, the address field should be empty
	c.Assert(body, checkers.JSONPathEquals("$.attributes.address"), "")
}

func TestLocationsUpdate_ForeignIDInRequestBody(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{
		LocationRegistry: newLocationRegistry(),
	}
	locations := must.Must(params.LocationRegistry.List())
	location := locations[0]
	anotherLocation := locations[1]

	updateObj := &jsonapi.LocationRequest{
		Data: &models.Location{
			ID:      anotherLocation.ID, // Using a different ID in the update request
			Name:    "Updated Name",
			Address: "Updated Address",
		},
	}
	updateData := must.Must(json.Marshal(updateObj))
	updateBuf := bytes.NewReader(updateData)

	req, err := http.NewRequest("PUT", "/api/v1/locations/"+location.ID, updateBuf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestLocationsUpdate_UnknownLocation(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{
		LocationRegistry: newLocationRegistry(),
	}

	unknownID := "unknown-id"

	updateObj := &jsonapi.LocationRequest{
		Data: &models.Location{
			ID:      unknownID,
			Name:    "Updated Name",
			Address: "Updated Address",
		},
	}
	updateData := must.Must(json.Marshal(updateObj))
	updateBuf := bytes.NewReader(updateData)

	req, err := http.NewRequest("PUT", "/api/v1/locations/"+unknownID, updateBuf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestLocationsDelete_MissingLocation(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{
		LocationRegistry: newLocationRegistry(),
	}

	missingID := "missing-id"

	req, err := http.NewRequest("DELETE", "/api/v1/locations/"+missingID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestLocationsCreate_UnexpectedDataStructure(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{
		LocationRegistry: newLocationRegistry(),
	}

	// Construct a request body with an unexpected data structure
	// For example, sending an array instead of an object
	data := []byte(`[{"name": "LocationResponse New", "address": "Address New"}]`)

	req, err := http.NewRequest("POST", "/api/v1/locations", bytes.NewReader(data))
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}
