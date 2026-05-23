package mock_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/aivision"
	"github.com/denisvmedia/inventario/internal/aivision/mock"
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
	c.Assert(errors.Is(err, wanted), qt.IsTrue)
}

func TestMockProvider_WithDefaultError(t *testing.T) {
	c := qt.New(t)
	wanted := errors.New("baseline")
	provider := mock.New(mock.WithDefaultError(wanted))

	_, err := provider.Scan(context.Background(), aivision.ScanRequest{
		Photos: []aivision.PhotoInput{{ContentType: "image/jpeg", Data: []byte("x")}},
	})
	c.Assert(errors.Is(err, wanted), qt.IsTrue)
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
