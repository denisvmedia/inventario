package registry

import (
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
)

// itemTypeFallbackSlug is the slug base of a type whose name has no letter
// or digit to make one from.
const itemTypeFallbackSlug = "type"

// maxItemTypeSlugAttempts bounds the search for a free slug. A group would
// need that many types with the same name base to exhaust it.
const maxItemTypeSlugAttempts = 1000

// ChooseItemTypeSlug sets t.Slug to the first free slug made from t.Name:
// the name's slug, then the same with -2, -3, ... appended, skipping the
// built-in slugs, which only the built-in types carry. taken reports whether
// a slug is used in t's group. Both backends call it under their group lock.
func ChooseItemTypeSlug(t *models.ItemType, taken func(models.CommodityType) (bool, error)) error {
	base := string(models.ItemTypeSlugFromName(t.Name))
	if base == "" {
		base = itemTypeFallbackSlug
	}
	for i := 1; i <= maxItemTypeSlugAttempts; i++ {
		candidate := itemTypeSlugCandidate(base, i)
		if candidate.IsBuiltin() {
			continue
		}
		isTaken, err := taken(candidate)
		if err != nil {
			return err
		}
		if !isTaken {
			t.Slug = candidate
			return nil
		}
	}
	return errxtrace.Classify(ErrAlreadyExists, errx.Attrs("slug", base))
}

// itemTypeSlugCandidate returns base for the first attempt and base-n for
// the n-th, with base cut so the result stays within the slug limit.
func itemTypeSlugCandidate(base string, n int) models.CommodityType {
	if n == 1 {
		return models.CommodityType(base)
	}
	suffix := "-" + strconv.Itoa(n)
	room := models.MaxItemTypeSlugRunes - utf8.RuneCountInString(suffix)
	if utf8.RuneCountInString(base) > room {
		base = strings.TrimRight(string([]rune(base)[:room]), "-")
	}
	return models.CommodityType(base + suffix)
}

// builtinItemTypeOrder is a built-in slug's position in
// models.BuiltinItemTypes.
var builtinItemTypeOrder = func() map[models.CommodityType]int {
	m := make(map[models.CommodityType]int, len(models.BuiltinItemTypes))
	for i, b := range models.BuiltinItemTypes {
		m[b.Slug] = i
	}
	return m
}()

// SortItemTypes orders item types the way both backends list them: the
// built-in types first, in their seed order, then the others by name key,
// name and ID. The frontend sorts by the name it shows instead; this order
// keeps the API and the AI scan prompt stable.
func SortItemTypes(types []*models.ItemType) {
	slices.SortStableFunc(types, func(a, b *models.ItemType) int {
		ai, aBuiltin := builtinItemTypeOrder[a.Slug]
		bi, bBuiltin := builtinItemTypeOrder[b.Slug]
		switch {
		case aBuiltin && bBuiltin:
			if ai != bi {
				return ai - bi
			}
		case aBuiltin:
			return -1
		case bBuiltin:
			return 1
		}
		if c := strings.Compare(a.NameKey, b.NameKey); c != 0 {
			return c
		}
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
}
