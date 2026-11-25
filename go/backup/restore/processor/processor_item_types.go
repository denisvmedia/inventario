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
}

// itemTypeResolver returns the operation's resolver, creating it on first
// use. A nil registry (dry run, or a factory set without item types) makes
// every slug resolve to itself.
func (l *RestoreOperationProcessor) itemTypeResolver(ctx context.Context, options types.RestoreOptions) (*itemTypeResolver, error) {
	if l.itemTypes != nil {
		return l.itemTypes, nil
	}
	r := &itemTypeResolver{slugs: map[models.CommodityType]models.CommodityType{}}
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
		created, err := r.create(ctx, t)
		if err != nil {
			return err
		}
		r.slugs[slug] = created
	}
	return nil
}

// resolve returns the destination slug of an item's archived type, creating
// the type when the group has neither it nor a type of that name. This
// covers archives that carry no item types (older formats and the XML
// backup) and types importArchived could not create.
func (r *itemTypeResolver) resolve(ctx context.Context, slug models.CommodityType) (models.CommodityType, error) {
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
		if dest, err = r.create(ctx, t); err != nil {
			return "", err
		}
	}
	r.slugs[slug] = dest
	return dest, nil
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

func (r *itemTypeResolver) create(ctx context.Context, t models.ItemType) (models.CommodityType, error) {
	created, err := r.reg.Create(ctx, t)
	if err != nil {
		return "", errxtrace.Wrap("failed to create item type", err, errx.Attrs("item_type", string(t.Slug), "name", t.Name))
	}
	return created.Slug, nil
}
