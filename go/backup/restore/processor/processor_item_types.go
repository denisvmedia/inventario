package processor

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// itemTypeResolver maps the item types of an archive onto the destination
// group's (#2802). Items store a type's slug, so restoring an item means
// finding the destination slug for its archived one: the same slug when the
// group has it, the type with the archived name when the group has that,
// otherwise a type the restore creates.
//
// It is created once per restore operation and caches each archived slug's
// destination. A dry run resolves nothing and writes nothing: items keep
// their archived slug, and the dry run reports what a real restore of those
// items would do apart from the types.
type itemTypeResolver struct {
	reg   registry.ItemTypeRegistry
	slugs map[models.CommodityType]models.CommodityType
	// fellBack holds the archived slugs whose type the group was too full
	// to create, so their items got the fallback type.
	fellBack map[models.CommodityType]bool
}

// itemTypeResolver returns the operation's resolver, creating it on first
// use. A nil registry (dry run, or a factory set without item types) makes
// every slug resolve to itself.
func (l *RestoreOperationProcessor) itemTypeResolver(ctx context.Context, options types.RestoreOptions) (*itemTypeResolver, error) {
	if l.itemTypes != nil {
		return l.itemTypes, nil
	}
	r := &itemTypeResolver{slugs: map[models.CommodityType]models.CommodityType{}, fellBack: map[models.CommodityType]bool{}}
	if !options.DryRun && l.factorySet != nil && l.factorySet.ItemTypeRegistryFactory != nil {
		reg, err := l.factorySet.ItemTypeRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to create item type registry", err)
		}
		if _, err := reg.EnsureBuiltins(ctx); err != nil {
			return nil, errxtrace.Wrap("failed to seed item types", err)
		}
		r.reg = reg
	}
	l.itemTypes = r
	return r, nil
}

// importArchived maps the item types an archive's manifest carries. A type
// the group lacks is created with its archived slug, name and icon; one that
// fails validation is counted as an error and its items fall back to
// resolve.
func (r *itemTypeResolver) importArchived(ctx context.Context, archived []types.INBItemType, stats *types.RestoreStats) error {
	if r.reg == nil {
		return nil
	}
	for _, a := range archived {
		slug := models.CommodityType(a.Slug)
		if slug == "" {
			continue
		}
		if _, done := r.slugs[slug]; done {
			continue
		}
		dest, err := r.find(ctx, slug, a.Name, a.Slug)
		if err != nil {
			return err
		}
		if dest != "" {
			r.slugs[slug] = dest
			continue
		}
		t := models.ItemType{Name: a.Name, Icon: a.Icon}
		if slug.IsValid() {
			t.Slug = slug
		}
		t.Normalize()
		if err := t.ValidateFields(ctx); err != nil {
			stats.ErrorCount++
			stats.Errors = append(stats.Errors, fmt.Sprintf("item type %s: %v", a.Slug, err))
			continue
		}
		created, full, err := r.create(ctx, t)
		if err != nil {
			return err
		}
		if full {
			r.fellBack[slug] = true
			recordGroupFull(stats, a.Slug, created)
		}
		r.slugs[slug] = created
	}
	return nil
}

// resolve returns the destination slug of an item's archived type, creating
// the type when the group has neither it nor a type of that name. This
// covers archives that carry no item types (older formats and the XML
// backup) and types importArchived could not create. A type it cannot
// create because the group is full is counted in stats once, as
// importArchived counts it; the cache keeps later items of that type from
// counting it again.
func (r *itemTypeResolver) resolve(ctx context.Context, slug models.CommodityType, stats *types.RestoreStats) (models.CommodityType, error) {
	if r.reg == nil || slug == "" {
		return slug, nil
	}
	if dest, ok := r.slugs[slug]; ok {
		return dest, nil
	}
	name := ""
	if !slug.IsBuiltin() {
		name = string(slug)
		if runes := []rune(name); len(runes) > models.MaxItemTypeNameRunes {
			name = string(runes[:models.MaxItemTypeNameRunes])
		}
	}
	dest, err := r.find(ctx, slug, name, string(slug))
	if err != nil {
		return "", err
	}
	if dest == "" {
		t := models.ItemType{Name: name}
		if slug.IsValid() {
			t.Slug = slug
		}
		t.Normalize()
		var full bool
		if dest, full, err = r.create(ctx, t); err != nil {
			return "", err
		}
		if full {
			r.fellBack[slug] = true
			recordGroupFull(stats, string(slug), dest)
		}
	}
	r.slugs[slug] = dest
	return dest, nil
}

// recordGroupFull counts an archived type the restore could not create
// because the group already keeps the most item types it may, and names the
// type its items get instead.
func recordGroupFull(stats *types.RestoreStats, archived string, fallback models.CommodityType) {
	if stats == nil {
		return
	}
	stats.ErrorCount++
	stats.Errors = append(stats.Errors, fmt.Sprintf(
		"item type %s: the group already has %d item types, the most it keeps; its items get the type %s",
		archived, models.MaxItemTypesPerGroup, fallback))
}

// find looks the type up by slug, then by name, and returns the slug of the
// match or "" when there is none.
func (r *itemTypeResolver) find(ctx context.Context, slug models.CommodityType, name, logID string) (models.CommodityType, error) {
	if t, err := r.reg.GetBySlug(ctx, slug); err == nil {
		return t.Slug, nil
	} else if !errors.Is(err, registry.ErrNotFound) {
		return "", errxtrace.Wrap("failed to look up item type", err, errx.Attrs("item_type", logID))
	}
	lookup := name
	if lookup == "" {
		if b, ok := models.BuiltinItemTypeFor(slug); ok {
			lookup = b.Name
		}
	}
	if lookup == "" {
		return "", nil
	}
	if t, err := r.reg.GetByName(ctx, lookup); err == nil {
		return t.Slug, nil
	} else if !errors.Is(err, registry.ErrNotFound) {
		return "", errxtrace.Wrap("failed to look up item type", err, errx.Attrs("item_type", logID))
	}
	return "", nil
}

// create creates t and returns its slug. When the group already has
// models.MaxItemTypesPerGroup types nothing is created: full is true, and
// the returned slug is the group's Other type, or its first type when Other
// was deleted, so the items still get a type the group has.
func (r *itemTypeResolver) create(ctx context.Context, t models.ItemType) (slug models.CommodityType, full bool, err error) {
	created, err := r.reg.Create(ctx, t)
	if errors.Is(err, registry.ErrItemTypeLimit) {
		fallback, err := r.fallback(ctx)
		return fallback, true, err
	}
	if err != nil {
		return "", false, errxtrace.Wrap("failed to create item type", err, errx.Attrs("item_type", string(t.Slug), "name", t.Name))
	}
	return created.Slug, false, nil
}

// fallback is the type an item gets when its own cannot be created.
func (r *itemTypeResolver) fallback(ctx context.Context) (models.CommodityType, error) {
	if other, err := r.reg.GetBySlug(ctx, models.CommodityTypeOther); err == nil {
		return other.Slug, nil
	} else if !errors.Is(err, registry.ErrNotFound) {
		return "", errxtrace.Wrap("failed to look up item type", err)
	}
	list, err := r.reg.List(ctx)
	if err != nil {
		return "", errxtrace.Wrap("failed to list item types", err)
	}
	if len(list) == 0 {
		return "", errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "ItemType"))
	}
	return list[0].Slug, nil
}
