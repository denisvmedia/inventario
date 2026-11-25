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

// NamedTotal pairs a value with the entity's id and human-readable name
// so the dashboard can render top-N grouping cards without a follow-up
// `/locations` or `/areas` round-trip (issue #1330 Copilot review). The
// list shape lets the frontend keep server-defined ordering.
type NamedTotal struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Value decimal.Decimal `json:"value"`
	// UnvaluedCount is the number of in-use items in this location or area
	// that have no known value and are therefore not part of Value.
	UnvaluedCount int `json:"unvalued_count" example:"0" format:"int64"`
}

// ValueAttrs represents the attributes of a value response. The totals sum
// the in-use, non-draft items whose value is known: the current price, or
// the purchase price in the group currency when the current price is
// unknown. A known zero counts as a value. Items with no known value are not
// added as zero; they are counted in UnvaluedCount instead.
type ValueAttrs struct {
	GlobalTotal decimal.Decimal `json:"global_total"`
	// ValuedCount is the number of in-use items that GlobalTotal includes.
	ValuedCount int `json:"valued_count" example:"12" format:"int64"`
	// UnvaluedCount is the number of in-use items GlobalTotal leaves out
	// because neither their current value nor their purchase price is known.
	UnvaluedCount  int          `json:"unvalued_count" example:"2" format:"int64"`
	LocationTotals []NamedTotal `json:"location_totals"`
	AreaTotals     []NamedTotal `json:"area_totals"`
}

// Render implements the render.Renderer interface for ValueResponse.
func (*ValueResponse) Render(_w http.ResponseWriter, _r *http.Request) error {
	return nil
}

// NewValueResponse creates a new ValueResponse.
func NewValueResponse(globalTotal decimal.Decimal, valuedCount, unvaluedCount int, locationTotals, areaTotals []NamedTotal) *ValueResponse {
	return &ValueResponse{
		Data: &ValueData{
			Type: "values",
			ID:   "global",
			Attributes: &ValueAttrs{
				GlobalTotal:    globalTotal,
				ValuedCount:    valuedCount,
				UnvaluedCount:  unvaluedCount,
				LocationTotals: locationTotals,
				AreaTotals:     areaTotals,
			},
		},
	}
}
