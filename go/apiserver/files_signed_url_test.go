package apiserver_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
)

func TestFilesAPI_GenerateSignedURL_JSONAPIFormat(t *testing.T) {
	c := qt.New(t)

	// Create a test response to verify JSON:API format
	fileID := "test-file-123"
	signedURL := "https://example.com/api/v1/files/download/test-file-123.pdf?sig=abc&exp=123&uid=user"

	response := jsonapi.NewSignedFileURLResponse(fileID, signedURL)

	c.Run("JSON:API response structure", func(c *qt.C) {
		// Verify the response structure
		c.Assert(response.ID, qt.Equals, fileID)
		c.Assert(response.Type, qt.Equals, "urls")
		c.Assert(response.Attributes.URL, qt.Equals, signedURL)
		c.Assert(response.HTTPStatusCode, qt.Equals, 0) // Default value
	})

	c.Run("WithStatusCode method", func(c *qt.C) {
		responseWithStatus := response.WithStatusCode(http.StatusCreated)
		c.Assert(responseWithStatus.HTTPStatusCode, qt.Equals, http.StatusCreated)
		c.Assert(responseWithStatus.ID, qt.Equals, fileID)
		c.Assert(responseWithStatus.Type, qt.Equals, "urls")
		c.Assert(responseWithStatus.Attributes.URL, qt.Equals, signedURL)

		// Verify original response is unchanged
		c.Assert(response.HTTPStatusCode, qt.Equals, 0)
	})

	c.Run("JSON serialization", func(c *qt.C) {
		// Test JSON serialization
		jsonData, err := json.Marshal(response)
		c.Assert(err, qt.IsNil)

		// Parse back to verify structure
		var parsed map[string]any
		err = json.Unmarshal(jsonData, &parsed)
		c.Assert(err, qt.IsNil)

		// Verify JSON structure
		c.Assert(parsed["id"], qt.Equals, fileID)
		c.Assert(parsed["type"], qt.Equals, "urls")

		attributes, ok := parsed["attributes"].(map[string]any)
		c.Assert(ok, qt.IsTrue)
		c.Assert(attributes["url"], qt.Equals, signedURL)

		// Verify HTTPStatusCode is not included in JSON
		_, exists := parsed["HTTPStatusCode"]
		c.Assert(exists, qt.IsFalse)
	})

	c.Run("Render method", func(c *qt.C) {
		// Test the Render method
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		err := response.Render(w, req)
		c.Assert(err, qt.IsNil)

		// Verify default status code is set
		c.Assert(w.Code, qt.Equals, http.StatusOK)
	})

	c.Run("Render method with custom status code", func(c *qt.C) {
		// Test the Render method with custom status code using render.Render
		responseWithStatus := response.WithStatusCode(http.StatusCreated)
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()

		err := render.Render(w, req, responseWithStatus)
		c.Assert(err, qt.IsNil)

		// Verify custom status code is set
		c.Assert(w.Code, qt.Equals, http.StatusCreated)
	})

	c.Run("Response format matches JSON:API specification", func(c *qt.C) {
		// Verify the response follows JSON:API specification
		jsonData, err := json.Marshal(response)
		c.Assert(err, qt.IsNil)

		expectedJSON := `{
			"id": "test-file-123",
			"type": "urls",
			"attributes": {
				"url": "https://example.com/api/v1/files/download/test-file-123.pdf?sig=abc&exp=123&uid=user"
			}
		}`

		var expected, actual map[string]any
		err = json.Unmarshal([]byte(expectedJSON), &expected)
		c.Assert(err, qt.IsNil)

		err = json.Unmarshal(jsonData, &actual)
		c.Assert(err, qt.IsNil)

		c.Assert(actual, qt.DeepEquals, expected)
	})
}

func TestSignedFileUrlResponse_Consistency(t *testing.T) {
	c := qt.New(t)

	c.Run("consistent with other JSON:API responses", func(c *qt.C) {
		// Verify that SignedFileURLResponse follows the same pattern as other responses
		fileID := "test-file-456"
		signedURL := "https://example.com/signed-url"

		response := jsonapi.NewSignedFileURLResponse(fileID, signedURL)

		// Should have the same structure as other JSON:API responses
		c.Assert(response.ID, qt.Not(qt.Equals), "")
		c.Assert(response.Type, qt.Not(qt.Equals), "")
		c.Assert(response.Attributes, qt.Not(qt.IsNil))

		// Should implement the same methods
		responseWithStatus := response.WithStatusCode(http.StatusAccepted)
		c.Assert(responseWithStatus.HTTPStatusCode, qt.Equals, http.StatusAccepted)

		// Should be renderable
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		err := response.Render(w, req)
		c.Assert(err, qt.IsNil)
	})

	c.Run("URLData structure", func(c *qt.C) {
		urlData := jsonapi.URLData{
			URL: "https://example.com/test-url",
		}

		// Test JSON serialization of URLData
		jsonData, err := json.Marshal(urlData)
		c.Assert(err, qt.IsNil)

		var parsed map[string]any
		err = json.Unmarshal(jsonData, &parsed)
		c.Assert(err, qt.IsNil)

		c.Assert(parsed["url"], qt.Equals, "https://example.com/test-url")
	})
}
