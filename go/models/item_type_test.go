package models_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func TestItemTypeSlugFromName(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name string
		want models.CommodityType
	}{
		{"Vehicle", "vehicle"},
		{"Power tools", "power-tools"},
		{"  Board   games!  ", "board-games"},
		{"Автомобиль", "автомобиль"},
		{"Oblečení", "oblečení"},
		{"###", ""},
		{strings.Repeat("ab ", 40), models.CommodityType(strings.Repeat("ab-", 21) + "a")},
		{strings.Repeat("abc ", 20), models.CommodityType(strings.TrimSuffix(strings.Repeat("abc-", 16), "-"))},
	}
	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			got := models.ItemTypeSlugFromName(tt.name)
			c.Assert(got, qt.Equals, tt.want)
			if got != "" {
				c.Assert(got.IsValid(), qt.IsTrue)
			}
		})
	}
}

func TestItemTypeNameKey(t *testing.T) {
	c := qt.New(t)

	c.Assert(models.ItemTypeNameKey(" Vehicle "), qt.Equals, "vehicle")
	c.Assert(models.ItemTypeNameKey("Board   GAMES"), qt.Equals, "board games")
	c.Assert(models.ItemTypeNameKey("Straße"), qt.Equals, models.ItemTypeNameKey("STRASSE"))
}

func TestItemType_DisplayNameAndNormalize(t *testing.T) {
	c := qt.New(t)

	builtin := models.NewBuiltinItemType(models.BuiltinItemTypes[0])
	c.Assert(builtin.Slug, qt.Equals, models.CommodityTypeWhiteGoods)
	c.Assert(builtin.Name, qt.Equals, "")
	c.Assert(builtin.Builtin(), qt.IsTrue)
	c.Assert(builtin.DisplayName(), qt.Equals, "Appliance")
	c.Assert(builtin.NameKey, qt.Equals, "appliance")
	c.Assert(builtin.Icon, qt.Equals, "refrigerator")

	renamed := builtin
	renamed.Name = "  Kitchen   appliance "
	renamed.Normalize()
	c.Assert(renamed.Name, qt.Equals, "Kitchen appliance")
	c.Assert(renamed.DisplayName(), qt.Equals, "Kitchen appliance")
	c.Assert(renamed.NameKey, qt.Equals, "kitchen appliance")

	custom := models.ItemType{Slug: "vehicle", Name: "Vehicle"}
	custom.Normalize()
	c.Assert(custom.Builtin(), qt.IsFalse)
	c.Assert(custom.NameKey, qt.Equals, "vehicle")
}

func TestItemType_ValidateFields(t *testing.T) {
	c := qt.New(t)

	tests := []struct {
		name    string
		in      models.ItemType
		wantErr string
	}{
		{name: "custom with name and icon", in: models.ItemType{Slug: "vehicle", Name: "Vehicle", Icon: "car"}},
		{name: "custom without icon", in: models.ItemType{Name: "Vehicle"}},
		{name: "built-in keeps its default name", in: models.ItemType{Slug: models.CommodityTypeOther}},
		{name: "custom needs a name", in: models.ItemType{Slug: "vehicle"}, wantErr: "name: cannot be blank."},
		{name: "name too long", in: models.ItemType{Name: strings.Repeat("я", models.MaxItemTypeNameRunes+1)}, wantErr: "name: the length must be no more than 50."},
		{name: "name at the limit in runes", in: models.ItemType{Name: strings.Repeat("я", models.MaxItemTypeNameRunes)}},
		{name: "icon with upper case", in: models.ItemType{Name: "Vehicle", Icon: "Car"}, wantErr: "icon: must be in a valid format."},
		{name: "icon with a space", in: models.ItemType{Name: "Vehicle", Icon: "washing machine"}, wantErr: "icon: must be in a valid format."},
		{name: "icon with hyphens", in: models.ItemType{Name: "Vehicle", Icon: "washing-machine"}},
	}
	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			in := tt.in
			in.Normalize()
			err := in.ValidateFields(c.Context())
			if tt.wantErr == "" {
				c.Assert(err, qt.IsNil)
				return
			}
			c.Assert(err, qt.ErrorMatches, tt.wantErr)
		})
	}
}
