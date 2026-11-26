package services

import (
	"context"
	"errors"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// sectionGate tells a reminder sweep whether a group hides the section the
// reminder belongs to (models.GroupSection). A hidden section gets no
// reminders: the sweep skips its rows without writing an idempotency row or
// flipping a sent flag, so showing the section again resumes the reminders
// that are still due.
//
// Each group is read once per sweep, however many of its rows are due.
type sectionGate struct {
	groups  registry.LocationGroupRegistry
	byGroup map[string]models.GroupSections
}

func newSectionGate(fs *registry.FactorySet) *sectionGate {
	gate := &sectionGate{byGroup: map[string]models.GroupSections{}}
	if fs != nil {
		gate.groups = fs.LocationGroupRegistry
	}
	return gate
}

// hides reports whether the group hides section. A row without a group, a
// group that is gone and a sweep without a group registry hide nothing.
// Any other lookup error is returned, so the caller fails the row for this
// sweep and retries it on the next one instead of sending a reminder the
// group may have turned off.
func (g *sectionGate) hides(ctx context.Context, groupID string, section models.GroupSection) (bool, error) {
	if g == nil || g.groups == nil || groupID == "" {
		return false, nil
	}
	hidden, ok := g.byGroup[groupID]
	if !ok {
		group, err := g.groups.Get(ctx, groupID)
		switch {
		case errors.Is(err, registry.ErrNotFound):
			hidden = models.GroupSections{}
		case err != nil:
			return false, err
		default:
			hidden = group.HiddenSections
		}
		g.byGroup[groupID] = hidden
	}
	return hidden.Contains(section), nil
}
