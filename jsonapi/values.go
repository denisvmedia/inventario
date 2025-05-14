package jsonapi

import (
	"net/http"

	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/valuation"
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
func (vr *ValueResponse) Render(w http.ResponseWriter, r *http.Request) error {
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

// DetailedValueResponse represents a JSON API response for detailed commodity values.
type DetailedValueResponse struct {
	Data []*DetailedValueData `json:"data"`
}

// DetailedValueData represents the data part of a JSON API response for detailed commodity values.
type DetailedValueData struct {
	Type       string                    `json:"type"`
	ID         string                    `json:"id"`
	Attributes *valuation.CommodityValue `json:"attributes"`
}

// Render implements the render.Renderer interface for DetailedValueResponse.
func (dvr *DetailedValueResponse) Render(w http.ResponseWriter, r *http.Request) error {
	return nil
}

// NewDetailedValueResponse creates a new DetailedValueResponse.
func NewDetailedValueResponse(commodityValues []*valuation.CommodityValue) *DetailedValueResponse {
	data := make([]*DetailedValueData, len(commodityValues))
	for i, cv := range commodityValues {
		data[i] = &DetailedValueData{
			Type:       "commodity_values",
			ID:         cv.CommodityID,
			Attributes: cv,
		}
	}
	return &DetailedValueResponse{
		Data: data,
	}
}
