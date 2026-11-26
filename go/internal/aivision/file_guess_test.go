package aivision_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/aivision"
)

func fileGuess(index int, docType string, docConfidence float64, title string, titleConfidence float64) aivision.FileGuess {
	g := aivision.FileGuess{Index: index}
	if docType != "" {
		g.DocumentType = &aivision.FieldGuess{Value: docType, Confidence: docConfidence}
	}
	if title != "" {
		g.Title = &aivision.FieldGuess{Value: title, Confidence: titleConfidence}
	}
	return g
}

// TestToScanResult_Files: the per-file suggestions are checked against the
// files of the request. A bad entry, or a bad half of one, is left out with a
// warning and never fails the scan.
func TestToScanResult_Files(t *testing.T) {
	longTitle := strings.Repeat("ab ", 60) + "end"
	tests := []struct {
		name         string
		files        string
		fileCount    int
		want         []aivision.FileGuess
		wantWarnings int
	}{
		{
			name: "one entry per file, ordered by index",
			files: `[
				{"index":1,"document_type":{"value":"manual","confidence":0.8},"title":{"value":"Bosch manual","confidence":0.7}},
				{"index":0,"document_type":{"value":"invoice","confidence":0.9},"title":{"value":"Bosch invoice, March 2024","confidence":0.9}}
			]`,
			fileCount: 2,
			want: []aivision.FileGuess{
				fileGuess(0, "invoice", 0.9, "Bosch invoice, March 2024", 0.9),
				fileGuess(1, "manual", 0.8, "Bosch manual", 0.7),
			},
		},
		{
			name: "an index past the last file is dropped",
			files: `[
				{"index":0,"document_type":{"value":"photo","confidence":0.9},"title":{"value":"Kettle","confidence":0.9}},
				{"index":2,"document_type":{"value":"invoice","confidence":0.9},"title":{"value":"Invoice","confidence":0.9}}
			]`,
			fileCount:    2,
			want:         []aivision.FileGuess{fileGuess(0, "photo", 0.9, "Kettle", 0.9)},
			wantWarnings: 1,
		},
		{
			name: "negative, fractional, missing and non-numeric indexes are dropped",
			files: `[
				{"index":-1,"document_type":{"value":"invoice","confidence":0.9}},
				{"index":0.5,"document_type":{"value":"invoice","confidence":0.9}},
				{"document_type":{"value":"invoice","confidence":0.9}},
				{"index":"0","document_type":{"value":"invoice","confidence":0.9}}
			]`,
			fileCount:    2,
			wantWarnings: 4,
		},
		{
			name: "a repeated index keeps the first entry",
			files: `[
				{"index":0,"document_type":{"value":"receipt","confidence":0.9},"title":{"value":"First","confidence":0.9}},
				{"index":0,"document_type":{"value":"manual","confidence":0.9},"title":{"value":"Second","confidence":0.9}}
			]`,
			fileCount:    1,
			want:         []aivision.FileGuess{fileGuess(0, "receipt", 0.9, "First", 0.9)},
			wantWarnings: 1,
		},
		{
			name:         "an unknown document type is left out, the title stays",
			files:        `[{"index":0,"document_type":{"value":"brochure","confidence":0.9},"title":{"value":"Leaflet","confidence":0.6}}]`,
			fileCount:    1,
			want:         []aivision.FileGuess{fileGuess(0, "", 0, "Leaflet", 0.6)},
			wantWarnings: 1,
		},
		{
			name:         "an empty title is left out, the document type stays",
			files:        `[{"index":0,"document_type":{"value":"warranty","confidence":0.7},"title":{"value":"  \n ","confidence":0.6}}]`,
			fileCount:    1,
			want:         []aivision.FileGuess{fileGuess(0, "warranty", 0.7, "", 0)},
			wantWarnings: 1,
		},
		{
			name:         "an entry with nothing usable left is dropped",
			files:        `[{"index":0,"document_type":{"value":7,"confidence":0.7},"title":{"value":"","confidence":0.6}}]`,
			fileCount:    1,
			wantWarnings: 1,
		},
		{
			name:         "an entry with neither part is dropped",
			files:        `[{"index":0}]`,
			fileCount:    1,
			wantWarnings: 1,
		},
		{
			name:         "the document type is matched without case or spaces",
			files:        `[{"index":0,"document_type":{"value":" Invoice ","confidence":0.9}}]`,
			fileCount:    1,
			want:         []aivision.FileGuess{fileGuess(0, "invoice", 0.9, "", 0)},
			wantWarnings: 0,
		},
		{
			name:      "a title is trimmed, its whitespace collapsed and its length capped",
			files:     `[{"index":0,"title":{"value":"  ` + longTitle + `\n\tmore ","confidence":0.5}}]`,
			fileCount: 1,
			want:      []aivision.FileGuess{fileGuess(0, "", 0, strings.TrimSpace(longTitle[:aivision.MaxFileTitleRunes]), 0.5)},
		},
		{
			name:      "control characters in a title become spaces",
			files:     `[{"index":0,"title":{"value":"Bosch\u0000invoice\r\n2024","confidence":0.5}}]`,
			fileCount: 1,
			want:      []aivision.FileGuess{fileGuess(0, "", 0, "Bosch invoice 2024", 0.5)},
		},
		{
			name:         "files that are not a list cost the file suggestions only",
			files:        `{"index":0}`,
			fileCount:    1,
			wantWarnings: 1,
		},
		{
			name:         "an entry that is not an object is dropped",
			files:        `["invoice",{"index":0,"title":{"value":"Kept","confidence":0.5}}]`,
			fileCount:    1,
			want:         []aivision.FileGuess{fileGuess(0, "", 0, "Kept", 0.5)},
			wantWarnings: 1,
		},
		{
			name:      "no files member",
			files:     `null`,
			fileCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			body := `{"items":[{"name":{"value":"Kettle","confidence":0.9}}],"files":` + tt.files + `}`
			res, err := aivision.ToScanResult([]byte(body), tt.fileCount)
			c.Assert(err, qt.IsNil)
			c.Check(res.Fields[aivision.FieldNameName].Value, qt.Equals, "Kettle")
			c.Check(res.Files, qt.DeepEquals, tt.want)
			c.Check(res.Warnings, qt.HasLen, tt.wantWarnings)
			for _, w := range res.Warnings {
				c.Check(w.Code, qt.Equals, "low_confidence")
				c.Check(w.Field, qt.Equals, aivision.FieldNameFiles)
			}
		})
	}
}

