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
	"github.com/denisvmedia/inventario/registry"
)

// Legacy `/locations/{id}/{images,files}*` route tests (formerly in this
// file: TestLocationImages_*, TestLocationFiles_*) were removed under
// #1421 alongside the routes themselves. The unified `/files` surface
// covers the same reads via `?linked_entity_type=location&linked_entity_id=…`
// and is exercised by the file-registry + apiserver/files tests.

func TestLocationsDelete(t *testing.T) {
	c := qt.New(t)

	// Use the full params which includes EntityService and all components
	params, testUser, testGroup := newParams()
	locations := must.Must(getRegistrySetFromParams(params, testUser).LocationRegistry.List(c.Context()))

	// Delete locations[1] instead of locations[0] because locations[0] has areas associated with it
	req, err := http.NewRequest("DELETE", "/api/v1/g/"+testGroup.Slug+"/locations/"+locations[1].ID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	expectedCount := must.Must(getRegistrySetFromParams(params, testUser).LocationRegistry.Count(c.Context())) - 1

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	cnt, err := getRegistrySetFromParams(params, testUser).LocationRegistry.Count(c.Context())
	c.Assert(err, qt.IsNil)
	c.Assert(cnt, qt.Equals, expectedCount)
}

// TestLocationsDelete_NonEmptyRejected covers #2119: deleting a NON-empty
// location through the API must be rejected (the handler no longer cascades).
// The seeded locations[0] contains an area whose commodity owns the seeded
// files; the DELETE returns 422 and the whole subtree survives (GET file →
// 200), proving the API path routes through the non-recursive
// EntityService.DeleteLocation which surfaces ErrCannotDelete for a non-empty
// location.
func TestLocationsDelete_NonEmptyRejected(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)

	// Seeded layout: locations[0] holds the areas; areas[0] holds
	// commodities[0], which owns the seeded files. Resolve a file linked
	// to a commodity that lives under locations[0] so we can prove nothing
	// was removed.
	locations := must.Must(registrySet.LocationRegistry.List(c.Context()))
	location := locations[0]
	areaIDs := must.Must(registrySet.LocationRegistry.GetAreas(c.Context(), location.ID))
	c.Assert(areaIDs, qt.Not(qt.HasLen), 0)
	commodityIDs := must.Must(registrySet.AreaRegistry.GetCommodities(c.Context(), areaIDs[0]))
	c.Assert(commodityIDs, qt.Not(qt.HasLen), 0)
	files := must.Must(registrySet.FileRegistry.List(c.Context()))
	var fileID string
	for _, f := range files {
		if f.LinkedEntityType == "commodity" && f.LinkedEntityID == commodityIDs[0] {
			fileID = f.ID
			break
		}
	}
	c.Assert(fileID, qt.Not(qt.Equals), "")

	req, err := http.NewRequest("DELETE", "/api/v1/g/"+testGroup.Slug+"/locations/"+location.ID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	// Non-empty location: rejected with 422, nothing removed.
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)

	// The location and its areas survive.
	c.Assert(must.Must(registrySet.LocationRegistry.Get(c.Context(), location.ID)), qt.IsNotNil)
	c.Assert(must.Must(registrySet.LocationRegistry.GetAreas(c.Context(), location.ID)), qt.Not(qt.HasLen), 0)

	// The file linked to the location's commodity survives too (GET → 200).
	req, err = http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/files/"+fileID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
}

