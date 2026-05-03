package services

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ErrTagInUse is returned by DeleteTag when the tag still has commodity or
// file references and `force=false`. The handler maps it to 409 with the
// usage breakdown so the FE can surface the conflict.
//
// Aliased onto registry.ErrTagInUse — the canonical sentinel lives in
// the registry package so DeleteAtomic can return it directly from
// inside the lock-protected tx, and the apiserver / FE behavior stays
// unchanged for callers that compare against services.ErrTagInUse.
var ErrTagInUse = registry.ErrTagInUse

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
		// later via PATCH. Timestamps are stamped explicitly so the memory
		// backend (which has no DEFAULT CURRENT_TIMESTAMP) returns a usable
		// CreatedAt for recency ranking; on postgres the value mirrors
		// what the DEFAULT would have produced.
		now := time.Now()
		tag := models.Tag{
			Slug:      slug,
			Label:     defaultLabelFromSlug(slug),
			Color:     models.DefaultTagColor,
			CreatedAt: now,
			UpdatedAt: now,
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
//
// Order of `raw` is preserved (first-seen wins on duplicates) so the
// resulting JSONB array is deterministic — round-tripping the same
// commodity payload twice produces byte-identical responses, and noisy
// order-only diffs in tests / e2e snapshots disappear.
//
// Returns an empty slice (never nil) when nothing usable remains, so
// callers persist `[]` instead of `null` on the JSONB column.
func (s *TagService) NormalizeAndEnsureSlugs(ctx context.Context, raw []string) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	tags, err := s.EnsureTagsExist(ctx, raw)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(raw))
	seen := make(map[string]struct{}, len(raw))
	for _, r := range raw {
		slug := models.NormalizeTagSlug(r)
		if slug == "" {
			continue
		}
		if _, dup := seen[slug]; dup {
			continue
		}
		if _, ok := tags[slug]; !ok {
			// Defensive: should never happen if EnsureTagsExist succeeded.
			continue
		}
		seen[slug] = struct{}{}
		out = append(out, slug)
	}
	return out, nil
}

// RenameTag mutates the metadata of an existing tag and, when the slug
// changes, rewrites every JSONB reference on commodities + files in the
// same group. The whole operation runs through TagRegistry.RenameAtomic,
// which holds a per-(group, tag id) lock so two parallel renames of the
// same tag serialize cleanly — the second re-reads the row inside its
// lock and renames from whatever slug the first one settled on.
func (s *TagService) RenameTag(ctx context.Context, id, newLabel, newSlug string, newColor models.TagColor) (*models.Tag, error) {
	tagReg, err := s.factorySet.TagRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create tag registry", err)
	}
	return tagReg.RenameAtomic(ctx, id, newLabel, newSlug, newColor)
}

// DeleteTag removes a tag. When force=false and the tag has any reference
// in commodities/files, returns ErrTagInUse along with the usage breakdown.
// When force=true, references are stripped from JSONB arrays first.
// `force` mirrors the public ?force= query parameter — splitting this
// into two methods would just push the flag into the apiserver layer.
//
// The whole operation runs through TagRegistry.DeleteAtomic, which holds
// a per-(group, tag id) + per-(group, slug) lock so a concurrent
// commodity / file insert that would otherwise leak an orphan JSONB
// reference serializes against the delete via the same slug lock taken
// in the insert path.
//
//revive:disable-next-line:flag-parameter
func (s *TagService) DeleteTag(ctx context.Context, id string, force bool) (registry.TagUsage, error) {
	tagReg, err := s.factorySet.TagRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return registry.TagUsage{}, errxtrace.Wrap("failed to create tag registry", err)
	}
	return tagReg.DeleteAtomic(ctx, id, force)
}

// defaultLabelFromSlug produces a sensible display label for an
// auto-created tag: split on '-', Title-Case each word.
//
// Rune-aware so a future relaxation of NormalizeTagSlug to keep unicode
// letters (or a caller passing already-mixed-case input) doesn't panic on
// `p[:1]` slicing into the middle of a UTF-8 sequence.
func defaultLabelFromSlug(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToUpper(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}