// TestToScanResult_FilesKeepTheModelsWarnings: the warnings about dropped
// file suggestions come after the ones the model sent.
func TestToScanResult_FilesKeepTheModelsWarnings(t *testing.T) {
	c := qt.New(t)
	body := `{"items":[{"name":{"value":"Kettle","confidence":0.9}}],
		"warnings":[{"code":"blurry","detail":"out of focus"}],
		"files":[{"index":5,"title":{"value":"Lost","confidence":0.5}}]}`
	res, err := aivision.ToScanResult([]byte(body), 1)
	c.Assert(err, qt.IsNil)
	c.Assert(res.Warnings, qt.HasLen, 2)
	c.Check(res.Warnings[0].Code, qt.Equals, "blurry")
	c.Check(res.Warnings[1].Field, qt.Equals, aivision.FieldNameFiles)
}

func TestScanResult_FillDocumentType(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]aivision.FieldGuess
		files  []aivision.FileGuess
		want   aivision.FieldGuess
	}{
		{
			name:  "the first document of the request wins over photos",
			files: []aivision.FileGuess{fileGuess(0, "photo", 0.9, "", 0), fileGuess(1, "receipt", 0.8, "", 0), fileGuess(2, "manual", 0.7, "", 0)},
			want:  aivision.FieldGuess{Value: "receipt", Confidence: 0.8},
		},
		{
			name:  "photos and other files only give other",
			files: []aivision.FileGuess{fileGuess(0, "photo", 0.9, "", 0), fileGuess(1, "other", 0.4, "", 0)},
			want:  aivision.FieldGuess{Value: "other", Confidence: 0.9},
		},
		{
			name:   "a scan-level value from the model is kept",
			fields: map[string]aivision.FieldGuess{aivision.FieldNameDocumentType: {Value: "manual", Confidence: 0.6}},
			files:  []aivision.FileGuess{fileGuess(0, "invoice", 0.9, "", 0)},
			want:   aivision.FieldGuess{Value: "manual", Confidence: 0.6},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			res := &aivision.ScanResult{Fields: tt.fields, Files: tt.files}
			res.FillDocumentType()
			c.Check(res.Fields[aivision.FieldNameDocumentType], qt.DeepEquals, tt.want)
		})
	}
}

