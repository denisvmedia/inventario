package models_test

import (
	"context"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func TestTagColor_IsValid(t *testing.T) {
	c := qt.New(t)

	for _, color := range models.ValidTagColors {
		c.Run(string(color), func(c *qt.C) {
			c.Assert(color.IsValid(), qt.IsTrue)
		})
	}

	c.Run("rejects unknown color", func(c *qt.C) {
		c.Assert(models.TagColor("rainbow").IsValid(), qt.IsFalse)
	})
	c.Run("rejects empty color", func(c *qt.C) {
		c.Assert(models.TagColor("").IsValid(), qt.IsFalse)
	})
}

func TestTagColor_Validate(t *testing.T) {
	c := qt.New(t)

	c.Assert(models.TagColorAmber.Validate(), qt.IsNil)
	c.Assert(models.TagColor("rainbow").Validate(), qt.IsNotNil)
}

// tagSlugCases is the shared table for the slug normalizer.
// frontend/src/features/tags/__tests__/schemas.test.ts runs normaliseSlug
// against the same inputs and outputs; keep the two tables identical.
var tagSlugCases = []struct {
	name string
	in   string
	out  string
}{
	{"plain", "kitchen", "kitchen"},
	{"capitalized", "Kitchen", "kitchen"},
	{"surrounding spaces", "  kitchen  ", "kitchen"},
	{"space", "front office", "front-office"},
	{"underscore", "FRONT_OFFICE", "front-office"},
	{"dash run", "front---office", "front-office"},
	{"punctuation run", "front!@#office", "front-office"},
	{"leading and trailing dashes", "-leading-and-trailing-", "leading-and-trailing"},
	{"empty", "", ""},
	{"spaces only", "   ", ""},
	{"punctuation only", "###", ""},
	{"latin with umlaut", "unicode-ücase", "unicode-ücase"},
	{"cyrillic", "кухня", "кухня"},
	{"cyrillic capitalized", "Кухня", "кухня"},
	{"cyrillic two words", "на продажу", "на-продажу"},
	{"cjk", "書類", "書類"},
	{"czech", "Kuchyně", "kuchyně"},
	{"czech leading caron", "Čaj", "čaj"},
	{"french leading accent", "électronique", "électronique"},
	{"mixed scripts", "USB кабель", "usb-кабель"},
	{"mixed scripts, second word differs", "USB хаб", "usb-хаб"},
	{"decomposed input", "Kuchyne\u030c", "kuchyně"},
	{"greek final sigma", "ΟΔΟΣ", "οδος"},
	{"devanagari vowel signs", "हिन्दी", "हिन्दी"},
	{"cjk with digits", "東京 2024", "東京-2024"},
	{"superscript digit", "m²", "m²"},
	{"emoji dropped", "🍕 pizza", "pizza"},
	{"stray leading mark", "\u0301abc", "abc"},
	{"dotted capital I", "İstanbul", "i\u0307stanbul"},
}

func TestNormalizeTagSlug(t *testing.T) {
	for _, tt := range tagSlugCases {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(models.NormalizeTagSlug(tt.in), qt.Equals, tt.out)
		})
	}
}

// A slug that comes out of the normalizer must be accepted by the
// validator and must survive a second pass unchanged; otherwise a tag typed
// in the item form could never be renamed to its own slug on the Tags page.
func TestNormalizeTagSlug_OutputIsCanonical(t *testing.T) {
	for _, tt := range tagSlugCases {
		if tt.out == "" {
			continue
		}
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(models.NormalizeTagSlug(tt.out), qt.Equals, tt.out)
			c.Assert(models.IsValidTagSlug(tt.out), qt.IsTrue)
		})
	}
}

func TestNormalizeTagSlug_DecomposedAndComposedInputAgree(t *testing.T) {
	c := qt.New(t)

	composed := "Kuchyn\u011b \u010daj"     // ě and č as single code points
	decomposed := "Kuchyne\u030c c\u030caj" // e and c followed by a combining caron
	c.Assert(models.NormalizeTagSlug(decomposed), qt.Equals, models.NormalizeTagSlug(composed))
	c.Assert(models.NormalizeTagSlug(composed), qt.Equals, "kuchyn\u011b-\u010daj")
}

func TestIsValidTagSlug(t *testing.T) {
	c := qt.New(t)

	valid := []string{
		"kitchen", "front-office", "tag-1", "a", "a-1-2",
		"кухня", "на-продажу", "書類", "kuchyně", "usb-кабель", "हिन्दी", "οδος",
	}
	invalid := []string{
		"", "Kitchen", "front_office", "front--office", "-front", "front-", "front office",
		"Кухня",         // not lowercase
		"kuchyne\u030c", // not NFC
		"\u0301abc",     // starts with a mark
		"pizza-🍕",       // emoji
	}

	for _, s := range valid {
		c.Run("valid:"+s, func(c *qt.C) {
			c.Assert(models.IsValidTagSlug(s), qt.IsTrue)
		})
	}
	for _, s := range invalid {
		c.Run("invalid:"+s, func(c *qt.C) {
			c.Assert(models.IsValidTagSlug(s), qt.IsFalse)
		})
	}
}

