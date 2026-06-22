package memory

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.UserContentOwnershipChecker = (*UserContentOwnershipChecker)(nil)

// UserContentOwnershipChecker is the in-memory counterpart to
// postgres.UserContentOwnershipChecker (#2147). It iterates the service-mode
// registry views (RLS-equivalent disabled) and reports whether the user authored
// any content — or created any location group — that is NOT inside the set of
// private groups the caller will purge.
//
// Like the memory TenantPurger it takes the whole FactorySet (it touches several
// content registries) and must be wired after every fs.* field is populated.
type UserContentOwnershipChecker struct {
	fs *registry.FactorySet
}

// NewUserContentOwnershipChecker wires a checker to the populated FactorySet.
func NewUserContentOwnershipChecker(fs *registry.FactorySet) *UserContentOwnershipChecker {
	return &UserContentOwnershipChecker{fs: fs}
}

// HasRetainedOwnedContent walks each group-scoped content registry plus
// location_groups and returns true on the first row the user owns in a retained
// group. It performs no writes.
func (r *UserContentOwnershipChecker) HasRetainedOwnedContent(ctx context.Context, tenantID, userID string, purgedGroupIDs []string) (bool, error) {
	if tenantID == "" {
		return false, errxtrace.Wrap("tenantID required", registry.ErrFieldRequired)
	}
	if userID == "" {
		return false, errxtrace.Wrap("userID required", registry.ErrFieldRequired)
	}

	purged := make(map[string]struct{}, len(purgedGroupIDs))
	for _, id := range purgedGroupIDs {
		purged[id] = struct{}{}
	}

	fs := r.fs

	type check struct {
		name string
		run  func() (bool, error)
	}
	checks := []check{
		// Group-scoped content authored by the user (created_by_user_id) in a
		// group outside the purged set. Every table embeds
		// TenantGroupAwareEntityID, so the generic created-by matcher applies.
		{"commodities", func() (bool, error) {
			reg := fs.CommodityRegistryFactory.CreateServiceRegistry()
			return ownsRetainedContent(ctx, tenantID, userID, purged, reg.List, createdByGroupAware[models.Commodity])
		}},
		{"files", func() (bool, error) {
			reg := fs.FileRegistryFactory.CreateServiceRegistry()
			return ownsRetainedContent(ctx, tenantID, userID, purged, reg.List, createdByGroupAware[models.FileEntity])
		}},
		{"areas", func() (bool, error) {
			reg := fs.AreaRegistryFactory.CreateServiceRegistry()
			return ownsRetainedContent(ctx, tenantID, userID, purged, reg.List, createdByGroupAware[models.Area])
		}},
		{"locations", func() (bool, error) {
			reg := fs.LocationRegistryFactory.CreateServiceRegistry()
			return ownsRetainedContent(ctx, tenantID, userID, purged, reg.List, createdByGroupAware[models.Location])
		}},
		{"exports", func() (bool, error) {
			reg := fs.ExportRegistryFactory.CreateServiceRegistry()
			return ownsRetainedContent(ctx, tenantID, userID, purged, reg.List, createdByGroupAware[models.Export])
		}},
		{"tags", func() (bool, error) {
			reg := fs.TagRegistryFactory.CreateServiceRegistry()
			return ownsRetainedContent(ctx, tenantID, userID, purged, reg.List, createdByGroupAware[models.Tag])
		}},
		// location_groups the user CREATED that are not being purged. The group's
		// own id is the exclusion key (it has no group_id), and the authorship
		// column is `created_by`.
		{"location_groups", func() (bool, error) {
			groups, err := fs.LocationGroupRegistry.List(ctx)
			if err != nil {
				return false, err
			}
			for _, g := range groups {
				if g == nil || g.GetTenantID() != tenantID || g.CreatedBy != userID {
					continue
				}
				if _, isPurged := purged[g.ID]; isPurged {
					continue
				}
				return true, nil
			}
			return false, nil
		}},
	}

	for _, ch := range checks {
		owns, err := ch.run()
		if err != nil {
			return false, errxtrace.Wrap("failed to check retained owned content in "+ch.name, err)
		}
		if owns {
			return true, nil
		}
	}
	return false, nil
}

// createdByEntry is the trio the ownership matcher needs from a content row:
// its tenant id, its group id, and the user who created it.
type createdByEntry struct {
	tenantID  string
	groupID   string
	createdBy string
}

// createdByGroupAware extracts (tenant, group, created_by) from any model whose
// pointer satisfies models.TenantGroupAware (every TenantGroupAwareEntityID
// embedder does). Used as the default extractor for ownsRetainedContent.
func createdByGroupAware[T any](item *T) createdByEntry {
	ga, ok := any(item).(models.TenantGroupAware)
	if !ok {
		return createdByEntry{}
	}
	return createdByEntry{
		tenantID:  ga.GetTenantID(),
		groupID:   ga.GetGroupID(),
		createdBy: ga.GetCreatedByUserID(),
	}
}

// ownsRetainedContent lists everything from a service-mode registry view and
// reports whether any row is owned by userID (created_by_user_id) in this tenant
// and in a group that is NOT being purged.
func ownsRetainedContent[T any](
	ctx context.Context,
	tenantID, userID string,
	purged map[string]struct{},
	list func(context.Context) ([]*T, error),
	extract func(*T) createdByEntry,
) (bool, error) {
	items, err := list(ctx)
	if err != nil {
		return false, err
	}
	for _, item := range items {
		if item == nil {
			continue
		}
		e := extract(item)
		if e.tenantID != tenantID || e.createdBy != userID {
			continue
		}
		if _, isPurged := purged[e.groupID]; isPurged {
			continue
		}
		return true, nil
	}
	return false, nil
}
