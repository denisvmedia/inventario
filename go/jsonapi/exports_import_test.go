package jsonapi_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/jsonapi"
)

func TestImportExportRequest_Validation(t *testing.T) {
	ctx := context.Background()

	// Test cases for happy path
	happyPathTests := []struct {
		name        string
		requestData jsonapi.ImportExportRequest
	}{
		{
			name: "valid import request",
			requestData: jsonapi.ImportExportRequest{
				Data: &jsonapi.ImportExportRequestData{
					Type: "exports",
					Attributes: &jsonapi.ImportExportAttributes{
						Description:    "Test import",
						SourceFilePath: "test-file.xml",
					},
				},
			},
		},
	}

	for _, tt := range happyPathTests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			err := tt.requestData.ValidateWithContext(ctx)
			c.Assert(err, qt.IsNil)

			// Also test that the attributes validate correctly
			if tt.requestData.Data != nil && tt.requestData.Data.Attributes != nil {
				err = tt.requestData.Data.Attributes.ValidateWithContext(ctx)
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestImportExportRequest_ValidationErrors(t *testing.T) {
	ctx := context.Background()

	// Test cases for unhappy path
	unhappyPathTests := []struct {
		name        string
		requestData jsonapi.ImportExportRequest
		expectError bool
	}{
		{
			name: "missing data",
			requestData: jsonapi.ImportExportRequest{
				Data: nil,
			},
			expectError: true,
		},
		{
			name: "missing attributes",
			requestData: jsonapi.ImportExportRequest{
				Data: &jsonapi.ImportExportRequestData{
					Type:       "exports",
					Attributes: nil,
				},
			},
			expectError: true,
		},
		{
			name: "invalid type",
			requestData: jsonapi.ImportExportRequest{
				Data: &jsonapi.ImportExportRequestData{
					Type: "invalid",
					Attributes: &jsonapi.ImportExportAttributes{
						Description:    "Test import",
						SourceFilePath: "test-file.xml",
					},
				},
			},
			expectError: true,
		},
		{
			name: "empty description",
			requestData: jsonapi.ImportExportRequest{
				Data: &jsonapi.ImportExportRequestData{
					Type: "exports",
					Attributes: &jsonapi.ImportExportAttributes{
						Description:    "",
						SourceFilePath: "test-file.xml",
					},
				},
			},
			expectError: true,
		},
		{
			name: "empty source file path",
			requestData: jsonapi.ImportExportRequest{
				Data: &jsonapi.ImportExportRequestData{
					Type: "exports",
					Attributes: &jsonapi.ImportExportAttributes{
						Description:    "Test import",
						SourceFilePath: "",
					},
				},
			},
			expectError: true,
		},
		{
			name: "description too long",
			requestData: jsonapi.ImportExportRequest{
				Data: &jsonapi.ImportExportRequestData{
					Type: "exports",
					Attributes: &jsonapi.ImportExportAttributes{
						Description:    string(make([]byte, 501)), // 501 characters, exceeds limit
						SourceFilePath: "test-file.xml",
					},
				},
			},
			expectError: true,
		},
	}

	for _, tt := range unhappyPathTests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			err := tt.requestData.ValidateWithContext(ctx)
			if tt.expectError {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestImportExportRequest_JSONSerialization(t *testing.T) {
	c := qt.New(t)

	// Test JSON serialization and deserialization
	originalRequest := jsonapi.ImportExportRequest{
		Data: &jsonapi.ImportExportRequestData{
			Type: "exports",
			Attributes: &jsonapi.ImportExportAttributes{
				Description:    "Test import description",
				SourceFilePath: "export_full_database_20250611_185919-1749815145.xml",
			},
		},
	}

	// Serialize to JSON
	jsonData, err := json.Marshal(originalRequest)
	c.Assert(err, qt.IsNil)

	// Deserialize from JSON
	var deserializedRequest jsonapi.ImportExportRequest
	err = json.Unmarshal(jsonData, &deserializedRequest)
	c.Assert(err, qt.IsNil)

	// Verify the data is correct
	c.Assert(deserializedRequest.Data, qt.IsNotNil)
	c.Assert(deserializedRequest.Data.Type, qt.Equals, "exports")
	c.Assert(deserializedRequest.Data.Attributes, qt.IsNotNil)
	c.Assert(deserializedRequest.Data.Attributes.Description, qt.Equals, "Test import description")
	c.Assert(deserializedRequest.Data.Attributes.SourceFilePath, qt.Equals, "export_full_database_20250611_185919-1749815145.xml")

	// Verify validation passes
	err = deserializedRequest.ValidateWithContext(context.Background())
	c.Assert(err, qt.IsNil)
}

func TestImportExportRequest_ExampleJSON(t *testing.T) {
	c := qt.New(t)

	// Test with the exact JSON structure from the user's request
	jsonStr := `{"data":{"type":"exports","attributes":{"description":"Last time export","source_file_path":"export_full_database_20250611_185919-1749815145.xml"}}}`

	var request jsonapi.ImportExportRequest
	err := json.Unmarshal([]byte(jsonStr), &request)
	c.Assert(err, qt.IsNil)

	// Verify the structure is correct
	c.Assert(request.Data, qt.IsNotNil)
	c.Assert(request.Data.Type, qt.Equals, "exports")
	c.Assert(request.Data.Attributes, qt.IsNotNil)
	c.Assert(request.Data.Attributes.Description, qt.Equals, "Last time export")
	c.Assert(request.Data.Attributes.SourceFilePath, qt.Equals, "export_full_database_20250611_185919-1749815145.xml")

	// Verify validation passes
	err = request.ValidateWithContext(context.Background())
	c.Assert(err, qt.IsNil)

	// Test attributes validation separately
	err = request.Data.Attributes.ValidateWithContext(context.Background())
	c.Assert(err, qt.IsNil)
}
