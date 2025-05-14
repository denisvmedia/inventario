package jsonapi

import (
	"net/http"

	"github.com/shopspring/decimal"
)

// ValueResponse represents a JSON API response for commodity values.
type ValueResponse struct {
	Data *ValueData `json:"data"`
}

// ValueData represents the data part of a JSON API response for commodity values.
type ValueData struct {
	Type       string      `json:"type"`
	ID         string      `json:"id"`
	Attributes *ValueAttrs `json:"attributes"`
}

// ValueAttrs represents the attributes of a value response.
type ValueAttrs struct {
	GlobalTotal    decimal.Decimal            `json:"global_total"`
	LocationTotals map[string]decimal.Decimal `json:"location_totals"`
	AreaTotals     map[string]decimal.Decimal `json:"area_totals"`
}

// Render implements the render.Renderer interface for ValueResponse.
func (*ValueResponse) Render(_w http.ResponseWriter, _r *http.Request) error {
	return nil
}

// NewValueResponse creates a new ValueResponse.
func NewValueResponse(globalTotal decimal.Decimal, locationTotals, areaTotals map[string]decimal.Decimal) *ValueResponse {
	return &ValueResponse{
		Data: &ValueData{
			Type: "values",
			ID:   "global",
			Attributes: &ValueAttrs{
				GlobalTotal:    globalTotal,
				LocationTotals: locationTotals,
				AreaTotals:     areaTotals,
			},
		},
	}
}
