package jsonapi_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/jsonapi"
)

// newFileUpdateRequest builds a well-formed JSON:API update envelope with the
// given attributes so each test can vary just the field under test.
func newFileUpdateRequest(attrs jsonapi.FileUpdateRequestFileData) *jsonapi.FileUpdateRequest {
	return &jsonapi.FileUpdateRequest{
		Data: &jsonapi.FileUpdateRequestData{
			ID:         "file-1",
			Type:       "files",
			Attributes: attrs,
		},
	}
}

// TestFileUpdateRequestValidate_Passes covers the request shapes that MUST be
// accepted — most importantly the live "attach uploaded file to commodity"
// flow (#1983 Part A / PR #2032), which sends a metadata-only PUT carrying no
// `path` and no `linked_entity_meta`. Before #2033 the nested attribute
// validator never ran at all; these guard that enforcing it didn't break a
// live flow.
func TestFileUpdateRequestValidate_Passes(t *testing.T) {
	cases := []struct {
		name  string
		attrs jsonapi.FileUpdateRequestFileData
	}{
		{
			name: "live commodity attach flow: no path, no meta, server derives",
			attrs: jsonapi.FileUpdateRequestFileData{
				LinkedEntityType: "commodity",
				LinkedEntityID:   "commodity-1",
			},
		},
		{
			name: "commodity attach with valid meta",
			attrs: jsonapi.FileUpdateRequestFileData{
				Path:             "receipt.pdf",
				LinkedEntityType: "commodity",
				LinkedEntityID:   "commodity-1",
				LinkedEntityMeta: "images",
			},
		},
		{
			name: "metadata-only edit: title + tags, no link",
			attrs: jsonapi.FileUpdateRequestFileData{
				Title: "Invoice 2026",
				Tags:  []string{"invoice"},
			},
		},
		{
			name: "rename: path supplied, no link",
			attrs: jsonapi.FileUpdateRequestFileData{
				Path: "renamed-file.jpg",
			},
		},
		{
			name: "location attach with valid meta",
			attrs: jsonapi.FileUpdateRequestFileData{
				LinkedEntityType: "location",
				LinkedEntityID:   "location-1",
				LinkedEntityMeta: "files",
			},
		},
		{
			name: "location attach, meta omitted (server derives)",
			attrs: jsonapi.FileUpdateRequestFileData{
				LinkedEntityType: "location",
				LinkedEntityID:   "location-1",
			},
		},
		{
			name: "export relink with valid meta",
			attrs: jsonapi.FileUpdateRequestFileData{
				LinkedEntityType: "export",
				LinkedEntityID:   "export-1",
				LinkedEntityMeta: "xml-1.0",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			req := newFileUpdateRequest(tc.attrs)
			err := req.ValidateWithContext(context.Background())
			c.Assert(err, qt.IsNil)
		})
	}
}

// TestFileUpdateRequestValidate_Rejects covers the bad request shapes that
// MUST now be rejected. Before #2033 the nested FileUpdateRequestFileData
// validator was silently skipped (Attributes is a struct value, the validator
// has a pointer receiver), so every one of these slipped through with a 200.
func TestFileUpdateRequestValidate_Rejects(t *testing.T) {
	cases := []struct {
		name  string
		attrs jsonapi.FileUpdateRequestFileData
	}{
		{
			name: "commodity with bogus meta enum",
			attrs: jsonapi.FileUpdateRequestFileData{
				LinkedEntityType: "commodity",
				LinkedEntityID:   "commodity-1",
				LinkedEntityMeta: "BOGUS",
			},
		},
		{
			name: "commodity link without an id",
			attrs: jsonapi.FileUpdateRequestFileData{
				LinkedEntityType: "commodity",
				LinkedEntityMeta: "images",
			},
		},
		{
			name: "location with meta from the wrong bucket",
			attrs: jsonapi.FileUpdateRequestFileData{
				LinkedEntityType: "location",
				LinkedEntityID:   "location-1",
				LinkedEntityMeta: "manuals",
			},
		},
		{
			name: "export with non-version meta",
			attrs: jsonapi.FileUpdateRequestFileData{
				LinkedEntityType: "export",
				LinkedEntityID:   "export-1",
				LinkedEntityMeta: "images",
			},
		},
		{
			name: "unknown linked_entity_type",
			attrs: jsonapi.FileUpdateRequestFileData{
				LinkedEntityType: "spaceship",
				LinkedEntityID:   "x-1",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			req := newFileUpdateRequest(tc.attrs)
			err := req.ValidateWithContext(context.Background())
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestFileUpdateRequestValidate_TypeRequired guards the outer envelope: a
// missing/wrong `type` is still rejected independently of the attribute rules.
func TestFileUpdateRequestValidate_TypeRequired(t *testing.T) {
	c := qt.New(t)
	req := &jsonapi.FileUpdateRequest{
		Data: &jsonapi.FileUpdateRequestData{
			ID:         "file-1",
			Type:       "not-files",
			Attributes: jsonapi.FileUpdateRequestFileData{Path: "f.jpg"},
		},
	}
	err := req.ValidateWithContext(context.Background())
	c.Assert(err, qt.IsNotNil)
}

// TestFileUpdateRequestValidate_DataRequired guards the top-level required Data.
func TestFileUpdateRequestValidate_DataRequired(t *testing.T) {
	c := qt.New(t)
	req := &jsonapi.FileUpdateRequest{}
	err := req.ValidateWithContext(context.Background())
	c.Assert(err, qt.IsNotNil)
}
