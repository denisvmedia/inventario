package models_test

import (
	"context"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

// Length limits on text count characters, not UTF-8 bytes. "ж" takes two
// bytes, so a limit counted in bytes refuses a Russian text at half the
// length the form promises.
const twoByteLetter = "ж"

type lengthLimitCase struct {
	name     string
	limit    int
	validate func(text string) error
}

func lengthLimitCases() []lengthLimitCase {
	validSlug := "abcdefghijklmnopqrstuv"
	group := func(mut func(*models.LocationGroup)) error {
		g := models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
			Slug:                validSlug,
			Name:                "My Group",
			Status:              models.LocationGroupStatusActive,
			CreatedBy:           "user-1",
			GroupCurrency:       models.Currency("USD"),
		}
		mut(&g)
		return g.ValidateWithContext(context.Background())
	}
	link := func(mut func(*models.SupplyLink)) error {
		l := models.SupplyLink{
			CommodityID: "commodity-1",
			Label:       "Water filter",
			URL:         "https://example.com/water-filter",
		}
		mut(&l)
		return l.ValidateWithContext(context.Background())
	}

	return []lengthLimitCase{
		{name: "group name", limit: 100, validate: func(s string) error {
			return group(func(g *models.LocationGroup) { g.Name = s })
		}},
		{name: "group description", limit: 200, validate: func(s string) error {
			return group(func(g *models.LocationGroup) { g.Description = s })
		}},
		{name: "supply link label", limit: 200, validate: func(s string) error {
			return link(func(l *models.SupplyLink) { l.Label = s })
		}},
		{name: "supply link notes", limit: 1000, validate: func(s string) error {
			return link(func(l *models.SupplyLink) { l.Notes = s })
		}},
	}
}

func TestLengthLimits_AcceptTheLimitInTwoByteCharacters(t *testing.T) {
	for _, tc := range lengthLimitCases() {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(tc.validate(strings.Repeat(twoByteLetter, tc.limit)), qt.IsNil)
		})
	}
}

func TestLengthLimits_RefuseOneCharacterOverTheLimit(t *testing.T) {
	for _, tc := range lengthLimitCases() {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(tc.validate(strings.Repeat(twoByteLetter, tc.limit+1)), qt.IsNotNil)
		})
	}
}
