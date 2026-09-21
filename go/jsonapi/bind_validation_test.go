package jsonapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/jsonapi"
)

// A JSON:API write payload nests its user-supplied fields two levels deep:
// data → attributes. `validation.Field` cannot reach an attributes struct
// whose validator has a pointer receiver — the library dereferences the
// field pointer first — so every rule on those fields has to be exercised
// through Bind, not by calling the attribute validator directly.
func TestBind_ValidatesAttributes(t *testing.T) {
	tests := []struct {
		name    string
		target  func() interface{ Bind(*http.Request) error }
		payload string
		wantErr bool
	}{
		{
			name:    "tag create: valid",
			target:  func() interface{ Bind(*http.Request) error } { return &jsonapi.TagRequest{} },
			payload: `{"data":{"type":"tags","attributes":{"kind":"commodity","slug":"garage","label":"Garage","color":"muted"}}}`,
		},
		{
			name:    "tag create: kind off the closed set",
			target:  func() interface{ Bind(*http.Request) error } { return &jsonapi.TagRequest{} },
			payload: `{"data":{"type":"tags","attributes":{"kind":"everything","slug":"garage","label":"Garage","color":"muted"}}}`,
			wantErr: true,
		},
		{
			name:    "tag create: slug not kebab-cased",
			target:  func() interface{ Bind(*http.Request) error } { return &jsonapi.TagRequest{} },
			payload: `{"data":{"type":"tags","attributes":{"kind":"commodity","slug":"Not A Slug","label":"Garage","color":"muted"}}}`,
			wantErr: true,
		},
		{
			name:    "tag create: color off the curated set",
			target:  func() interface{ Bind(*http.Request) error } { return &jsonapi.TagRequest{} },
			payload: `{"data":{"type":"tags","attributes":{"kind":"commodity","slug":"garage","label":"Garage","color":"chartreuse"}}}`,
			wantErr: true,
		},
		{
			name:    "tag patch: valid",
			target:  func() interface{ Bind(*http.Request) error } { return &jsonapi.TagUpdateRequest{} },
			payload: `{"data":{"id":"t1","type":"tags","attributes":{"slug":"garage"}}}`,
		},
		{
			name:    "tag patch: slug not kebab-cased",
			target:  func() interface{ Bind(*http.Request) error } { return &jsonapi.TagUpdateRequest{} },
			payload: `{"data":{"id":"t1","type":"tags","attributes":{"slug":"Not A Slug"}}}`,
			wantErr: true,
		},
		{
			name:   "supply link create: valid",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.SupplyLinkRequest{} },
			payload: `{"data":{"type":"commodity_supply_links","attributes":` +
				`{"label":"Spares","url":"https://example.com/spares"}}}`,
		},
		{
			name:   "supply link create: label missing",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.SupplyLinkRequest{} },
			payload: `{"data":{"type":"commodity_supply_links","attributes":` +
				`{"label":"","url":"https://example.com/spares"}}}`,
			wantErr: true,
		},
		{
			name:   "maintenance create: valid",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.MaintenanceScheduleRequest{} },
			payload: `{"data":{"type":"maintenance_schedules","attributes":` +
				`{"title":"Oil change","interval_days":90,"next_due_at":"2027-01-31"}}}`,
		},
		{
			name:   "maintenance create: interval below the floor",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.MaintenanceScheduleRequest{} },
			payload: `{"data":{"type":"maintenance_schedules","attributes":` +
				`{"title":"Oil change","interval_days":-3,"next_due_at":"2027-01-31"}}}`,
			wantErr: true,
		},
		{
			// next_due_at is optional, so an omitted one must stay valid
			// now that the field is actually checked.
			name:   "maintenance create: next_due_at omitted",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.MaintenanceScheduleRequest{} },
			payload: `{"data":{"type":"maintenance_schedules","attributes":` +
				`{"title":"Oil change","interval_days":90}}}`,
		},

		// The patch surfaces carry a second defect of the same kind. Their
		// rules were written as validation.Field(x.Field, ...) rather than
		// validation.Field(&x.Field, ...), which the library answers with
		// "field #N cannot be found in the struct" — an internal error, not
		// a validation one. Dead code hid it; every patch below reaches it.
		{
			name:    "loan patch: note only",
			target:  func() interface{ Bind(*http.Request) error } { return &jsonapi.CommodityLoanUpdateRequest{} },
			payload: `{"data":{"id":"l1","type":"commodity_loans","attributes":{"borrower_note":"new note"}}}`,
		},
		{
			name:   "loan patch: borrower name over the limit",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.CommodityLoanUpdateRequest{} },
			payload: `{"data":{"id":"l1","type":"commodity_loans","attributes":{"borrower_name":"` +
				strings.Repeat("x", 201) + `"}}}`,
			wantErr: true,
		},
		{
			name:    "loan patch: clearing due_back_at stays valid",
			target:  func() interface{ Bind(*http.Request) error } { return &jsonapi.CommodityLoanUpdateRequest{} },
			payload: `{"data":{"id":"l1","type":"commodity_loans","attributes":{"due_back_at":null}}}`,
		},
		{
			name:   "supply link patch: label only",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.SupplyLinkUpdateRequest{} },
			payload: `{"data":{"id":"s1","type":"commodity_supply_links",` +
				`"attributes":{"label":"Renamed"}}}`,
		},
		{
			name:   "supply link patch: url over the limit",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.SupplyLinkUpdateRequest{} },
			payload: `{"data":{"id":"s1","type":"commodity_supply_links","attributes":{"url":"` +
				strings.Repeat("u", 2049) + `"}}}`,
			wantErr: true,
		},
		{
			name:   "service patch: provider only",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.CommodityServiceUpdateRequest{} },
			payload: `{"data":{"id":"v1","type":"commodity_services",` +
				`"attributes":{"provider_name":"Acme"}}}`,
		},
		{
			name:   "service patch: reason over the limit",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.CommodityServiceUpdateRequest{} },
			payload: `{"data":{"id":"v1","type":"commodity_services","attributes":{"reason":"` +
				strings.Repeat("r", 1001) + `"}}}`,
			wantErr: true,
		},
		{
			name:   "maintenance patch: title only",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.MaintenanceScheduleUpdateRequest{} },
			payload: `{"data":{"id":"m1","type":"maintenance_schedules",` +
				`"attributes":{"title":"Oil change"}}}`,
		},
		{
			name:   "maintenance patch: interval above the ceiling",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.MaintenanceScheduleUpdateRequest{} },
			payload: `{"data":{"id":"m1","type":"maintenance_schedules",` +
				`"attributes":{"interval_days":36501}}}`,
			wantErr: true,
		},
		{
			// Recorded rather than asserted the other way: the library
			// skips a zero value for every rule but Required, so Min(1)
			// cannot see this one. The create path is covered because it
			// carries Required; the patch path has nothing to lean on.
			// Tracked separately — this test is about the rules running
			// at all, not about how strong each one is.
			name:   "maintenance patch: interval of zero slips past Min(1)",
			target: func() interface{ Bind(*http.Request) error } { return &jsonapi.MaintenanceScheduleUpdateRequest{} },
			payload: `{"data":{"id":"m1","type":"maintenance_schedules",` +
				`"attributes":{"interval_days":0}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			target := tt.target()
			c.Assert(json.Unmarshal([]byte(tt.payload), target), qt.IsNil)

			req, err := http.NewRequest("POST", "/", bytes.NewReader([]byte(tt.payload)))
			c.Assert(err, qt.IsNil)

			err = target.Bind(req)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil, qt.Commentf("payload was accepted: %s", tt.payload))
				return
			}
			c.Assert(err, qt.IsNil)
		})
	}
}
