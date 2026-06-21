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

// mockRestoreWorker is a mock implementation of RestoreStatusQuerier for testing
type mockRestoreWorker struct {
	hasRunningRestores bool
}

func (m *mockRestoreWorker) HasRunningRestores(_ctx context.Context) (bool, error) {
	return m.hasRunningRestores, nil
}

func TestAreasList(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	ctx := createTestUserContext(testUser.ID, testUser.TenantID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	expectedAreas := must.Must(registrySet.AreaRegistry.List(context.Background()))

	req, err := http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/areas", nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

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

// TestAreasList_FilterByLocationID covers #1473 — clients used to have to
// fetch every area in the group and filter by `location_id` client-side.
// The filterable-flat contract (`?location_id=…`) returns the same shape,
// scoped to a single location, and an unknown ID is an empty list rather
// than a 4xx so the FE doesn't have to special-case errors here.
func TestAreasList_FilterByLocationID(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	ctx := createTestUserContext(testUser.ID, testUser.TenantID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))

	locations := must.Must(registrySet.LocationRegistry.List(context.Background()))
	c.Assert(locations, qt.HasLen, 2)
	loc1 := locations[0]
	loc2 := locations[1]

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)

	doGET := func(query string) *httptest.ResponseRecorder {
		req, err := http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/areas"+query, nil)
		c.Assert(err, qt.IsNil)
		addTestUserAuthHeader(req, testUser.ID)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}

	// Location 1 holds both seeded areas — every returned row carries its id.
	rr := doGET("?location_id=" + loc1.ID)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathMatches("$.data", qt.HasLen), 2)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.location_id"), loc1.ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].attributes.location_id"), loc1.ID)

	// Location 2 has no seeded areas — empty collection, 200.
	rr = doGET("?location_id=" + loc2.ID)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 0)

	// Unknown / cross-tenant id is also empty — see AreaListOptions doc.
	rr = doGET("?location_id=does-not-exist")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 0)
}

func TestAreasGet(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	ctx := createTestUserContext(testUser.ID, testUser.TenantID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	expectedAreas := must.Must(registrySet.AreaRegistry.List(context.Background()))
	area := expectedAreas[0]

	req, err := http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/areas/"+area.ID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

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

	params, testUser, testGroup := newParams()
	ctx := createTestUserContext(testUser.ID, testUser.TenantID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	expectedLocations := must.Must(registrySet.LocationRegistry.List(context.Background()))
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

	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/areas", buf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

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

	req, err = http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/locations/"+location.ID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

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

	params, testUser, testGroup := newParamsAreaRegistryOnly()
	ctx := createTestUserContext(testUser.ID, testUser.TenantID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	expectedAreas := must.Must(registrySet.AreaRegistry.List(context.Background()))
	area := expectedAreas[0]

	req, err := http.NewRequest("DELETE", "/api/v1/g/"+testGroup.Slug+"/areas/"+area.ID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	expectedCount := must.Must(registrySet.AreaRegistry.Count(context.Background())) - 1

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	cnt, err := registrySet.AreaRegistry.Count(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(cnt, qt.Equals, expectedCount)
}

// TestAreaDelete_NonEmptyRejected covers #2119: deleting a NON-empty area
// through the API must be rejected (the handler no longer cascades). The
// seeded area holds a commodity which in turn owns several files; the DELETE
// returns 422 and the area, its commodities and the linked file all survive
// (GET file → 200), proving the API path routes through the non-recursive
// EntityService.DeleteArea which surfaces ErrCannotDelete for a non-empty area.
func TestAreaDelete_NonEmptyRejected(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	ctx := createTestUserContext(testUser.ID, testUser.TenantID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))

	// Seeded layout: areas[0] holds commodities[0], which owns the seeded
	// files. Pick a file linked to that commodity so we can prove nothing
	// was removed.
	expectedAreas := must.Must(registrySet.AreaRegistry.List(context.Background()))
	area := expectedAreas[0]
	commodityIDs := must.Must(registrySet.AreaRegistry.GetCommodities(context.Background(), area.ID))
	c.Assert(commodityIDs, qt.Not(qt.HasLen), 0)
	files := must.Must(registrySet.FileRegistry.List(context.Background()))
	var fileID string
	for _, f := range files {
		if f.LinkedEntityType == "commodity" && f.LinkedEntityID == commodityIDs[0] {
			fileID = f.ID
			break
		}
	}
	c.Assert(fileID, qt.Not(qt.Equals), "")

	req, err := http.NewRequest("DELETE", "/api/v1/g/"+testGroup.Slug+"/areas/"+area.ID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	// Non-empty area: rejected with 422, nothing removed.
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)

	// The area still exists.
	c.Assert(must.Must(registrySet.AreaRegistry.Get(context.Background(), area.ID)), qt.IsNotNil)
	// Its commodities survive.
	c.Assert(must.Must(registrySet.AreaRegistry.GetCommodities(context.Background(), area.ID)), qt.Not(qt.HasLen), 0)

	// The file linked to the area's commodity survives too (GET → 200).
	req, err = http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/files/"+fileID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
}

func TestAreaUpdate(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	ctx := createTestUserContext(testUser.ID, testUser.TenantID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	expectedAreas := must.Must(registrySet.AreaRegistry.List(context.Background()))
	area := expectedAreas[0]

	obj := &jsonapi.AreaRequest{
		Data: &jsonapi.AreaData{
			ID:   area.ID,
			Type: "areas",
			Attributes: models.WithID(area.ID, &models.Area{
				Name:       "Updated Area",
				LocationID: area.LocationID,
			}),
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("PUT", "/api/v1/g/"+testGroup.Slug+"/areas/"+area.ID, buf)
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/json")
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		c.Logf("Response body: %s", rr.Body.String())
	}
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.data.id"), area.ID)
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "areas")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.name"), "Updated Area")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.location_id"), area.LocationID)
}

func TestAreaGet_InvalidID(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()

	invalidID := "invalid-id"

	req, err := http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/areas/"+invalidID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAreaCreate_InvalidData(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()

	// Send an invalid area request with missing required fields
	invalidObj := &jsonapi.AreaRequest{
		Data: &jsonapi.AreaData{
			Type:       "areas",
			Attributes: &models.Area{},
		},
	}
	invalidData := must.Must(json.Marshal(invalidObj))
	invalidBuf := bytes.NewReader(invalidData)

	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/areas", invalidBuf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestAreaDelete_MissingArea(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()

	missingID := "missing-id"

	req, err := http.NewRequest("DELETE", "/api/v1/g/"+testGroup.Slug+"/areas/"+missingID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestAreaUpdate_WrongIDInRequestBody(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	ctx := createTestUserContext(testUser.ID, testUser.TenantID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
	expectedAreas := must.Must(registrySet.AreaRegistry.List(context.Background()))
	area := expectedAreas[0]

	wrongID := "wrong-id"

	obj := &jsonapi.AreaRequest{
		Data: &jsonapi.AreaData{
			ID:   wrongID,
			Type: "areas",
			Attributes: &models.Area{
				TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{EntityID: models.EntityID{ID: wrongID}, TenantID: "default-tenant"}, // Using a different ID in the update request
				Name:                     "Updated Area",
				LocationID:               area.LocationID,
			},
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("PUT", "/api/v1/g/"+testGroup.Slug+"/areas/"+area.ID, buf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}
