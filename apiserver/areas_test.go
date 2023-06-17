package apiserver_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/yalp/jsonpath"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/checkers"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
)

func TestAreasList(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedAreas := must.Must(params.AreaRegistry.List())

	req, err := http.NewRequest("GET", "/api/v1/areas", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathMatches("$.data", qt.HasLen), len(expectedAreas))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].id"), expectedAreas[0].ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].name"), expectedAreas[0].Name)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].location_id"), expectedAreas[0].LocationID)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].id"), expectedAreas[1].ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].name"), expectedAreas[1].Name)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].location_id"), expectedAreas[1].LocationID)
}

func TestAreasGet(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedAreas := must.Must(params.AreaRegistry.List())
	area := expectedAreas[0]

	req, err := http.NewRequest("GET", "/api/v1/areas/"+area.ID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.type"), "areas")
	c.Assert(body, checkers.JSONPathEquals("$.id"), area.ID)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.name"), area.Name)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.location_id"), area.LocationID)
}

func TestAreaCreate(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedLocations := must.Must(params.LocationRegistry.List())
	location := expectedLocations[1]

	obj := &jsonapi.AreaRequest{
		Data: &models.Area{
			Name:       "New Area in location 2",
			LocationID: location.ID,
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("POST", "/api/v1/areas", buf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusCreated)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathEquals("$.type"), "areas")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.name"), "New Area in location 2")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.location_id"), location.ID)
	c.Assert(body, checkers.JSONPathMatches("$.id", qt.Matches), "^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$")

	var v any
	err = json.Unmarshal(body, &v)
	c.Assert(err, qt.IsNil)
	areaID, err := jsonpath.Read(v, "$.id")
	c.Assert(err, qt.IsNil)

	// check that the area was attached to the location

	req, err = http.NewRequest("GET", "/api/v1/locations/"+location.ID, nil)
	c.Assert(err, qt.IsNil)

	rr = httptest.NewRecorder()

	handler = apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body = rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathMatches("$.attributes.areas", qt.HasLen), 1)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.areas[0]"), areaID)
}

func TestAreaDelete(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedAreas := must.Must(params.AreaRegistry.List())
	area := expectedAreas[0]

	req, err := http.NewRequest("DELETE", "/api/v1/areas/"+area.ID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	expectedCount := must.Must(params.AreaRegistry.Count()) - 1

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	cnt, err := params.AreaRegistry.Count()
	c.Assert(err, qt.IsNil)
	c.Assert(cnt, qt.Equals, expectedCount)
}

func TestAreaUpdate(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedAreas := must.Must(params.AreaRegistry.List())
	area := expectedAreas[0]

	obj := &jsonapi.AreaRequest{
		Data: &models.Area{
			ID:         area.ID,
			Name:       "Updated Area",
			LocationID: area.LocationID,
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("PUT", "/api/v1/areas/"+area.ID, buf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.id"), area.ID)
	c.Assert(body, checkers.JSONPathEquals("$.type"), "areas")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.name"), "Updated Area")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.location_id"), area.LocationID)
}

func TestAreaGet_InvalidID(t *testing.T) {
	c := qt.New(t)

	params := newParams()

	invalidID := "invalid-id"

	req, err := http.NewRequest("GET", "/api/v1/areas/"+invalidID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAreaCreate_InvalidData(t *testing.T) {
	c := qt.New(t)

	params := newParams()

	// Send an invalid area request with missing required fields
	invalidObj := &jsonapi.AreaRequest{
		Data: &models.Area{},
	}
	invalidData := must.Must(json.Marshal(invalidObj))
	invalidBuf := bytes.NewReader(invalidData)

	req, err := http.NewRequest("POST", "/api/v1/areas", invalidBuf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestAreaDelete_MissingArea(t *testing.T) {
	c := qt.New(t)

	params := newParams()

	missingID := "missing-id"

	req, err := http.NewRequest("DELETE", "/api/v1/areas/"+missingID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAreaUpdate_WrongIDInRequestBody(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedAreas := must.Must(params.AreaRegistry.List())
	area := expectedAreas[0]

	wrongID := "wrong-id"

	obj := &jsonapi.AreaRequest{
		Data: &models.Area{
			ID:         wrongID, // Using a different ID in the update request
			Name:       "Updated Area",
			LocationID: area.LocationID,
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("PUT", "/api/v1/areas/"+area.ID, buf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}
