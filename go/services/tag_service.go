package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"
	"unicode"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"golang.org/x/text/unicode/norm"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
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

// tagLabelSource says where EnsureTagsExist takes the label of a tag it has
// to create.
type tagLabelSource int

const (
	// labelFromInput keeps the text the caller passed in, trimmed and in
	// NFC: what the user typed in a tag field.
	labelFromInput tagLabelSource = iota
	// labelFromSlugShapedInput title-cases a reference that is already a
	// canonical slug ("new-tag-a" becomes "New Tag A") and keeps any other
	// reference as typed. Backups store tag references as slugs, so this
	// gives a restored tag a readable label instead of its identifier.
	labelFromSlugShapedInput
)

// EnsureTagsExist looks up each tag the user typed in the current group and
// provisions a new Tag row with DefaultTagColor for those that don't exist
// yet. Returns the canonical tags map keyed by slug.
//
// Each input is normalized via models.NormalizeTagSlug, which keeps letters
// and digits of every script. An input that normalizes to nothing (only
// spaces, punctuation or symbols) is skipped. A created tag's label is the
// input as typed (trimmed, in NFC), so "USB кабель" gets the slug
// "usb-кабель" and the label "USB кабель". When several inputs map to one
// slug, the first one names the tag.
//
// The method is idempotent — calling it twice with the same input yields
// the same result. Concurrent calls with overlapping slugs may race on the
// (group_id, slug) unique index; the second writer's INSERT fails with a
// duplicate-key error and the caller should retry. Currently we surface
// the error rather than retrying — the autocomplete UI prevents most
// realistic races, and the issue body permits this (concurrency target is
// rename, not auto-create).
func (s *TagService) EnsureTagsExist(ctx context.Context, kind models.TagKind, raw []string) (map[string]*models.Tag, error) {
	return s.ensureTagsExist(ctx, kind, raw, labelFromInput)
}

func (s *TagService) ensureTagsExist(ctx context.Context, kind models.TagKind, raw []string, labels tagLabelSource) (map[string]*models.Tag, error) {
	tagReg, err := s.factorySet.TagRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create tag registry", err)
	}

	out := make(map[string]*models.Tag, len(raw))

	for _, input := range raw {
		slug := models.NormalizeTagSlug(input)
		if slug == "" {
			continue
		}
		if _, ok := out[slug]; ok {
			continue
		}

		existing, err := tagReg.GetBySlug(ctx, kind, slug)
		if err == nil && existing != nil {
			out[slug] = existing
			continue
		}
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			return nil, errxtrace.Wrap("failed to look up tag", err, errx.Attrs("slug", slug))
		}

		// Auto-create with the default color. The user can rename it
		// later via PATCH. Timestamps are stamped explicitly so the memory
		// backend (which has no DEFAULT CURRENT_TIMESTAMP) returns a usable
		// CreatedAt for recency ranking; on postgres the value mirrors
		// what the DEFAULT would have produced.
		now := time.Now()
		tag := models.Tag{
			Kind:      kind,
			Slug:      slug,
			Label:     autoCreatedTagLabel(input, slug, labels),
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

// NormalizeAndEnsureSlugs takes a user-supplied tag list (raw text as
// typed, or existing slugs), normalizes each into canonical slugs, ensures
// the underlying rows exist via EnsureTagsExist, and returns the
// deduplicated slug list ready to be persisted into JSONB.
//
// Order of `raw` is preserved (first-seen wins on duplicates) so the
// resulting JSONB array is deterministic — round-tripping the same
// commodity payload twice produces byte-identical responses, and noisy
// order-only diffs in tests / e2e snapshots disappear.
//
// Returns an empty slice (never nil) when nothing usable remains, so
// callers persist `[]` instead of `null` on the JSONB column.
func (s *TagService) NormalizeAndEnsureSlugs(ctx context.Context, kind models.TagKind, raw []string) ([]string, error) {
	return s.normalizeAndEnsureSlugs(ctx, kind, raw, labelFromInput)
}

// NormalizeAndEnsureRestoredSlugs is NormalizeAndEnsureSlugs for tag
// references read back from a backup. Those are slugs, so a tag it has to
// create gets a label title-cased from the slug ("new-tag-a" becomes
// "New Tag A"); a reference that is not slug-shaped keeps its text.
func (s *TagService) NormalizeAndEnsureRestoredSlugs(ctx context.Context, kind models.TagKind, refs []string) ([]string, error) {
	return s.normalizeAndEnsureSlugs(ctx, kind, refs, labelFromSlugShapedInput)
}

func (s *TagService) normalizeAndEnsureSlugs(ctx context.Context, kind models.TagKind, raw []string, labels tagLabelSource) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	tags, err := s.ensureTagsExist(ctx, kind, raw, labels)
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
			// Defensive: should never happen if ensureTagsExist succeeded.
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
//
// Saved views name item tags by slug, so after an item tag's slug changes
// the views of the group that name it are rewritten from the slug the rename
// started from (read under the rename's lock) to the slug the tag has when
// the views are rewritten. That rewrite runs after the rename commits; when
// it fails the rename stands and the views show the old slug as a tag that
// no longer exists.
func (s *TagService) RenameTag(ctx context.Context, id, newLabel, newSlug string, newColor models.TagColor) (*models.Tag, error) {
	tagReg, err := s.factorySet.TagRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create tag registry", err)
	}
	renamed, from, err := tagReg.RenameAtomic(ctx, id, newLabel, newSlug, newColor)
	if err != nil {
		return nil, err
	}
	if renamed.Kind == models.TagKindCommodity && renamed.Slug != from && s.factorySet.SavedViewRegistryFactory != nil {
		if _, err := NewSavedViewService(s.factorySet).RenameTag(ctx, from, id); err != nil {
			slog.WarnContext(ctx, "Saved views kept the old tag slug after a rename",
				"tag_id", id, "from", from, "to", renamed.Slug, "error", err)
		}
	}
	return renamed, nil
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

// autoCreatedTagLabel picks the label for a tag created on first reference.
// Typed text longer than models.MaxTagLabelRunes is cut to that length: the
// slug can fit the limit while the text it came from does not, since the
// slug drops punctuation and spaces.
func autoCreatedTagLabel(input, slug string, labels tagLabelSource) string {
	typed := norm.NFC.String(strings.TrimSpace(input))
	if labels == labelFromSlugShapedInput && typed == slug {
		return defaultLabelFromSlug(slug)
	}
	if runes := []rune(typed); len(runes) > models.MaxTagLabelRunes {
		typed = strings.TrimSpace(string(runes[:models.MaxTagLabelRunes]))
	}
	return typed
}

// defaultLabelFromSlug produces a display label from a slug: split on '-',
// title-case the first letter of each word. Rune-aware, so a word that
// starts with a multi-byte letter ("čaj", "кухня") is not cut mid-sequence.
func defaultLabelFromSlug(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		runes := []rune(p)
		runes[0] = unicode.ToTitle(runes[0])
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}
