package memory

import (
	"context"
	"slices"
	"sort"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

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

func (r *TagRegistry) GetBySlug(ctx context.Context, slug string) (*models.Tag, error) {
	tags, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, t := range tags {
		if t.Slug == slug {
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

	if opts.Search != "" {
		needle := strings.ToLower(opts.Search)
		filtered := all[:0:0]
		for _, t := range all {
			if strings.Contains(strings.ToLower(t.Label), needle) || strings.Contains(t.Slug, needle) {
				filtered = append(filtered, t)
			}
		}
		all = filtered
	}

	usageBySlug := map[string]int{}
	if opts.SortField == registry.TagSortUsage {
		// Only walk commodities/files when usage sort is requested — it is
		// the expensive case. Other sorts read fields directly off the tag.
		usageBySlug, err = r.computeUsageMap(ctx)
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
			ui := usageBySlug[all[i].Slug]
			uj := usageBySlug[all[j].Slug]
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

func (r *TagRegistry) Search(ctx context.Context, q string, limit int) ([]*models.Tag, error) {
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	needle := strings.ToLower(q)
	matched := all[:0:0]
	for _, t := range all {
		if needle == "" ||
			strings.Contains(strings.ToLower(t.Label), needle) ||
			strings.Contains(t.Slug, needle) {
			matched = append(matched, t)
		}
	}

	usageBySlug, err := r.computeUsageMap(ctx)
	if err != nil {
		return nil, err
	}
	// Rank: usage desc, then created_at desc (recent wins ties).
	sort.SliceStable(matched, func(i, j int) bool {
		ui, uj := usageBySlug[matched[i].Slug], usageBySlug[matched[j].Slug]
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

func (r *TagRegistry) GetUsage(ctx context.Context, slug string) (registry.TagUsage, error) {
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

	return registry.TagUsage{Commodities: commodityCount, Files: fileCount}, nil
}

func (r *TagRegistry) computeUsageMap(ctx context.Context) (map[string]int, error) {
	usage := map[string]int{}

	commodities, err := r.commodityRegistry.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodities", err)
	}
	for _, c := range commodities {
		for _, slug := range c.Tags {
			usage[slug]++
		}
	}

	files, err := r.fileRegistry.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files", err)
	}
	for _, f := range files {
		for _, slug := range f.Tags {
			usage[slug]++
		}
	}
	return usage, nil
}

func (r *TagRegistry) RewriteSlugReferences(ctx context.Context, oldSlug, newSlug string) (commodityRows, fileRows int, err error) {
	if oldSlug == newSlug {
		return 0, 0, nil
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

	files, err := r.fileRegistry.List(ctx)
	if err != nil {
		return commodityCount, 0, errxtrace.Wrap("failed to list files", err)
	}
	fileCount := 0
	for _, f := range files {
		changed, newTags := replaceTagSlugString(f.Tags, oldSlug, newSlug)
		if !changed {
			continue
		}
		f.Tags = newTags
		if _, err := r.fileRegistry.Update(ctx, *f); err != nil {
			return commodityCount, fileCount, errxtrace.Wrap("failed to rewrite file tag", err)
		}
		fileCount++
	}

	return commodityCount, fileCount, nil
}

func (r *TagRegistry) StripSlugReferences(ctx context.Context, slug string) (commodityRows, fileRows int, err error) {
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

	files, err := r.fileRegistry.List(ctx)
	if err != nil {
		return commodityCount, 0, errxtrace.Wrap("failed to list files", err)
	}
	fileCount := 0
	for _, f := range files {
		changed, newTags := stripTagSlugString(f.Tags, slug)
		if !changed {
			continue
		}
		f.Tags = newTags
		if _, err := r.fileRegistry.Update(ctx, *f); err != nil {
			return commodityCount, fileCount, errxtrace.Wrap("failed to strip file tag", err)
		}
		fileCount++
	}
	return commodityCount, fileCount, nil
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
