package apiserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const (
	locationCtxKey  ctxValueKey = "location"
	commodityCtxKey ctxValueKey = "commodity"
	areaCtxKey      ctxValueKey = "area"
	entityIDKey     ctxValueKey = "entityID"
)

func locationFromContext(ctx context.Context) *models.Location {
	location, ok := ctx.Value(locationCtxKey).(*models.Location)
	if !ok {
		return nil
	}
	return location
}

func commodityFromContext(ctx context.Context) *models.Commodity {
	commodity, ok := ctx.Value(commodityCtxKey).(*models.Commodity)
	if !ok {
		return nil
	}
	return commodity
}

func entityIDFromContext(ctx context.Context) string {
	entityID, ok := ctx.Value(entityIDKey).(string)
	if !ok {
		return ""
	}
	return entityID
}

func commodityCtx(commodityRegistry registry.CommodityRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			commodityID := chi.URLParam(r, "commodityID")

			// Add debug logging for CI debugging
			slog.Info("CommodityCtx: Loading commodity", "commodity_id", commodityID, "method", r.Method, "path", r.URL.Path)

			comReg, err := commodityRegistry.WithCurrentUser(r.Context())
			if err != nil {
				slog.Error("CommodityCtx: Failed to get user-aware registry", "error", err, "commodity_id", commodityID)
				renderEntityError(w, r, err)
				return
			}

			commodity, err := comReg.Get(r.Context(), commodityID)
			if err != nil {
				slog.Error("CommodityCtx: Failed to get commodity", "error", err, "commodity_id", commodityID, "method", r.Method, "comReg_type", fmt.Sprintf("%T", comReg))
				renderEntityError(w, r, err)
				return
			}

			slog.Info("CommodityCtx: Successfully loaded commodity", "commodity_id", commodityID, "commodity_name", commodity.Name)

			ctx := context.WithValue(r.Context(), commodityCtxKey, commodity)
			ctx = context.WithValue(ctx, entityIDKey, commodityID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func locationCtx(locationRegistry registry.LocationRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			locationID := chi.URLParam(r, "locationID")
			locReg, err := locationRegistry.WithCurrentUser(r.Context())
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			location, err := locReg.Get(r.Context(), locationID)
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), locationCtxKey, location)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func areaCtx(areaRegistry registry.AreaRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			areaReg, err := areaRegistry.WithCurrentUser(r.Context())
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			areaID := chi.URLParam(r, "areaID")
			area, err := areaReg.Get(r.Context(), areaID)
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), areaCtxKey, area)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
