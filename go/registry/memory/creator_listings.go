package memory

import (
	"context"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// The creator listings (#2934) keep the rows of List, or of the Trash
// listing, that the user created. The memory backend holds every row in
// memory anyway; the postgres backend is the one that saves the reads.

// createdByUser returns the rows whose creator is userID.
func createdByUser[T any, P interface {
	*T
	GetCreatedByUserID() string
}](rows []P, userID string) []P {
	out := make([]P, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.GetCreatedByUserID() == userID {
			out = append(out, row)
		}
	}
	return out
}

// listCreatedBy keeps the rows of list whose creator is userID.
func listCreatedBy[T any, P interface {
	*T
	GetCreatedByUserID() string
}](ctx context.Context, list func(context.Context) ([]P, error), userID string) ([]P, error) {
	rows, err := list(ctx)
	if err != nil {
		return nil, err
	}
	return createdByUser(rows, userID), nil
}

func (r *CommodityRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.Commodity, error) {
	return listCreatedBy(ctx, r.List, userID)
}

func (r *CommodityRegistry) ListTrashedCreatedBy(_ context.Context, userID string) ([]*models.Commodity, error) {
	return r.trashedItems(func(c *models.Commodity) bool { return c.CreatedByUserID == userID }), nil
}

func (r *CommodityRegistry) ListRefs(ctx context.Context) ([]registry.EntityRef, error) {
	live, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	trashed := r.trashedItems(func(*models.Commodity) bool { return true })
	out := make([]registry.EntityRef, 0, len(live)+len(trashed))
	for _, c := range append(live, trashed...) {
		out = append(out, registry.EntityRef{ID: c.ID, UUID: c.UUID, Name: c.Name})
	}
	return out, nil
}

func (r *FileRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.FileEntity, error) {
	return listCreatedBy(ctx, r.List, userID)
}

func (r *FileRegistry) ListTrashedCreatedBy(_ context.Context, userID string) ([]*models.FileEntity, error) {
	return r.trashedFiles(func(f *models.FileEntity, _ []models.FileLink) bool {
		return f.CreatedByUserID == userID
	}), nil
}

// The loans, schedules and service records of items in Trash are listed
// too: the embedded registry's List keeps them.

func (r *CommodityLoanRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.CommodityLoan, error) {
	return listCreatedBy(ctx, r.Registry.List, userID)
}

func (r *MaintenanceScheduleRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.MaintenanceSchedule, error) {
	return listCreatedBy(ctx, r.Registry.List, userID)
}

func (r *CommodityServiceRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.CommodityService, error) {
	return listCreatedBy(ctx, r.Registry.List, userID)
}

func (r *CommodityReminderRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.CommodityReminder, error) {
	return listCreatedBy(ctx, r.List, userID)
}

func (r *SupplyLinkRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.SupplyLink, error) {
	return listCreatedBy(ctx, r.List, userID)
}

func (r *CommodityDraftRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.CommodityDraft, error) {
	return listCreatedBy(ctx, r.List, userID)
}

func (r *CommodityDraftRegistry) ListRefs(ctx context.Context) ([]registry.EntityRef, error) {
	drafts, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]registry.EntityRef, 0, len(drafts))
	for _, d := range drafts {
		out = append(out, registry.EntityRef{ID: d.ID, UUID: d.UUID})
	}
	return out, nil
}

func (r *CommodityEventRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.CommodityEvent, error) {
	return listCreatedBy(ctx, r.List, userID)
}

func (r *AreaRegistry) ListRefs(ctx context.Context) ([]registry.AreaRef, error) {
	areas, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]registry.AreaRef, 0, len(areas))
	for _, a := range areas {
		out = append(out, registry.AreaRef{EntityRef: registry.EntityRef{ID: a.ID, UUID: a.UUID, Name: a.Name}, LocationID: a.LocationID})
	}
	return out, nil
}

func (r *LocationRegistry) ListRefs(ctx context.Context) ([]registry.EntityRef, error) {
	locations, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]registry.EntityRef, 0, len(locations))
	for _, l := range locations {
		out = append(out, registry.EntityRef{ID: l.ID, UUID: l.UUID, Name: l.Name})
	}
	return out, nil
}