// TestScanResult_FillDocumentType_NothingToDeriveFrom: without a document
// type among the files the result gets none.
func TestScanResult_FillDocumentType_NothingToDeriveFrom(t *testing.T) {
	tests := map[string][]aivision.FileGuess{
		"titles only": {fileGuess(0, "", 0, "Title only", 0.5)},
		"no files":    nil,
	}
	for name, files := range tests {
		t.Run(name, func(t *testing.T) {
			c := qt.New(t)
			res := &aivision.ScanResult{Files: files}
			res.FillDocumentType()
			_, ok := res.Fields[aivision.FieldNameDocumentType]
			c.Check(ok, qt.IsFalse)
		})
	}
}

// TestScanResult_FillDocumentTypeCopiesTheFields: the derived value does not
// leak into the first product, which may share its fields map.
func TestScanResult_FillDocumentTypeCopiesTheFields(t *testing.T) {
	c := qt.New(t)
	shared := map[string]aivision.FieldGuess{aivision.FieldNameName: {Value: "Kettle", Confidence: 0.9}}
	res := &aivision.ScanResult{
		Fields: shared,
		Items:  []aivision.ScanItem{{Fields: shared}, {Fields: map[string]aivision.FieldGuess{}}},
		Files:  []aivision.FileGuess{fileGuess(0, "invoice", 0.9, "", 0)},
	}
	res.FillDocumentType()
	c.Check(res.Fields[aivision.FieldNameDocumentType].Value, qt.Equals, "invoice")
	_, leaked := shared[aivision.FieldNameDocumentType]
	c.Check(leaked, qt.IsFalse)
}

// TestResponseSchemaAndPrompt_FilesFollowTheRequest: the schema asks for one
// entry per attached file and the prompt says how the files are numbered.
func TestResponseSchemaAndPrompt_FilesFollowTheRequest(t *testing.T) {
	c := qt.New(t)
	req := aivision.ScanRequest{Photos: []aivision.PhotoInput{
		{ContentType: "image/jpeg", Data: []byte{1}},
		{ContentType: "application/pdf", Data: []byte{2}},
		{ContentType: "image/png", Data: []byte{3}},
	}}

	files := aivision.ResponseSchema(req)["properties"].(map[string]any)[aivision.FieldNameFiles].(map[string]any)
	c.Check(files["minItems"], qt.Equals, 3)
	c.Check(files["maxItems"], qt.Equals, 3)
	entry := files["items"].(map[string]any)
	c.Check(entry["required"], qt.DeepEquals, []string{"index", "document_type", "title"})
	props := entry["properties"].(map[string]any)
	c.Check(props["index"].(map[string]any)["maximum"], qt.Equals, 2)
	value := func(name string) map[string]any {
		return props[name].(map[string]any)["properties"].(map[string]any)["value"].(map[string]any)
	}
	c.Check(value("document_type")["enum"], qt.DeepEquals, aivision.FileDocumentTypes)
	c.Check(value("title")["maxLength"], qt.Equals, aivision.MaxFileTitleRunes)

	c.Check(aivision.UserPromptHeader(req), qt.Contains, "There are 3 attached files, labeled File 0 to File 2.")
	c.Check(aivision.SystemPrompt, qt.Contains, `"File N"`)
	c.Check(aivision.FileLabel(2), qt.Equals, "File 2")

	one := aivision.ScanRequest{Photos: req.Photos[:1]}
	c.Check(aivision.UserPromptHeader(one), qt.Contains, "There is 1 attached file, labeled File 0.")
}
