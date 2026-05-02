package services

import (
	"context"
	"errors"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ErrTagInUse is returned by DeleteTag when the tag still has commodity or
// file references and `force=false`. The handler maps it to 409 with the
// usage breakdown so the FE can surface the conflict.
var ErrTagInUse = errors.New("tag is in use")

// TagService coordinates the tag entity with the JSONB associations on
// commodities + files. The registry handles the per-table mechanics; the
// service stitches them into the user-visible operations (auto-create on
// reference, rename-with-rewrite, force-delete-with-strip).
type TagService struct {
	factorySet *registry.FactorySet
}

func NewTagService(factorySet *registry.FactorySet) *TagService {
	return &TagService{factorySet: factorySet}
}

// EnsureTagsExist looks up each slug in the current group and provisions a
// new Tag row with DefaultTagColor for slugs that don't exist yet.
// Returns the canonical tags map keyed by slug.
//
// Slugs are normalized via models.NormalizeTagSlug; empty results after
// normalization are filtered out (callers typically log a warning when a
// user-typed string normalizes to nothing).
//
// The method is idempotent — calling it twice with the same input yields
// the same result. Concurrent calls with overlapping slugs may race on the
// (group_id, slug) unique index; the second writer's INSERT fails with a
// duplicate-key error and the caller should retry. Currently we surface
// the error rather than retrying — the autocomplete UI prevents most
// realistic races, and the issue body permits this (concurrency target is
// rename, not auto-create).
func (s *TagService) EnsureTagsExist(ctx context.Context, slugs []string) (map[string]*models.Tag, error) {
	tagReg, err := s.factorySet.TagRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create tag registry", err)
	}

	out := make(map[string]*models.Tag, len(slugs))
	seen := make(map[string]struct{}, len(slugs))

	for _, raw := range slugs {
		slug := models.NormalizeTagSlug(raw)
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}

		existing, err := tagReg.GetBySlug(ctx, slug)
		if err == nil && existing != nil {
			out[slug] = existing
			continue
		}
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			return nil, errxtrace.Wrap("failed to look up tag", err, errx.Attrs("slug", slug))
		}

		// Auto-create with default color and a label derived from the slug
		// (replace hyphens with spaces + Title-case). The user can rename
		// later via PATCH.
		tag := models.Tag{
			Slug:  slug,
			Label: defaultLabelFromSlug(slug),
			Color: models.DefaultTagColor,
		}
		created, err := tagReg.Create(ctx, tag)
		if err != nil {
			return nil, errxtrace.Wrap("failed to auto-create tag", err, errx.Attrs("slug", slug))
		}
		out[slug] = created
	}

	return out, nil
}

// NormalizeAndEnsureSlugs takes a user-supplied tag list (possibly raw
// labels), normalizes each into canonical slugs, ensures the underlying
// rows exist via EnsureTagsExist, and returns the deduplicated slug
// list ready to be persisted into JSONB.
func (s *TagService) NormalizeAndEnsureSlugs(ctx context.Context, raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	tags, err := s.EnsureTagsExist(ctx, raw)
	if err != nil {
		return nil, err
	}
	slugs := make([]string, 0, len(tags))
	for slug := range tags {
		slugs = append(slugs, slug)
	}
	return slugs, nil
}

// RenameTag mutates the metadata of an existing tag and, when the slug
// changes, rewrites every JSONB reference on commodities + files in the
// same group. The rewrite happens in a single registry transaction; the
// metadata Update is a separate transaction immediately afterwards. A
// failure between the two leaves a small inconsistency window where rows
// have already been rewritten but the tag row still carries the old slug;
// we accept that risk for now (issue's concurrency target is two parallel
// renames, not crash recovery).
func (s *TagService) RenameTag(ctx context.Context, id, newLabel, newSlug string, newColor models.TagColor) (*models.Tag, error) {
	tagReg, err := s.factorySet.TagRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create tag registry", err)
	}

	current, err := tagReg.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to look up tag", err)
	}

	updated := *current
	if strings.TrimSpace(newLabel) != "" {
		updated.Label = newLabel
	}
	if newColor != "" {
		updated.Color = newColor
	}

	slugChanged := newSlug != "" && newSlug != current.Slug
	if slugChanged {
		updated.Slug = newSlug
		// Refuse pre-emptively if a different tag already owns the new slug
		// — relying on the unique index to fail later would still work but
		// produces a worse error message.
		clash, err := tagReg.GetBySlug(ctx, newSlug)
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			return nil, errxtrace.Wrap("failed to check slug availability", err)
		}
		if clash != nil && clash.ID != current.ID {
			return nil, errxtrace.Wrap(
				"target slug is already used by another tag",
				registry.ErrAlreadyExists,
				errx.Attrs("slug", newSlug),
			)
		}

		if _, _, err := tagReg.RewriteSlugReferences(ctx, current.Slug, newSlug); err != nil {
			return nil, errxtrace.Wrap("failed to rewrite slug references", err)
		}
	}

	final, err := tagReg.Update(ctx, updated)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update tag", err)
	}
	return final, nil
}

// DeleteTag removes a tag. When force=false and the tag has any reference
// in commodities/files, returns ErrTagInUse along with the usage breakdown.
// When force=true, references are stripped from JSONB arrays first.
// `force` mirrors the public ?force= query parameter — splitting this
// into two methods would just push the flag into the apiserver layer.
//
//revive:disable-next-line:flag-parameter
func (s *TagService) DeleteTag(ctx context.Context, id string, force bool) (registry.TagUsage, error) {
	tagReg, err := s.factorySet.TagRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return registry.TagUsage{}, errxtrace.Wrap("failed to create tag registry", err)
	}

	current, err := tagReg.Get(ctx, id)
	if err != nil {
		return registry.TagUsage{}, errxtrace.Wrap("failed to look up tag", err)
	}

	usage, err := tagReg.GetUsage(ctx, current.Slug)
	if err != nil {
		return registry.TagUsage{}, errxtrace.Wrap("failed to compute tag usage", err)
	}
	if usage.Commodities+usage.Files > 0 && !force {
		return usage, ErrTagInUse
	}
	if usage.Commodities+usage.Files > 0 {
		if _, _, err := tagReg.StripSlugReferences(ctx, current.Slug); err != nil {
			return usage, errxtrace.Wrap("failed to strip slug references", err)
		}
	}

	if err := tagReg.Delete(ctx, id); err != nil {
		return usage, errxtrace.Wrap("failed to delete tag", err)
	}
	return usage, nil
}

// defaultLabelFromSlug produces a sensible display label for an
// auto-created tag: split on '-', Title-Case each word.
func defaultLabelFromSlug(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}