// TestLocationsDelete_CascadeStrategy covers #2137: DELETE ?strategy=cascade on
// a non-empty location returns 204 and removes the location, its areas and the
// commodities under them.
func TestLocationsDelete_CascadeStrategy(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)

	locations := must.Must(registrySet.LocationRegistry.List(c.Context()))
	location := locations[0]
	areaIDs := must.Must(registrySet.LocationRegistry.GetAreas(c.Context(), location.ID))
	c.Assert(areaIDs, qt.Not(qt.HasLen), 0)
	commodityIDs := must.Must(registrySet.AreaRegistry.GetCommodities(c.Context(), areaIDs[0]))
	c.Assert(commodityIDs, qt.Not(qt.HasLen), 0)

	req, err := http.NewRequest("DELETE", "/api/v1/g/"+testGroup.Slug+"/locations/"+location.ID+"?strategy=cascade", nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	// The location, its areas and commodities are all gone.
	_, err = registrySet.LocationRegistry.Get(c.Context(), location.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	for _, id := range areaIDs {
		_, err = registrySet.AreaRegistry.Get(c.Context(), id)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	}
	for _, id := range commodityIDs {
		_, err = registrySet.CommodityRegistry.Get(c.Context(), id)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	}
}

// TestLocationsDelete_UnlinkStrategy covers #2137: DELETE ?strategy=unlink on a
// non-empty location returns 204, removes the location and its areas, and keeps
// the commodities — left area-less (AreaID == nil); a GET on a surviving
// commodity still returns 200.
func TestLocationsDelete_UnlinkStrategy(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)

	locations := must.Must(registrySet.LocationRegistry.List(c.Context()))
	location := locations[0]
	areaIDs := must.Must(registrySet.LocationRegistry.GetAreas(c.Context(), location.ID))
	c.Assert(areaIDs, qt.Not(qt.HasLen), 0)
	commodityIDs := must.Must(registrySet.AreaRegistry.GetCommodities(c.Context(), areaIDs[0]))
	c.Assert(commodityIDs, qt.Not(qt.HasLen), 0)

	req, err := http.NewRequest("DELETE", "/api/v1/g/"+testGroup.Slug+"/locations/"+location.ID+"?strategy=unlink", nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	// The location and its areas are gone.
	_, err = registrySet.LocationRegistry.Get(c.Context(), location.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	for _, id := range areaIDs {
		_, err = registrySet.AreaRegistry.Get(c.Context(), id)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	}

	// The commodities survive, now area-less; a GET still returns 200.
	for _, id := range commodityIDs {
		commodity := must.Must(registrySet.CommodityRegistry.Get(c.Context(), id))
		c.Assert(commodity.AreaID, qt.IsNil)

		req, err = http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/commodities/"+id, nil)
		c.Assert(err, qt.IsNil)
		addTestUserAuthHeader(req, testUser.ID)
		rr = httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		c.Assert(rr.Code, qt.Equals, http.StatusOK)
	}
}

// TestLocationsDelete_BogusStrategy covers #2137: an unknown `?strategy=` value
// is rejected with 422 before anything is removed.
func TestLocationsDelete_BogusStrategy(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)

	locations := must.Must(registrySet.LocationRegistry.List(c.Context()))
	location := locations[0]

	req, err := http.NewRequest("DELETE", "/api/v1/g/"+testGroup.Slug+"/locations/"+location.ID+"?strategy=bogus", nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)

	// The location survives — nothing was removed.
	c.Assert(must.Must(registrySet.LocationRegistry.Get(c.Context(), location.ID)), qt.IsNotNil)
}

func TestLocationsCreate(t *testing.T) {
	c := qt.New(t)

	obj := &jsonapi.LocationRequest{
		Data: &jsonapi.LocationData{
			Type: "locations",
			Attributes: &models.Location{
				Name:    "LocationResponse New",
				Address: "Address New",
			},
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	// Use the full params which includes EntityService and all components
	params, testUser, testGroup := newParams()

	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/locations", buf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusCreated)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "locations")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.name"), "LocationResponse New")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.address"), "Address New")
	c.Assert(body, checkers.JSONPathMatches("$.data.id", qt.Matches), "^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$")

	// Extract the created location ID and verify it can be retrieved
	var v any
	err = json.Unmarshal(body, &v)
	c.Assert(err, qt.IsNil)
	locationID, err := jsonpath.Read(v, "$.data.id")
	c.Assert(err, qt.IsNil)

	// Verify the location can be retrieved by ID to ensure it was properly persisted
	retrievedLocation, err := getRegistrySetFromParams(params, testUser).LocationRegistry.Get(c.Context(), locationID.(string))
	c.Assert(err, qt.IsNil)
	c.Assert(retrievedLocation.Name, qt.Equals, "LocationResponse New")
	c.Assert(retrievedLocation.Address, qt.Equals, "Address New")
}

func TestLocationsGet(t *testing.T) {
	c := qt.New(t)

	// Use the full params which includes EntityService and all components
	params, testUser, testGroup := newParams()
	locations := must.Must(getRegistrySetFromParams(params, testUser).LocationRegistry.List(c.Context()))
	location := locations[0]

	req, err := http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/locations/"+location.ID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "locations")
	c.Assert(body, checkers.JSONPathEquals("$.data.id"), location.ID)
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.name"), location.Name)
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.address"), location.Address)

	areas, err := getRegistrySetFromParams(params, testUser).LocationRegistry.GetAreas(c.Context(), location.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(body, checkers.JSONPathMatches("$.data.attributes.areas", qt.HasLen), len(areas))
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.areas[0]"), areas[0])
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.areas[1]"), areas[1])
}

func TestLocationsList(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	// Override with specific location registry for this test
	// Note: Cannot easily replace location registry in factory pattern
	expectedLocations := must.Must(getRegistrySetFromParams(params, testUser).LocationRegistry.List(c.Context()))

	req, err := http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/locations", nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathMatches("$.data", qt.HasLen), len(expectedLocations))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].id"), expectedLocations[0].ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.name"), expectedLocations[0].Name)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].attributes.address"), expectedLocations[0].Address)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].id"), expectedLocations[1].ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].attributes.name"), expectedLocations[1].Name)
	c.Assert(body, checkers.JSONPathEquals("$.data[1].attributes.address"), expectedLocations[1].Address)
}

