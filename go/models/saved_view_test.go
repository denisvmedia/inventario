package models_test

import (
	"context"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func validSavedView() models.SavedView {
	return models.SavedView{
		Name:         "Expiring warranties",
		Icon:         models.SavedViewIconShield,
		Entity:       models.SavedViewEntityCommodities,
		Query:        "warranty_status=expiring",
		QueryVersion: models.SavedViewQueryVersion,
		Layout:       models.SavedViewLayoutList,
	}
}

func TestSavedView_Validate_RejectsContextlessValidate(t *testing.T) {
	c := qt.New(t)
	c.Assert((&models.SavedView{}).Validate(), qt.Equals, models.ErrMustUseValidateWithContext)
}

func TestSavedView_ValidateWithContext_HappyPath(t *testing.T) {
	c := qt.New(t)
	for _, layout := range []models.SavedViewLayout{models.SavedViewLayoutDefault, models.SavedViewLayoutGrid, models.SavedViewLayoutList} {
		v := validSavedView()
		v.Layout = layout
		c.Assert(v.ValidateWithContext(context.Background()), qt.IsNil, qt.Commentf("layout %q", layout))
	}
	for _, icon := range models.ValidSavedViewIcons {
		v := validSavedView()
		v.Icon = icon
		c.Assert(v.ValidateWithContext(context.Background()), qt.IsNil, qt.Commentf("icon %q", icon))
	}
	v := validSavedView()
	v.Name = strings.Repeat("ž", models.MaxSavedViewNameRunes)
	v.Query = ""
	c.Assert(v.ValidateWithContext(context.Background()), qt.IsNil)
}

func TestSavedView_ValidateWithContext_UnhappyPath(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*models.SavedView)
		want string
	}{
		{"name empty", func(v *models.SavedView) { v.Name = "" }, "name"},
		{"name too long", func(v *models.SavedView) { v.Name = strings.Repeat("x", models.MaxSavedViewNameRunes+1) }, "name"},
		{"icon empty", func(v *models.SavedView) { v.Icon = "" }, "icon"},
		{"icon unknown", func(v *models.SavedView) { v.Icon = "rocket" }, "icon"},
		{"entity unknown", func(v *models.SavedView) { v.Entity = "files" }, "entity"},
		{"layout unknown", func(v *models.SavedView) { v.Layout = "cards" }, "layout"},
		{"query too long", func(v *models.SavedView) { v.Query = strings.Repeat("a", models.MaxSavedViewQueryBytes+1) }, "query"},
		{"query version missing", func(v *models.SavedView) { v.QueryVersion = 0 }, "query_version"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			v := validSavedView()
			tc.mut(&v)
			err := v.ValidateWithContext(context.Background())
			c.Assert(err, qt.IsNotNil)
			c.Assert(err.Error(), qt.Contains, tc.want)
		})
	}
}

func TestSavedView_VisibleTo(t *testing.T) {
	c := qt.New(t)
	v := validSavedView()
	v.CreatedByUserID = "alice"
	c.Assert(v.VisibleTo("alice"), qt.IsTrue)
	c.Assert(v.VisibleTo("bob"), qt.IsFalse)
	v.Shared = true
	c.Assert(v.VisibleTo("bob"), qt.IsTrue)
}
