package jsonapi_test

import (
	"context"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/jsonapi"
)

// Request length limits count characters, not UTF-8 bytes. "ж" takes two
// bytes, so a limit counted in bytes refuses a Russian text at half the
// length the form promises.
const twoByteLetter = "ж"

type requestLengthCase struct {
	name     string
	limit    int
	validate func(text string) error
}

func requestLengthCases() []requestLengthCase {
	ctx := context.Background()
	loan := func(mut func(*jsonapi.CommodityLoanRequestData)) error {
		d := jsonapi.CommodityLoanRequestData{BorrowerName: "Alice", LentAt: "2026-01-01"}
		mut(&d)
		return d.ValidateWithContext(ctx)
	}
	link := func(mut func(*jsonapi.SupplyLinkRequestData)) error {
		d := jsonapi.SupplyLinkRequestData{Label: "Filter", URL: "https://example.com/filter"}
		mut(&d)
		return d.ValidateWithContext(ctx)
	}
	schedule := func(mut func(*jsonapi.MaintenanceScheduleRequestData)) error {
		d := jsonapi.MaintenanceScheduleRequestData{Title: "Descale", IntervalDays: 90}
		mut(&d)
		return d.ValidateWithContext(ctx)
	}
	file := func(mut func(*jsonapi.FileUpdateRequestFileData)) error {
		d := jsonapi.FileUpdateRequestFileData{}
		mut(&d)
		return d.ValidateWithContext(ctx)
	}

	return []requestLengthCase{
		{name: "loan borrower name", limit: 200, validate: func(s string) error {
			return loan(func(d *jsonapi.CommodityLoanRequestData) { d.BorrowerName = s })
		}},
		{name: "loan borrower note", limit: 1000, validate: func(s string) error {
			return loan(func(d *jsonapi.CommodityLoanRequestData) { d.BorrowerNote = s })
		}},
		{name: "supply link label", limit: 200, validate: func(s string) error {
			return link(func(d *jsonapi.SupplyLinkRequestData) { d.Label = s })
		}},
		{name: "maintenance title", limit: 200, validate: func(s string) error {
			return schedule(func(d *jsonapi.MaintenanceScheduleRequestData) { d.Title = s })
		}},
		{name: "file description", limit: 1000, validate: func(s string) error {
			return file(func(d *jsonapi.FileUpdateRequestFileData) { d.Description = s })
		}},
	}
}

func TestRequestLengthLimits_AcceptTheLimitInTwoByteCharacters(t *testing.T) {
	for _, tc := range requestLengthCases() {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(tc.validate(strings.Repeat(twoByteLetter, tc.limit)), qt.IsNil)
		})
	}
}

func TestRequestLengthLimits_RefuseOneCharacterOverTheLimit(t *testing.T) {
	for _, tc := range requestLengthCases() {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(tc.validate(strings.Repeat(twoByteLetter, tc.limit+1)), qt.IsNotNil)
		})
	}
}
