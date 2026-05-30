package memory

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// tagAtomicMu serializes RenameAtomic / DeleteAtomic in the memory
// backend with each other so the iterate-then-update sequences they run
// (List → Update per tag) don't interleave.
//
// **Scope caveat:** the lock does NOT cover commodity / file Create or
// Update on the memory backend — those write paths persist into the
// shared in-memory map under their own per-registry lock and don't
// touch tagAtomicMu. A concurrent memory-backend commodity write could
// therefore still leave a JSONB reference to a slug DeleteAtomic is
// about to remove, or use a slug RenameAtomic is currently rewriting.
// We accept that gap because:
//   - memory backend exists for unit tests / single-process flows;
//     none of those exercise concurrent writes (postgres is the
//     production target with the real cross-tx coordination — see
//     ensureTagRowsInTx + pg_advisory_xact_lock in registry/postgres);
//   - closing it would require routing every commodity / file write
//     through a per-slug serialization layer in the memory backend
//     for no production benefit.
//
// If a future use case puts the memory backend under concurrent write
// load, this is the spot to fix.
var tagAtomicMu sync.Mutex

// TagRegistryFactory creates TagRegistry instances with proper context.
// The factory stores references to the commodity + file factories so the
// per-request registry can compute usage counts and rewrite/strip JSONB
// references without cross-package plumbing.
type TagRegistryFactory struct {
	base             *Registry[models.Tag, *models.Tag]
	commodityFactory *CommodityRegistryFactory
	fileFactory      *FileRegistryFactory
}

// TagRegistry is a context-aware registry for tags. Cross-entity helpers
// (usage / rewrite / strip) operate against the commodity + file registries
// supplied at construction time.
type TagRegistry struct {
	*Registry[models.Tag, *models.Tag]

	userID            string
	commodityRegistry registry.CommodityRegistry
	fileRegistry      registry.FileRegistry
}

var (
	_ registry.TagRegistry        = (*TagRegistry)(nil)
	_ registry.TagRegistryFactory = (*TagRegistryFactory)(nil)
)

func NewTagRegistryFactory(commodityFactory *CommodityRegistryFactory, fileFactory *FileRegistryFactory) *TagRegistryFactory {
	return &TagRegistryFactory{
		base:             NewRegistry[models.Tag, *models.Tag](),
		commodityFactory: commodityFactory,
		fileFactory:      fileFactory,
	}
}

func (f *TagRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.TagRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *TagRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.TagRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}

	groupID := appctx.GroupIDFromContext(ctx)
	userRegistry := &Registry[models.Tag, *models.Tag]{
		items:   f.base.items,
		lock:    f.base.lock,
		userID:  user.ID,
		groupID: groupID,
	}

	commodityReg, err := f.commodityFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create user commodity registry", err)
	}
	fileReg, err := f.fileFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create user file registry", err)
	}

	return &TagRegistry{
		Registry:          userRegistry,
		userID:            user.ID,
		commodityRegistry: commodityReg,
		fileRegistry:      fileReg,
	}, nil
}

func (f *TagRegistryFactory) CreateServiceRegistry() registry.TagRegistry {
	serviceRegistry := &Registry[models.Tag, *models.Tag]{
		items:  f.base.items,
		lock:   f.base.lock,
		userID: "",
	}

	return &TagRegistry{
		Registry:          serviceRegistry,
		userID:            "",
		commodityRegistry: f.commodityFactory.CreateServiceRegistry(),
		fileRegistry:      f.fileFactory.CreateServiceRegistry(),
	}
}

// Create overrides the base Create so it can populate user/group fields
// from context (CreateWithUser handles that for us) and stamp timestamps.
func (r *TagRegistry) Create(ctx context.Context, tag models.Tag) (*models.Tag, error) {
	created, err := r.Registry.CreateWithUser(ctx, tag)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create tag", err)
	}
	return created, nil
}

func (r *TagRegistry) Update(ctx context.Context, tag models.Tag) (*models.Tag, error) {
	updated, err := r.Registry.UpdateWithUser(ctx, tag)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update tag", err)
	}
	return updated, nil
}

func (r *TagRegistry) GetBySlug(ctx context.Context, kind models.TagKind, slug string) (*models.Tag, error) {
	tags, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		if t.Kind == kind && t.Slug == slug {
			return t, nil
		}
	}
	return nil, registry.ErrNotFound
}

