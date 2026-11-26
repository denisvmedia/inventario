package mock_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/aivision"
	"go.5x5.cz/inventario/internal/aivision/mock"
)

func TestMockProvider_DefaultResultCoversAllFields(t *testing.T) {
	c := qt.New(t)
	provider := mock.New()

	result, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{Filename: "x.jpg", ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result, qt.IsNotNil)
	c.Assert(provider.Name(), qt.Equals, mock.Name)

	for _, field := range aivision.AllFieldNames {
		_, ok := result.Fields[field]
		c.Assert(ok, qt.IsTrue, qt.Commentf("expected default result to populate field %q", field))
	}
}

func TestMockProvider_ContextOverride(t *testing.T) {
	c := qt.New(t)
	provider := mock.New()

	custom := aivision.ScanResult{
		Fields: map[string]aivision.FieldGuess{
			aivision.FieldNameName: {Value: "Custom Item", Confidence: 1.0},
		},
	}
	ctx := mock.WithResultOverride(context.Background(), custom)
	result, err := provider.Scan(ctx, aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result.Fields[aivision.FieldNameName].Value, qt.Equals, "Custom Item")
	// Confirm only the overridden field is present.
	c.Assert(result.Fields, qt.HasLen, 1)
}

func TestMockProvider_ErrorOverride(t *testing.T) {
	c := qt.New(t)
	provider := mock.New()

	wanted := errors.New("boom")
	ctx := mock.WithErrorOverride(context.Background(), wanted)
	_, err := provider.Scan(ctx, aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.ErrorIs, wanted)
}

func TestMockProvider_WithDefaultError(t *testing.T) {
	c := qt.New(t)
	wanted := errors.New("baseline")
	provider := mock.New(mock.WithDefaultError(wanted))

	_, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.ErrorIs, wanted)
}

func TestMockProvider_InvoiceFilenameReturnsMultipleItems(t *testing.T) {
	c := qt.New(t)
	provider := mock.New()

	// An invoice can be a photo/scan, not only a PDF — the mock keys off the
	// filename, so a JPG named like an invoice yields the multi-item result.
	result, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{
			{Filename: "invoice-scan.jpg", ContentType: "image/jpeg", Data: []byte("x")},
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(len(result.Items) >= 2, qt.IsTrue)
	// Fields mirrors the first (most prominent) item, per the provider contract.
	c.Assert(result.Fields[aivision.FieldNameName].Value, qt.Equals, result.Items[0].Fields[aivision.FieldNameName].Value)
}

func TestMockProvider_PlainPhotoStaysSingleItem(t *testing.T) {
	c := qt.New(t)
	provider := mock.New()
	result, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{
			{Filename: "photo.jpg", ContentType: "image/jpeg", Data: []byte("x")},
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result.Items, qt.HasLen, 0)
}

func TestMockProvider_WithDefaultResult(t *testing.T) {
	c := qt.New(t)
	custom := aivision.ScanResult{
		Fields: map[string]aivision.FieldGuess{
			aivision.FieldNameName: {Value: "Custom", Confidence: 0.5},
		},
	}
	provider := mock.New(mock.WithDefaultResult(custom))
	result, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result.Fields[aivision.FieldNameName].Value, qt.Equals, "Custom")
}

// TestMockProvider_SuggestsEveryFile: the canned results carry one
// suggestion per file, read from the filename, the same on every call.
func TestMockProvider_SuggestsEveryFile(t *testing.T) {
	c := qt.New(t)
	provider := mock.New()
	req := aivision.ScanRequest{Photos: []aivision.PhotoInput{
		{Filename: "kettle.jpg", ContentType: "image/jpeg", Data: []byte("a")},
		{Filename: "Faktura-2024.pdf", ContentType: "application/pdf", Data: []byte("b")},
		{Filename: "uctenka.jpg", ContentType: "image/jpeg", Data: []byte("c")},
		{Filename: "user-guide.pdf", ContentType: "application/pdf", Data: []byte("d")},
		{Filename: "warranty-card.png", ContentType: "image/png", Data: []byte("e")},
		{Filename: "scan.pdf", ContentType: "application/pdf", Data: []byte("f")},
	}}

	result, err := provider.Scan(context.Background(), req)
	c.Assert(err, qt.IsNil)
	got := make([][2]any, 0, len(result.Files))
	for i, f := range result.Files {
		c.Check(f.Index, qt.Equals, i)
		got = append(got, [2]any{f.DocumentType.Value, f.Title.Value})
	}
	c.Check(got, qt.DeepEquals, [][2]any{
		{"photo", "Sample product photo"},
		{"invoice", "Sample Store invoice, March 2024"},
		{"receipt", "Sample Store receipt, March 2024"},
		{"manual", "Sample user manual"},
		{"warranty", "Sample warranty card"},
		{"other", "Sample document"},
	})

	again, err := provider.Scan(context.Background(), req)
	c.Assert(err, qt.IsNil)
	c.Check(again.Files, qt.DeepEquals, result.Files)
	c.Check(mock.FileGuessesFor(req.Photos), qt.DeepEquals, result.Files)
}

// TestMockProvider_SingleItemSuggestsItsFile: the single-product default
// suggests its file too.
func TestMockProvider_SingleItemSuggestsItsFile(t *testing.T) {
	c := qt.New(t)
	result, err := mock.New().Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{Filename: "photo.jpg", ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(result.Items, qt.HasLen, 0)
	c.Assert(result.Files, qt.HasLen, 1)
	c.Check(result.Files[0].DocumentType.Value, qt.Equals, aivision.DocumentTypePhoto)
}

// TestMockProvider_GivenResultsHaveNoFileSuggestions: a result a test hands
// the mock comes back as given.
func TestMockProvider_GivenResultsHaveNoFileSuggestions(t *testing.T) {
	c := qt.New(t)
	given := aivision.ScanResult{Fields: map[string]aivision.FieldGuess{aivision.FieldNameName: {Value: "Given", Confidence: 0.5}}}
	req := aivision.ScanRequest{Photos: []aivision.PhotoInput{{Filename: "invoice.pdf", ContentType: "application/pdf", Data: []byte("x")}}}

	fromDefault, err := mock.New(mock.WithDefaultResult(given)).Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{Filename: "photo.jpg", ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(err, qt.IsNil)
	c.Check(fromDefault.Files, qt.IsNil)

	fromContext, err := mock.New().Scan(mock.WithResultOverride(context.Background(), given), req)
	c.Assert(err, qt.IsNil)
	c.Check(fromContext.Files, qt.IsNil)
}

// TestMockProvider_ResultsDoNotShareFileSuggestions: changing a returned
// suggestion leaves the next result alone.
func TestMockProvider_ResultsDoNotShareFileSuggestions(t *testing.T) {
	c := qt.New(t)
	given := aivision.ScanResult{Files: []aivision.FileGuess{{
		Index: 0, Title: &aivision.FieldGuess{Value: "Kept", Confidence: 0.5},
	}}}
	provider := mock.New(mock.WithDefaultResult(given))
	req := aivision.ScanRequest{Photos: []aivision.PhotoInput{{Filename: "photo.jpg", ContentType: "image/jpeg", Data: []byte("x")}}}

	first, err := provider.Scan(context.Background(), req)
	c.Assert(err, qt.IsNil)
	first.Files[0].Title.Value = "Changed"

	second, err := provider.Scan(context.Background(), req)
	c.Assert(err, qt.IsNil)
	c.Check(second.Files[0].Title.Value, qt.Equals, "Kept")
}
