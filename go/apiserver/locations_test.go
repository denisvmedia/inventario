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
	"github.com/denisvmedia/inventario/registry/memory"
)

// newParamsWithLocationRegistry creates test params with a specific location registry
func newParamsWithLocationRegistry(locationRegistry registry.LocationRegistry) (apiserver.Params, *models.User) {
	params, testUser := newParams()
	params.RegistrySet.LocationRegistry = locationRegistry
	return params, testUser
}

func TestLocationsDelete(t *testing.T) {
	c := qt.New(t)

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(newLocationRegistry(testUser.ID))
	params.RegistrySet.UserRegistry = userRegistry
	locations := must.Must(params.RegistrySet.LocationRegistry.List(c.Context()))

	req, err := http.NewRequest("DELETE", "/api/v1/locations/"+locations[0].ID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	expectedCount := must.Must(params.RegistrySet.LocationRegistry.Count(c.Context())) - 1

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	cnt, err := params.RegistrySet.LocationRegistry.Count(c.Context())
	c.Assert(err, qt.IsNil)
	c.Assert(cnt, qt.Equals, expectedCount)
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

	req, err := http.NewRequest("POST", "/api/v1/locations", buf)
	c.Assert(err, qt.IsNil)

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(newLocationRegistry(testUser.ID))
	params.RegistrySet.UserRegistry = userRegistry
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()
	expectedCount := must.Must(params.RegistrySet.LocationRegistry.Count(c.Context())) + 1

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusCreated)
	body := rr.Body.Bytes()
	c.Assert(body, checkers.JSONPathEquals("$.data.type"), "locations")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.name"), "LocationResponse New")
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.address"), "Address New")
	c.Assert(body, checkers.JSONPathMatches("$.data.id", qt.Matches), "^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$")

	cnt, err := params.RegistrySet.LocationRegistry.Count(c.Context())
	c.Assert(err, qt.IsNil)
	c.Assert(cnt, qt.Equals, expectedCount)
}

func TestLocationsGet(t *testing.T) {
	c := qt.New(t)

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(newLocationRegistry(testUser.ID))
	params.RegistrySet.UserRegistry = userRegistry
	params.RegistrySet.AreaRegistry = newAreaRegistry(params.RegistrySet.LocationRegistry, testUser.ID)
	locations := must.Must(params.RegistrySet.LocationRegistry.List(c.Context()))
	location := locations[0]

	req, err := http.NewRequest("GET", "/api/v1/locations/"+location.ID, nil)
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

	areas, err := params.RegistrySet.LocationRegistry.GetAreas(c.Context(), location.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(body, checkers.JSONPathMatches("$.data.attributes.areas", qt.HasLen), len(areas))
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.areas[0]"), areas[0])
	c.Assert(body, checkers.JSONPathEquals("$.data.attributes.areas[1]"), areas[1])
}

func TestLocationsList(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	// Override with specific location registry for this test
	params.RegistrySet.LocationRegistry = newLocationRegistry(testUser.ID)
	expectedLocations := must.Must(params.RegistrySet.LocationRegistry.List(c.Context()))

	req, err := http.NewRequest("GET", "/api/v1/locations", nil)
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

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(newLocationRegistry(testUser.ID))
	params.RegistrySet.UserRegistry = userRegistry
	locations := must.Must(params.RegistrySet.LocationRegistry.List(c.Context()))
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

	req, err := http.NewRequest("PUT", "/api/v1/locations/"+location.ID, updateBuf)
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

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(memory.NewLocationRegistry()) // Empty registry
	params.RegistrySet.UserRegistry = userRegistry

	req, err := http.NewRequest("GET", "/api/v1/locations", nil)
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

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(newLocationRegistry(testUser.ID))
	params.RegistrySet.UserRegistry = userRegistry

	invalidID := "invalid-id"

	req, err := http.NewRequest("GET", "/api/v1/locations/"+invalidID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestLocationsUpdate_PartialData(t *testing.T) {
	c := qt.New(t)

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(newLocationRegistry(testUser.ID))
	params.RegistrySet.UserRegistry = userRegistry
	locations := must.Must(params.RegistrySet.LocationRegistry.List(c.Context()))
	location := locations[0]

	updateObj := &jsonapi.LocationRequest{
		Data: &jsonapi.LocationData{
			ID:   location.ID,
			Type: "locations",
			Attributes: models.WithID(location.ID, &models.Location{
				Name: "Updated Name",
				// Address field is not provided
			}),
		},
	}
	updateData := must.Must(json.Marshal(updateObj))
	updateBuf := bytes.NewReader(updateData)

	req, err := http.NewRequest("PUT", "/api/v1/locations/"+location.ID, updateBuf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	body := rr.Body.Bytes()
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("Body: %s", body))
	c.Assert(body, checkers.JSONPathEquals("$.errors[0].error.error.data.attributes.address"), "cannot be blank")
}

func TestLocationsUpdate_ForeignIDInRequestBody(t *testing.T) {
	c := qt.New(t)

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(newLocationRegistry(testUser.ID))
	params.RegistrySet.UserRegistry = userRegistry
	locations := must.Must(params.RegistrySet.LocationRegistry.List(c.Context()))
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

	req, err := http.NewRequest("PUT", "/api/v1/locations/"+location.ID, updateBuf)
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

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(newLocationRegistry(testUser.ID))
	params.RegistrySet.UserRegistry = userRegistry

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

	req, err := http.NewRequest("PUT", "/api/v1/locations/"+unknownID, updateBuf)
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

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(newLocationRegistry(testUser.ID))
	params.RegistrySet.UserRegistry = userRegistry

	missingID := "missing-id"

	req, err := http.NewRequest("DELETE", "/api/v1/locations/"+missingID, nil)
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

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(newLocationRegistry(testUser.ID))
	params.RegistrySet.UserRegistry = userRegistry

	// Construct a request body with an unexpected data structure
	// For example, sending an array instead of an object
	data := []byte(`[{"name": "LocationResponse New", "address": "Address New"}]`)

	req, err := http.NewRequest("POST", "/api/v1/locations", bytes.NewReader(data))
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

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(newLocationRegistry(testUser.ID))
	params.RegistrySet.UserRegistry = userRegistry
	locations := must.Must(params.RegistrySet.LocationRegistry.List(c.Context()))
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

	req, err := http.NewRequest("PUT", "/api/v1/locations/"+location.ID, nestedUpdateBuf)
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

	// Create a user first to get the user ID for the location registry
	userRegistry, testUser := newUserRegistryWithUser()
	params, _ := newParamsWithLocationRegistry(newLocationRegistry(testUser.ID))
	params.RegistrySet.UserRegistry = userRegistry
	locations := must.Must(params.RegistrySet.LocationRegistry.List(c.Context()))
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