func (r *TagRegistry) ListPaginated(ctx context.Context, offset, limit int, opts registry.TagListOptions) ([]*models.Tag, int, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, 0, err
	}

	all = filterTagsBySearch(all, opts.Search)
	all = filterTagsByKind(all, opts.Kind)

	// usagePerScope is computed only when usage sort is selected — the kind
	// filter is now intrinsic (the Kind field), not usage-derived.
	var usagePerScope map[string]registry.TagUsage
	if opts.SortField == registry.TagSortUsage {
		usagePerScope, err = r.computePerScopeUsageMap(ctx)
		if err != nil {
			return nil, 0, err
		}
	}

	sortField := opts.SortField
	if !sortField.IsValid() {
		sortField = registry.TagSortLabel
	}
	sort.SliceStable(all, func(i, j int) bool {
		var less bool
		switch sortField {
		case registry.TagSortCreatedAt:
			less = all[i].CreatedAt.Before(all[j].CreatedAt)
		case registry.TagSortUsage:
			ui := scopedUsage(usagePerScope[all[i].Slug], opts.Kind)
			uj := scopedUsage(usagePerScope[all[j].Slug], opts.Kind)
			if ui == uj {
				less = strings.ToLower(all[i].Label) < strings.ToLower(all[j].Label)
			} else {
				less = ui < uj
			}
		default:
			less = strings.ToLower(all[i].Label) < strings.ToLower(all[j].Label)
		}
		if opts.SortDesc {
			return !less
		}
		return less
	})

	total := len(all)
	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}
	start := min(offset, total)
	end := min(start+limit, total)
	return all[start:end], total, nil
}

func (r *TagRegistry) Search(ctx context.Context, q string, limit int, kind models.TagKind) ([]*models.Tag, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	// Reuse the same substring filter as ListPaginated — empty q is a
	// no-op pass-through, so the "match everything" case stays cheap.
	matched := filterTagsBySearch(all, q)

	// Intrinsic kind filter — a commodity input only ever sees commodity
	// tags, a file input only file tags.
	matched = filterTagsByKind(matched, kind)

	usagePerScope, err := r.computePerScopeUsageMap(ctx)
	if err != nil {
		return nil, err
	}

	// Rank: kind usage desc, then created_at desc (recent wins ties).
	sort.SliceStable(matched, func(i, j int) bool {
		ui := scopedUsage(usagePerScope[matched[i].Slug], kind)
		uj := scopedUsage(usagePerScope[matched[j].Slug], kind)
		if ui != uj {
			return ui > uj
		}
		return matched[i].CreatedAt.After(matched[j].CreatedAt)
	})

	if limit > 0 && limit < len(matched) {
		matched = matched[:limit]
	}
	return matched, nil
}

// scopedUsage returns the part of TagUsage relevant to the given kind.
// Empty kind sums commodities + files; explicit kinds return just that
// bucket.
func scopedUsage(u registry.TagUsage, kind models.TagKind) int {
	switch kind {
	case models.TagKindCommodity:
		return u.Commodities
	case models.TagKindFile:
		return u.Files
	default:
		return u.Commodities + u.Files
	}
}

