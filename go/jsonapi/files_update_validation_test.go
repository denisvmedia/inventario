package jsonapi_test

import (
	"context"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/jsonapi"
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
// accepted: a metadata-only edit carries no `path`, and the handler keeps the
// persisted one.
func TestFileUpdateRequestValidate_Passes(t *testing.T) {
	cases := []struct {
		name  string
		attrs jsonapi.FileUpdateRequestFileData
	}{
		{
			name: "metadata-only edit: title + tags, no path",
			attrs: jsonapi.FileUpdateRequestFileData{
				Title: "Invoice 2026",
				Tags:  []string{"invoice"},
			},
		},
		{
			name: "rename: path supplied",
			attrs: jsonapi.FileUpdateRequestFileData{
				Path: "renamed-file.jpg",
			},
		},
		{
			// 255 characters, 510 bytes in UTF-8: the limit counts characters.
			name:  "title of 255 Cyrillic characters",
			attrs: jsonapi.FileUpdateRequestFileData{Title: strings.Repeat("ж", 255)},
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

// TestFileUpdateRequestValidate_Rejects covers the bad request shapes that MUST
// be rejected. Attributes is a struct value whose validator has a pointer
// receiver, so these only fail when the parent invokes the nested validator
// explicitly (#2033).
func TestFileUpdateRequestValidate_Rejects(t *testing.T) {
	cases := []struct {
		name  string
		attrs jsonapi.FileUpdateRequestFileData
	}{
		{
			name:  "title over 255 characters",
			attrs: jsonapi.FileUpdateRequestFileData{Title: strings.Repeat("t", 256)},
		},
		{
			name:  "title over 255 Czech characters",
			attrs: jsonapi.FileUpdateRequestFileData{Title: strings.Repeat("ř", 256)},
		},
		{
			name:  "description over 1000 characters",
			attrs: jsonapi.FileUpdateRequestFileData{Description: strings.Repeat("d", 1001)},
		},
		{
			name:  "path over 1024 characters",
			attrs: jsonapi.FileUpdateRequestFileData{Path: strings.Repeat("p", 1025)},
		},
		{
			name:  "more than 100 tags",
			attrs: jsonapi.FileUpdateRequestFileData{Tags: make([]string, 101)},
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

func newFileLinkRequest(attrs jsonapi.FileLinkRequestAttributes) *jsonapi.FileLinkRequest {
	return &jsonapi.FileLinkRequest{
		Data: &jsonapi.FileLinkRequestData{Type: "file-links", Attributes: attrs},
	}
}

// TestFileLinkRequestValidate_Passes covers the attach requests the API
// accepts: every inventory entity type, with a meta of its own bucket or none.
func TestFileLinkRequestValidate_Passes(t *testing.T) {
	cases := []struct {
		name  string
		attrs jsonapi.FileLinkRequestAttributes
	}{
		{
			name:  "commodity, meta omitted (server picks the bucket)",
			attrs: jsonapi.FileLinkRequestAttributes{EntityType: "commodity", EntityID: "commodity-1"},
		},
		{
			name:  "commodity with documents meta",
			attrs: jsonapi.FileLinkRequestAttributes{EntityType: "commodity", EntityID: "commodity-1", Meta: "documents"},
		},
		{
			name:  "location with files meta",
			attrs: jsonapi.FileLinkRequestAttributes{EntityType: "location", EntityID: "location-1", Meta: "files"},
		},
		{
			name:  "area with images meta",
			attrs: jsonapi.FileLinkRequestAttributes{EntityType: "area", EntityID: "area-1", Meta: "images"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			err := newFileLinkRequest(tc.attrs).ValidateWithContext(context.Background())
			c.Assert(err, qt.IsNil)
		})
	}
}

// TestFileLinkRequestValidate_Rejects covers the attach requests the API
// refuses before touching a registry.
func TestFileLinkRequestValidate_Rejects(t *testing.T) {
	cases := []struct {
		name  string
		attrs jsonapi.FileLinkRequestAttributes
	}{
		{
			name:  "commodity with bogus meta",
			attrs: jsonapi.FileLinkRequestAttributes{EntityType: "commodity", EntityID: "commodity-1", Meta: "BOGUS"},
		},
		{
			name:  "commodity with the location bucket",
			attrs: jsonapi.FileLinkRequestAttributes{EntityType: "commodity", EntityID: "commodity-1", Meta: "files"},
		},
		{
			name:  "location with the commodity bucket",
			attrs: jsonapi.FileLinkRequestAttributes{EntityType: "location", EntityID: "location-1", Meta: "documents"},
		},
		{
			name:  "missing entity id",
			attrs: jsonapi.FileLinkRequestAttributes{EntityType: "commodity"},
		},
		{
			name:  "export is not attachable",
			attrs: jsonapi.FileLinkRequestAttributes{EntityType: "export", EntityID: "export-1", Meta: "xml-1.0"},
		},
		{
			name:  "unknown entity type",
			attrs: jsonapi.FileLinkRequestAttributes{EntityType: "spaceship", EntityID: "x-1"},
		},
		{
			name:  "missing entity type",
			attrs: jsonapi.FileLinkRequestAttributes{EntityID: "x-1"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			err := newFileLinkRequest(tc.attrs).ValidateWithContext(context.Background())
			c.Assert(err, qt.IsNotNil)
		})
	}
}

// TestFileLinkRequestValidate_Envelope guards the outer envelope: a missing
// data object or a wrong type is rejected regardless of the attributes.
func TestFileLinkRequestValidate_Envelope(t *testing.T) {
	c := qt.New(t)
	c.Assert((&jsonapi.FileLinkRequest{}).ValidateWithContext(context.Background()), qt.IsNotNil)

	req := newFileLinkRequest(jsonapi.FileLinkRequestAttributes{EntityType: "commodity", EntityID: "commodity-1"})
	req.Data.Type = "files"
	c.Assert(req.ValidateWithContext(context.Background()), qt.IsNotNil)
}

// TestFileRequestDataValidate_TitleLength checks that the title limit of a
// new file counts characters, as the update request and the model do.
func TestFileRequestDataValidate_TitleLength(t *testing.T) {
	c := qt.New(t)
	ok := jsonapi.FileRequestData{Title: strings.Repeat("ж", 255)}
	c.Assert(ok.ValidateWithContext(context.Background()), qt.IsNil)
	tooLong := jsonapi.FileRequestData{Title: strings.Repeat("ж", 256)}
	c.Assert(tooLong.ValidateWithContext(context.Background()), qt.IsNotNil)
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