func TestLocationsUpdate(t *testing.T) {
	c := qt.New(t)

	// Use the full params which includes EntityService and all components
	params, testUser, testGroup := newParams()
	locations := must.Must(getRegistrySetFromParams(params, testUser).LocationRegistry.List(c.Context()))
	location := locations[0]

	updateObj := &jsonapi.LocationRequest{
		Data: &jsonapi.LocationData{
			ID:   location.ID,
			Type: "locations",
			Attributes: models.WithID(location.ID, &models.Location{
				Name:    "Updated Name",
				Address: "Updated Address",
			}),
		},
	}
	updateData := must.Must(json.Marshal(updateObj))
	updateBuf := bytes.NewReader(updateData)

	req, err := http.NewRequest("PUT", "/api/v1/g/"+testGroup.Slug+"/locations/"+location.ID, updateBuf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathEquals("$.data.id"), location.ID)
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "locations")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.name"), "Updated Name")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.address"), "Updated Address")
}

func TestLocationsList_EmptyRegistry(t *testing.T) {
	c := qt.New(t)

	// Create a second user who has no location data in the factory set.
	params, _, _ := newParamsAreaRegistryOnly()
	defaultTenant := must.Must(params.FactorySet.TenantRegistry.GetDefault(context.Background()))
	testUser := must.Must(params.FactorySet.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: defaultTenant.ID},
		Email:               "empty@example.com",
		Name:                "Empty User",
		IsActive:            true,
	}))

	// Create a separate group for the second user (so they have no data)
	emptyGroup := createTestGroupForUser(params.FactorySet, defaultTenant.ID, testUser.ID)

	req, err := http.NewRequest("GET", "/api/v1/g/"+emptyGroup.Slug+"/locations", nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.data"), make([]any, 0))
}

func TestLocationsGet_InvalidID(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParamsAreaRegistryOnly()

	invalidID := "invalid-id"

	req, err := http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/locations/"+invalidID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestLocationsUpdate_AddressOptional(t *testing.T) {
	c := qt.New(t)

	// Address is optional: a location may carry only a name. Updating with
	// the address omitted must succeed and clear the stored address rather
	// than rejecting the request with a 422 (the old, wrong behaviour).
	params, testUser, testGroup := newParams()
	locations := must.Must(getRegistrySetFromParams(params, testUser).LocationRegistry.List(c.Context()))
	location := locations[0]

	updateObj := &jsonapi.LocationRequest{
		Data: &jsonapi.LocationData{
			ID:   location.ID,
			Type: "locations",
			Attributes: models.WithID(location.ID, &models.Location{
				Name: "Updated Name",
				// Address field is not provided — it is optional.
			}),
		},
	}
	updateData := must.Must(json.Marshal(updateObj))
	updateBuf := bytes.NewReader(updateData)

	req, err := http.NewRequest("PUT", "/api/v1/g/"+testGroup.Slug+"/locations/"+location.ID, updateBuf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	body := rr.Body.Bytes()
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("Body: %s", body))
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.name"), "Updated Name")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.address"), "")
}