// filterTagsBySearch is the substring-match helper shared by
// ListPaginated and (indirectly) Search. Lifted out so the cognitive
// complexity of ListPaginated stays under gocognit's threshold.
func filterTagsBySearch(in []*models.Tag, search string) []*models.Tag {
	if search == "" {
		return in
	}
	needle := strings.ToLower(search)
	filtered := in[:0:0]
	for _, t := range in {
		if strings.Contains(strings.ToLower(t.Label), needle) || strings.Contains(t.Slug, needle) {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// filterTagsByKind keeps only tags whose stored Kind matches. Empty kind
// (TagKindAny) is a no-op pass-through (list across all kinds).
func filterTagsByKind(in []*models.Tag, kind models.TagKind) []*models.Tag {
	if kind != models.TagKindCommodity && kind != models.TagKindFile {
		return in
	}
	filtered := in[:0:0]
	for _, t := range in {
		if t.Kind == kind {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

func (r *TagRegistry) GetUsageBatch(ctx context.Context, kind models.TagKind, slugs []string) (map[string]registry.TagUsage, error) {
	out := make(map[string]registry.TagUsage, len(slugs))
	for _, s := range slugs {
		out[s] = registry.TagUsage{}
	}
	if len(slugs) == 0 {
		return out, nil
	}

	// Count only the kind's own table — commodity tags count commodity
	// rows, file tags count file rows. Per-row seen-set so a row with a
	// duplicated slug in its JSONB tags array is counted at most once.
	if kind == models.TagKindFile {
		files, err := r.fileRegistry.List(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to list files", err)
		}
		for _, f := range files {
			seen := map[string]struct{}{}
			for _, slug := range f.Tags {
				if _, dup := seen[slug]; dup {
					continue
				}
				seen[slug] = struct{}{}
				if u, ok := out[slug]; ok {
					u.Files++
					out[slug] = u
				}
			}
		}
		return out, nil
	}

	commodities, err := r.commodityRegistry.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodities", err)
	}
	for _, c := range commodities {
		seen := map[string]struct{}{}
		for _, slug := range c.Tags {
			if _, dup := seen[slug]; dup {
				continue
			}
			seen[slug] = struct{}{}
			if u, ok := out[slug]; ok {
				u.Commodities++
				out[slug] = u
			}
		}
	}
	return out, nil
}

func (r *TagRegistry) GetStats(ctx context.Context) (registry.TagStats, error) {
	tags, err := r.List(ctx)
	if err != nil {
		return registry.TagStats{}, errxtrace.Wrap("failed to list tags", err)
	}
	commodityTagsTotal, fileTagsTotal := 0, 0
	for _, t := range tags {
		switch t.Kind {
		case models.TagKindCommodity:
			commodityTagsTotal++
		case models.TagKindFile:
			fileTagsTotal++
		}
	}

	commodities, err := r.commodityRegistry.List(ctx)
	if err != nil {
		return registry.TagStats{}, errxtrace.Wrap("failed to list commodities", err)
	}
	itemsTagged := 0
	for _, c := range commodities {
		if len(c.Tags) > 0 {
			itemsTagged++
		}
	}

	files, err := r.fileRegistry.List(ctx)
	if err != nil {
		return registry.TagStats{}, errxtrace.Wrap("failed to list files", err)
	}
	filesTagged := 0
	for _, f := range files {
		if len(f.Tags) > 0 {
			filesTagged++
		}
	}

	return registry.TagStats{
		TagsTotal:          len(tags),
		CommodityTagsTotal: commodityTagsTotal,
		FileTagsTotal:      fileTagsTotal,
		ItemsTagged:        itemsTagged,
		ItemsUntagged:      len(commodities) - itemsTagged,
		FilesTagged:        filesTagged,
		FilesUntagged:      len(files) - filesTagged,
	}, nil
}

func (r *TagRegistry) GetUsage(ctx context.Context, kind models.TagKind, slug string) (registry.TagUsage, error) {
	if kind == models.TagKindFile {
		files, err := r.fileRegistry.List(ctx)
		if err != nil {
			return registry.TagUsage{}, errxtrace.Wrap("failed to list files", err)
		}
		fileCount := 0
		for _, f := range files {
			if slices.Contains([]string(f.Tags), slug) {
				fileCount++
			}
		}
		return registry.TagUsage{Files: fileCount}, nil
	}

	commodities, err := r.commodityRegistry.List(ctx)
	if err != nil {
		return registry.TagUsage{}, errxtrace.Wrap("failed to list commodities", err)
	}
	commodityCount := 0
	for _, c := range commodities {
		if slices.Contains([]string(c.Tags), slug) {
			commodityCount++
		}
	}
	return registry.TagUsage{Commodities: commodityCount}, nil
}

// computePerScopeUsageMap walks commodities + files once each and returns
// a slug→TagUsage breakdown with separate Commodities / Files counts.
// Mirrors the postgres `scopedUsageExpr` semantics: a commodity / file
// with a duplicated slug in its tags array is counted at most once
// (matching @> containment + COUNT(DISTINCT id)).
func (r *TagRegistry) computePerScopeUsageMap(ctx context.Context) (map[string]registry.TagUsage, error) {
	usage := map[string]registry.TagUsage{}

	commodities, err := r.commodityRegistry.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodities", err)
	}
	for _, c := range commodities {
		seen := map[string]struct{}{}
		for _, slug := range c.Tags {
			if _, dup := seen[slug]; dup {
				continue
			}
			seen[slug] = struct{}{}
			u := usage[slug]
			u.Commodities++
			usage[slug] = u
		}
	}

	files, err := r.fileRegistry.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files", err)
	}
	for _, f := range files {
		seen := map[string]struct{}{}
		for _, slug := range f.Tags {
			if _, dup := seen[slug]; dup {
				continue
			}
			seen[slug] = struct{}{}
			u := usage[slug]
			u.Files++
			usage[slug] = u
		}
	}
	return usage, nil
}

func (r *TagRegistry) RewriteSlugReferences(ctx context.Context, kind models.TagKind, oldSlug, newSlug string) (commodityRows, fileRows int, err error) {
	if oldSlug == newSlug {
		return 0, 0, nil
	}

	if kind == models.TagKindFile {
		files, err := r.fileRegistry.List(ctx)
		if err != nil {
			return 0, 0, errxtrace.Wrap("failed to list files", err)
		}
		fileCount := 0
		for _, f := range files {
			changed, newTags := replaceTagSlugString(f.Tags, oldSlug, newSlug)
			if !changed {
				continue
			}
			f.Tags = newTags
			if _, err := r.fileRegistry.Update(ctx, *f); err != nil {
				return 0, fileCount, errxtrace.Wrap("failed to rewrite file tag", err)
			}
			fileCount++
		}
		return 0, fileCount, nil
	}

	commodities, err := r.commodityRegistry.List(ctx)
	if err != nil {
		return 0, 0, errxtrace.Wrap("failed to list commodities", err)
	}
	commodityCount := 0
	for _, c := range commodities {
		changed, newTags := replaceTagSlug(c.Tags, oldSlug, newSlug)
		if !changed {
			continue
		}
		c.Tags = newTags
		if _, err := r.commodityRegistry.Update(ctx, *c); err != nil {
			return commodityCount, 0, errxtrace.Wrap("failed to rewrite commodity tag", err)
		}
		commodityCount++
	}
	return commodityCount, 0, nil
}

func (r *TagRegistry) StripSlugReferences(ctx context.Context, kind models.TagKind, slug string) (commodityRows, fileRows int, err error) {
	if kind == models.TagKindFile {
		files, err := r.fileRegistry.List(ctx)
		if err != nil {
			return 0, 0, errxtrace.Wrap("failed to list files", err)
		}
		fileCount := 0
		for _, f := range files {
			changed, newTags := stripTagSlugString(f.Tags, slug)
			if !changed {
				continue
			}
			f.Tags = newTags
			if _, err := r.fileRegistry.Update(ctx, *f); err != nil {
				return 0, fileCount, errxtrace.Wrap("failed to strip file tag", err)
			}
			fileCount++
		}
		return 0, fileCount, nil
	}

	commodities, err := r.commodityRegistry.List(ctx)
	if err != nil {
		return 0, 0, errxtrace.Wrap("failed to list commodities", err)
	}
	commodityCount := 0
	for _, c := range commodities {
		changed, newTags := stripTagSlug(c.Tags, slug)
		if !changed {
			continue
		}
		c.Tags = newTags
		if _, err := r.commodityRegistry.Update(ctx, *c); err != nil {
			return commodityCount, 0, errxtrace.Wrap("failed to strip commodity tag", err)
		}
		commodityCount++
	}
	return commodityCount, 0, nil
}

// RenameAtomic mirrors the postgres semantics: re-read the tag, run the
// slug-clash check, rewrite JSONB references, and update the tag row,
// all under the same mutex. The memory backend doesn't have separate
// transactions — the mutex is the entire serialization mechanism.
func (r *TagRegistry) RenameAtomic(ctx context.Context, id, newLabel, newSlug string, newColor models.TagColor) (*models.Tag, error) {
	tagAtomicMu.Lock()
	defer tagAtomicMu.Unlock()

	current, err := r.Get(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to look up tag", err)
	}

	updated := *current
	updated.UpdatedAt = time.Now()
	if strings.TrimSpace(newLabel) != "" {
		updated.Label = newLabel
	}
	if newColor != "" {
		updated.Color = newColor
	}

	slugChanged := newSlug != "" && newSlug != current.Slug
	if slugChanged {
		updated.Slug = newSlug
		// Clash + rewrite are scoped to the tag's own kind: the same slug
		// may legitimately exist under the other kind.
		clash, clashErr := r.GetBySlug(ctx, current.Kind, newSlug)
		if clashErr != nil && !errors.Is(clashErr, registry.ErrNotFound) {
			return nil, errxtrace.Wrap("failed to check slug availability", clashErr)
		}
		if clash != nil && clash.ID != current.ID {
			return nil, errxtrace.Wrap("target slug is already used by another tag",
				registry.ErrAlreadyExists, errx.Attrs("slug", newSlug))
		}
		if _, _, err := r.RewriteSlugReferences(ctx, current.Kind, current.Slug, newSlug); err != nil {
			return nil, errxtrace.Wrap("failed to rewrite slug references", err)
		}
	}

	final, err := r.Update(ctx, updated)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update tag", err)
	}
	return final, nil
}

// DeleteAtomic mirrors the postgres semantics: usage check + strip (when
// force=true) + delete, all under the same mutex.
//
//revive:disable-next-line:flag-parameter
func (r *TagRegistry) DeleteAtomic(ctx context.Context, id string, force bool) (registry.TagUsage, error) {
	tagAtomicMu.Lock()
	defer tagAtomicMu.Unlock()

	current, err := r.Get(ctx, id)
	if err != nil {
		return registry.TagUsage{}, errxtrace.Wrap("failed to look up tag", err)
	}

	usage, err := r.GetUsage(ctx, current.Kind, current.Slug)
	if err != nil {
		return registry.TagUsage{}, errxtrace.Wrap("failed to compute tag usage", err)
	}

	if usage.Commodities+usage.Files > 0 && !force {
		return usage, registry.ErrTagInUse
	}
	if usage.Commodities+usage.Files > 0 {
		if _, _, err := r.StripSlugReferences(ctx, current.Kind, current.Slug); err != nil {
			return usage, errxtrace.Wrap("failed to strip slug references", err)
		}
	}
	if err := r.Delete(ctx, id); err != nil {
		return usage, errxtrace.Wrap("failed to delete tag", err)
	}
	return usage, nil
}

// replaceTagSlug rewrites every occurrence of oldSlug in a ValuerSlice[string]
// to newSlug. Returns whether anything changed and the resulting slice.
// Skips the rewrite when newSlug already appears alongside oldSlug to avoid
// duplicates after the merge.
func replaceTagSlug(in models.ValuerSlice[string], oldSlug, newSlug string) (bool, models.ValuerSlice[string]) {
	if !slices.Contains([]string(in), oldSlug) {
		return false, in
	}
	hasNew := slices.Contains([]string(in), newSlug)
	out := make(models.ValuerSlice[string], 0, len(in))
	for _, t := range in {
		if t == oldSlug {
			if hasNew {
				continue
			}
			out = append(out, newSlug)
			continue
		}
		out = append(out, t)
	}
	return true, out
}

func replaceTagSlugString(in models.StringSlice, oldSlug, newSlug string) (bool, models.StringSlice) {
	if !slices.Contains([]string(in), oldSlug) {
		return false, in
	}
	hasNew := slices.Contains([]string(in), newSlug)
	out := make(models.StringSlice, 0, len(in))
	for _, t := range in {
		if t == oldSlug {
			if hasNew {
				continue
			}
			out = append(out, newSlug)
			continue
		}
		out = append(out, t)
	}
	return true, out
}

func stripTagSlug(in models.ValuerSlice[string], slug string) (bool, models.ValuerSlice[string]) {
	if !slices.Contains([]string(in), slug) {
		return false, in
	}
	out := make(models.ValuerSlice[string], 0, len(in))
	for _, t := range in {
		if t != slug {
			out = append(out, t)
		}
	}
	return true, out
}

func stripTagSlugString(in models.StringSlice, slug string) (bool, models.StringSlice) {
	if !slices.Contains([]string(in), slug) {
		return false, in
	}
	out := make(models.StringSlice, 0, len(in))
	for _, t := range in {
		if t != slug {
			out = append(out, t)
		}
	}
	return true, out
}
