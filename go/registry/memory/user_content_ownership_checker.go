package memory

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
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

// HasRetainedOwnedContent walks the ownedSources whose rows can block a
// deletion and returns true on the first row the user owns in a retained
// group: any row of a deletionBlocks source, and a row of a deletionHandsOver
// source whose group has no other member to hand it to. It performs no writes.
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
	heirs := heirLookup{memberships: r.fs.GroupMembershipRegistry, userID: userID, known: map[string]bool{}}

	for _, src := range ownedSources(r.fs) {
		if src.onDeletion != deletionBlocks && src.onDeletion != deletionHandsOver {
			continue
		}
		rows, err := src.rows(ctx)
		if err != nil {
			return false, errxtrace.Wrap("failed to check retained owned content in "+string(src.kind), err)
		}
		for _, row := range rows {
			if row.tenantID != tenantID || row.createdBy != userID {
				continue
			}
			if _, isPurged := purged[row.groupID]; isPurged {
				continue
			}
			if src.onDeletion == deletionHandsOver {
				has, err := heirs.has(ctx, row.groupID)
				if err != nil {
					return false, err
				}
				if has {
					continue
				}
			}
			return true, nil
		}
	}
	return false, nil
}

// heirLookup answers, once per group, whether a group has a member other
// than userID to hand the user's rows to.
type heirLookup struct {
	memberships registry.GroupMembershipRegistry
	userID      string
	known       map[string]bool
}

func (h heirLookup) has(ctx context.Context, groupID string) (bool, error) {
	if has, ok := h.known[groupID]; ok {
		return has, nil
	}
	members, err := h.memberships.ListByGroup(ctx, groupID)
	if err != nil {
		return false, errxtrace.Wrap("failed to list the group's members", err)
	}
	has := false
	for _, m := range members {
		if m != nil && m.MemberUserID != h.userID {
			has = true
			break
		}
	}
	h.known[groupID] = has
	return has, nil
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
// embedder does). Used as the extractor for the group-scoped ownedSources.
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
