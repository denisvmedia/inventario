package models_test

import (
	"context"
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func TestGroupSection_Validate(t *testing.T) {
	for _, section := range models.AllGroupSections() {
		t.Run(string(section), func(t *testing.T) {
			c := qt.New(t)
			c.Assert(section.Validate(), qt.IsNil)
		})
	}
	for _, section := range []models.GroupSection{"", "Warranties", "files", "loan"} {
		t.Run("refuses "+string(section), func(t *testing.T) {
			c := qt.New(t)
			c.Assert(section.Validate(), qt.ErrorMatches, "must be one of: warranties, maintenance, loans, upcoming")
		})
	}
}

func TestGroupSections_Validate(t *testing.T) {
	c := qt.New(t)

	c.Assert(models.GroupSections(nil).Validate(), qt.IsNil)
	c.Assert(models.GroupSections{models.GroupSectionLoans, models.GroupSectionLoans}.Validate(), qt.IsNil)
	c.Assert(models.GroupSections{models.GroupSectionLoans, "tags"}.Validate(), qt.IsNotNil)
}

func TestGroupSections_Normalize(t *testing.T) {
	tests := []struct {
		name string
		in   models.GroupSections
		want models.GroupSections
	}{
		{name: "nil", in: nil, want: models.GroupSections{}},
		{
			name: "drops duplicates and keeps the settings order",
			in: models.GroupSections{
				models.GroupSectionUpcoming, models.GroupSectionWarranties,
				models.GroupSectionUpcoming, models.GroupSectionWarranties,
			},
			want: models.GroupSections{models.GroupSectionWarranties, models.GroupSectionUpcoming},
		},
		{
			name: "drops unknown sections",
			in:   models.GroupSections{"tags", models.GroupSectionMaintenance},
			want: models.GroupSections{models.GroupSectionMaintenance},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			got := tt.in.Normalize()
			c.Assert(got, qt.IsNotNil)
			c.Assert(got, qt.DeepEquals, tt.want)
		})
	}
}

func TestGroupSections_EncodesNilAsEmptyList(t *testing.T) {
	c := qt.New(t)

	raw, err := json.Marshal(models.LocationGroup{})
	c.Assert(err, qt.IsNil)
	var decoded map[string]any
	c.Assert(json.Unmarshal(raw, &decoded), qt.IsNil)
	c.Assert(decoded["hidden_sections"], qt.DeepEquals, []any{})

	value, err := models.GroupSections(nil).Value()
	c.Assert(err, qt.IsNil)
	c.Assert(value, qt.DeepEquals, []byte("[]"))
}

func TestGroupSections_Scan(t *testing.T) {
	tests := []struct {
		name string
		src  any
		want models.GroupSections
	}{
		{name: "bytes", src: []byte(`["loans","upcoming"]`), want: models.GroupSections{models.GroupSectionLoans, models.GroupSectionUpcoming}},
		{name: "string", src: `["warranties"]`, want: models.GroupSections{models.GroupSectionWarranties}},
		{name: "empty list", src: []byte(`[]`), want: models.GroupSections{}},
		{name: "json null", src: []byte(`null`), want: models.GroupSections{}},
		{name: "sql null", src: nil, want: models.GroupSections{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			var got models.GroupSections
			c.Assert(got.Scan(tt.src), qt.IsNil)
			c.Assert(got, qt.DeepEquals, tt.want)
		})
	}

	t.Run("refuses another type", func(t *testing.T) {
		c := qt.New(t)
		var got models.GroupSections
		c.Assert(got.Scan(42), qt.ErrorMatches, "cannot scan type int into GroupSections")
	})
}

func TestLocationGroup_HiddenSections(t *testing.T) {
	c := qt.New(t)
	group := models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
		Slug:                "abcdefghijklmnopqrstuv",
		Name:                "Collection",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           "user-1",
		GroupCurrency:       models.Currency("USD"),
		HiddenSections:      models.GroupSections{models.GroupSectionWarranties},
	}

	c.Assert(group.ValidateWithContext(context.Background()), qt.IsNil)
	c.Assert(group.HidesSection(models.GroupSectionWarranties), qt.IsTrue)
	c.Assert(group.HidesSection(models.GroupSectionLoans), qt.IsFalse)

	group.HiddenSections = models.GroupSections{"tags"}
	c.Assert(group.ValidateWithContext(context.Background()), qt.ErrorMatches, "hidden_sections: must be one of.*")
}