func TestTag_ValidateWithContext(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		tag     models.Tag
		wantErr bool
	}{
		{
			name: "valid tag",
			tag: models.Tag{
				Kind:  models.TagKindCommodity,
				Slug:  "kitchen",
				Label: "Kitchen",
				Color: models.TagColorAmber,
			},
			wantErr: false,
		},
		{
			name: "valid file tag",
			tag: models.Tag{
				Kind:  models.TagKindFile,
				Slug:  "invoice",
				Label: "Invoice",
				Color: models.TagColorBlue,
			},
			wantErr: false,
		},
		{
			name: "missing kind",
			tag: models.Tag{
				Slug:  "kitchen",
				Label: "Kitchen",
				Color: models.TagColorAmber,
			},
			wantErr: true,
		},
		{
			name: "invalid kind",
			tag: models.Tag{
				Kind:  models.TagKind("widget"),
				Slug:  "kitchen",
				Label: "Kitchen",
				Color: models.TagColorAmber,
			},
			wantErr: true,
		},
		{
			name: "missing slug",
			tag: models.Tag{
				Kind:  models.TagKindCommodity,
				Label: "Kitchen",
				Color: models.TagColorAmber,
			},
			wantErr: true,
		},
		{
			name: "invalid slug shape",
			tag: models.Tag{
				Kind:  models.TagKindCommodity,
				Slug:  "Kitchen Office",
				Label: "Kitchen Office",
				Color: models.TagColorAmber,
			},
			wantErr: true,
		},
		{
			name: "missing label",
			tag: models.Tag{
				Kind:  models.TagKindCommodity,
				Slug:  "kitchen",
				Color: models.TagColorAmber,
			},
			wantErr: true,
		},
		{
			name: "label too long",
			tag: models.Tag{
				Kind:  models.TagKindCommodity,
				Slug:  "kitchen",
				Label: stringOfLen(65),
				Color: models.TagColorAmber,
			},
			wantErr: true,
		},
		{
			name: "cyrillic slug and label",
			tag: models.Tag{
				Kind:  models.TagKindCommodity,
				Slug:  "кухня",
				Label: "Кухня",
				Color: models.TagColorAmber,
			},
			wantErr: false,
		},
		{
			name: "64-character cyrillic label fits",
			tag: models.Tag{
				Kind:  models.TagKindCommodity,
				Slug:  "кухня",
				Label: strings.Repeat("ж", 64),
				Color: models.TagColorAmber,
			},
			wantErr: false,
		},
		{
			name: "65-character cyrillic label is too long",
			tag: models.Tag{
				Kind:  models.TagKindCommodity,
				Slug:  "кухня",
				Label: strings.Repeat("ж", 65),
				Color: models.TagColorAmber,
			},
			wantErr: true,
		},
		{
			name: "64-character cyrillic slug fits",
			tag: models.Tag{
				Kind:  models.TagKindCommodity,
				Slug:  strings.Repeat("ж", 64),
				Label: "Кухня",
				Color: models.TagColorAmber,
			},
			wantErr: false,
		},
		{
			name: "65-character slug is too long",
			tag: models.Tag{
				Kind:  models.TagKindCommodity,
				Slug:  strings.Repeat("ж", 65),
				Label: "Кухня",
				Color: models.TagColorAmber,
			},
			wantErr: true,
		},
		{
			name: "missing color",
			tag: models.Tag{
				Kind:  models.TagKindCommodity,
				Slug:  "kitchen",
				Label: "Kitchen",
			},
			wantErr: true,
		},
		{
			name: "invalid color",
			tag: models.Tag{
				Kind:  models.TagKindCommodity,
				Slug:  "kitchen",
				Label: "Kitchen",
				Color: models.TagColor("rainbow"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		c.Run(tt.name, func(c *qt.C) {
			err := tt.tag.ValidateWithContext(ctx)
			if tt.wantErr {
				c.Assert(err, qt.IsNotNil)
			} else {
				c.Assert(err, qt.IsNil)
			}
		})
	}
}

func TestTag_TenantGroupAwareIDable(t *testing.T) {
	c := qt.New(t)

	var tag models.Tag
	var _ models.IDable = &tag
	var _ models.TenantAware = &tag
	var _ models.GroupAware = &tag
	var _ models.CreatedByUserAware = &tag
	var _ models.TenantGroupAwareIDable = &tag

	c.Assert(&tag, qt.IsNotNil)
}

func stringOfLen(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return string(b)
}
