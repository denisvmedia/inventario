package apiserver_test

import (
	"bytes"
	"context"
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

// mockRestoreWorker is a mock implementation of RestoreWorkerInterface for testing
type mockRestoreWorker struct {
	hasRunningRestores bool
}

func (m *mockRestoreWorker) HasRunningRestores(_ctx context.Context) (bool, error) {
	return m.hasRunningRestores, nil
}

func TestAreasList(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedAreas := must.Must(params.RegistrySet.AreaRegistry.List(context.Background()))

	req, err := http.NewRequest("GET", "/api/v1/areas", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathMatches("$.data", qt.HasLen), len(expectedAreas))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].id"), expectedAreas[0].ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.name"), expectedAreas[0].Name)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.location_id"), expectedAreas[0].LocationID)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].id"), expectedAreas[1].ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].attributes.name"), expectedAreas[1].Name)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].attributes.location_id"), expectedAreas[1].LocationID)
}

func TestAreasGet(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedAreas := must.Must(params.RegistrySet.AreaRegistry.List(context.Background()))
	area := expectedAreas[0]

	req, err := http.NewRequest("GET", "/api/v1/areas/"+area.ID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "areas")
	c.Assert(body, checkers.JSONPathEquals("$.data.id"), area.ID)
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.name"), area.Name)
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.location_id"), area.LocationID)
}

func TestAreaCreate(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedLocations := must.Must(params.RegistrySet.LocationRegistry.List(context.Background()))
	location := expectedLocations[1]

	obj := &jsonapi.AreaRequest{
		Data: &jsonapi.AreaData{
			Type: "areas",
			Attributes: &models.Area{
				Name:       "New Area in location 2",
				LocationID: location.ID,
			},
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("POST", "/api/v1/areas", buf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	body := rr.Body.Bytes()
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("Body: %s", string(body)))
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "areas")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.name"), "New Area in location 2")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.location_id"), location.ID)
	c.Assert(body, checkers.JSONPathMatches("$.data.id", qt.Matches), "^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$")

	var v any
	err = json.Unmarshal(body, &v)
	c.Assert(err, qt.IsNil)
	areaID, err := jsonpath.Read(v, "$.data.id")
	c.Assert(err, qt.IsNil)

	// check that the area was attached to the location

	req, err = http.NewRequest("GET", "/api/v1/locations/"+location.ID, nil)
	c.Assert(err, qt.IsNil)

	rr = httptest.NewRecorder()

	handler = apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body = rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathMatches("$.data.attributes.areas", qt.HasLen), 1)
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.areas[0]"), areaID)
}

func TestAreaDelete(t *testing.T) {
	c := qt.New(t)

	params := newParamsAreaRegistryOnly()
	expectedAreas := must.Must(params.RegistrySet.AreaRegistry.List(context.Background()))
	area := expectedAreas[0]

	req, err := http.NewRequest("DELETE", "/api/v1/areas/"+area.ID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	expectedCount := must.Must(params.RegistrySet.AreaRegistry.Count(context.Background())) - 1

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	cnt, err := params.RegistrySet.AreaRegistry.Count(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(cnt, qt.Equals, expectedCount)
}

func TestAreaDelete_AreaHasCommodities(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedAreas := must.Must(params.RegistrySet.AreaRegistry.List(context.Background()))
	area := expectedAreas[0]

	req, err := http.NewRequest("DELETE", "/api/v1/areas/"+area.ID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	expectedCount := must.Must(params.RegistrySet.AreaRegistry.Count(context.Background()))

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)

	cnt, err := params.RegistrySet.AreaRegistry.Count(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(cnt, qt.Equals, expectedCount)
}

func TestAreaUpdate(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedAreas := must.Must(params.RegistrySet.AreaRegistry.List(context.Background()))
	area := expectedAreas[0]

	obj := &jsonapi.AreaRequest{
		Data: &jsonapi.AreaData{
			ID:   area.ID,
			Type: "areas",
			Attributes: &models.Area{
				TenantAwareEntityID: models.WithTenantAwareEntityID(area.ID, "default-tenant"),
				Name:       "Updated Area",
				LocationID: area.LocationID,
			},
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("PUT", "/api/v1/areas/"+area.ID, buf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.data.id"), area.ID)
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "areas")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.name"), "Updated Area")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.location_id"), area.LocationID)
}

func TestAreaGet_InvalidID(t *testing.T) {
	c := qt.New(t)

	params := newParams()

	invalidID := "invalid-id"

	req, err := http.NewRequest("GET", "/api/v1/areas/"+invalidID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAreaCreate_InvalidData(t *testing.T) {
	c := qt.New(t)

	params := newParams()

	// Send an invalid area request with missing required fields
	invalidObj := &jsonapi.AreaRequest{
		Data: &jsonapi.AreaData{
			Type:       "areas",
			Attributes: &models.Area{},
		},
	}
	invalidData := must.Must(json.Marshal(invalidObj))
	invalidBuf := bytes.NewReader(invalidData)

	req, err := http.NewRequest("POST", "/api/v1/areas", invalidBuf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
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

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAreaUpdate_WrongIDInRequestBody(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedAreas := must.Must(params.RegistrySet.AreaRegistry.List(context.Background()))
	area := expectedAreas[0]

	wrongID := "wrong-id"

	obj := &jsonapi.AreaRequest{
		Data: &jsonapi.AreaData{
			ID:   wrongID,
			Type: "areas",
			Attributes: &models.Area{
				TenantAwareEntityID: models.WithTenantAwareEntityID(wrongID, "default-tenant"), // Using a different ID in the update request
				Name:       "Updated Area",
				LocationID: area.LocationID,
			},
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("PUT", "/api/v1/areas/"+area.ID, buf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}
