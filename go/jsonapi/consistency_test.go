package jsonapi_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
)

func TestResponseConsistency_EmptySlicesNotNull(t *testing.T) {
	tests := []struct {
		name     string
		response any
		jsonPath string
	}{
		{
			name:     "FilesResponse with nil slice",
			response: jsonapi.NewFilesResponse(nil, 0),
			jsonPath: "data",
		},
		{
			name:     "FilesResponse with empty slice",
			response: jsonapi.NewFilesResponse([]*models.FileEntity{}, 0),
			jsonPath: "data",
		},
		{
			name:     "ImagesResponse with nil slice",
			response: jsonapi.NewImagesResponse(nil, 0),
			jsonPath: "data",
		},
		{
			name:     "ImagesResponse with empty slice",
			response: jsonapi.NewImagesResponse([]*models.Image{}, 0),
			jsonPath: "data",
		},
		{
			name:     "InvoicesResponse with nil slice",
			response: jsonapi.NewInvoicesResponse(nil, 0),
			jsonPath: "data",
		},
		{
			name:     "InvoicesResponse with empty slice",
			response: jsonapi.NewInvoicesResponse([]*models.Invoice{}, 0),
			jsonPath: "data",
		},
		{
			name:     "ManualsResponse with nil slice",
			response: jsonapi.NewManualsResponse(nil, 0),
			jsonPath: "data",
		},
		{
			name:     "ManualsResponse with empty slice",
			response: jsonapi.NewManualsResponse([]*models.Manual{}, 0),
			jsonPath: "data",
		},
		{
			name:     "ExportsResponse with nil slice",
			response: jsonapi.NewExportsResponse(nil, 0),
			jsonPath: "data",
		},
		{
			name:     "ExportsResponse with empty slice",
			response: jsonapi.NewExportsResponse([]*models.Export{}, 0),
			jsonPath: "data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			// Marshal to JSON
			jsonBytes, err := json.Marshal(tt.response)
			c.Assert(err, qt.IsNil)

			// Parse back to verify structure
			var result map[string]any
			err = json.Unmarshal(jsonBytes, &result)
			c.Assert(err, qt.IsNil)

			// Check that data field exists and is an array, not null
			data, exists := result[tt.jsonPath]
			c.Assert(exists, qt.IsTrue, qt.Commentf("Field %s should exist", tt.jsonPath))
			c.Assert(data, qt.IsNotNil, qt.Commentf("Field %s should not be null", tt.jsonPath))

			// Verify it's an array
			dataArray, ok := data.([]any)
			c.Assert(ok, qt.IsTrue, qt.Commentf("Field %s should be an array", tt.jsonPath))
			c.Assert(dataArray, qt.HasLen, 0, qt.Commentf("Field %s should be an empty array", tt.jsonPath))

			// Verify the JSON contains [] not null for the data field
			jsonStr := string(jsonBytes)
			c.Assert(jsonStr, qt.Contains, `"`+tt.jsonPath+`":[]`, qt.Commentf("JSON should contain empty array for %s", tt.jsonPath))
			c.Assert(jsonStr, qt.Not(qt.Contains), `"`+tt.jsonPath+`":null`, qt.Commentf("JSON should not contain null for %s", tt.jsonPath))
		})
	}
}