func TestLocationsUpdate_ForeignIDInRequestBody(t *testing.T) {
	c := qt.New(t)

	// Use the full params which includes EntityService and all components
	params, testUser, testGroup := newParams()
	locations := must.Must(getRegistrySetFromParams(params, testUser).LocationRegistry.List(c.Context()))
	location := locations[0]
	anotherLocation := locations[1]

	updateObj := &jsonapi.LocationRequest{
		Data: &jsonapi.LocationData{
			ID:   anotherLocation.ID,
			Type: "locations",
			Attributes: models.WithID(anotherLocation.ID, &models.Location{
				Name:    "Updated Name",
				Address: "Updated Address",
			}),
		},
	}
	updateData := must.Must(json.Marshal(updateObj))
	updateBuf := bytes.NewReader(updateData)

	req, err := http.NewRequest("PUT", "/api/v1/g/"+testGroup.Slug+"/locations/"+location.ID, updateBuf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestLocationsUpdate_UnknownLocation(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParamsAreaRegistryOnly()

	unknownID := "unknown-id"

	updateObj := &jsonapi.LocationRequest{
		Data: &jsonapi.LocationData{
			Type: "locations",
			Attributes: models.WithID(unknownID, &models.Location{
				Name:    "Updated Name",
				Address: "Updated Address",
			}),
		},
	}
	updateData := must.Must(json.Marshal(updateObj))
	updateBuf := bytes.NewReader(updateData)

	req, err := http.NewRequest("PUT", "/api/v1/g/"+testGroup.Slug+"/locations/"+unknownID, updateBuf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestLocationsDelete_MissingLocation(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParamsAreaRegistryOnly()

	missingID := "missing-id"

	req, err := http.NewRequest("DELETE", "/api/v1/g/"+testGroup.Slug+"/locations/"+missingID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestLocationsCreate_UnexpectedDataStructure(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParamsAreaRegistryOnly()

	// Construct a request body with an unexpected data structure
	// For example, sending an array instead of an object
	data := []byte(`[{"name": "LocationResponse New", "address": "Address New"}]`)

	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/locations", bytes.NewReader(data))
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestLocationsUpdate_WithNestedData(t *testing.T) {
	c := qt.New(t)

	// Use the full params which includes EntityService and all components
	params, testUser, testGroup := newParams()
	locations := must.Must(getRegistrySetFromParams(params, testUser).LocationRegistry.List(c.Context()))
	location := locations[0]

	// This test simulates the issue where the frontend sends a nested data structure
	// The attributes field contains a nested data structure instead of just the location attributes
	nestedUpdateObj := map[string]any{
		"data": map[string]any{
			"id":   location.ID,
			"type": "locations",
			"attributes": map[string]any{
				"data": map[string]any{
					"id":   location.ID,
					"type": "locations",
					"attributes": map[string]any{
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

	req, err := http.NewRequest("PUT", "/api/v1/g/"+testGroup.Slug+"/locations/"+location.ID, nestedUpdateBuf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	// This should fail with a 422 Unprocessable Entity
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestLocationsUpdate_WithCorrectData(t *testing.T) {
	c := qt.New(t)

	// Use the full params which includes EntityService and all components
	params, testUser, testGroup := newParams()
	locations := must.Must(getRegistrySetFromParams(params, testUser).LocationRegistry.List(c.Context()))
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

	req, err := http.NewRequest("PUT", "/api/v1/g/"+testGroup.Slug+"/locations/"+location.ID, updateBuf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathEquals("$.data.id"), location.ID)
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "locations")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.name"), "Correct Update Name")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.address"), "Correct Update Address")
}
