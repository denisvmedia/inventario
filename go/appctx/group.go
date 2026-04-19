package appctx

import (
	"context"

	"github.com/denisvmedia/inventario/models"
)

const (
	groupCtxKey contextKey = "group"
)

// GroupFromContext extracts the location group from the context.
func GroupFromContext(ctx context.Context) *models.LocationGroup {
	group, ok := ctx.Value(groupCtxKey).(*models.LocationGroup)
	if !ok {
		return nil
	}
	return group
}

// WithGroup adds a location group to the context.
func WithGroup(ctx context.Context, group *models.LocationGroup) context.Context {
	return context.WithValue(ctx, groupCtxKey, group)
}

// GroupIDFromContext extracts the group ID from the context.
// Returns empty string if no group is present.
func GroupIDFromContext(ctx context.Context) string {
	group := GroupFromContext(ctx)
	if group == nil {
		return ""
	}
	return group.ID
}
