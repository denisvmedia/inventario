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

// entityIDFromContext was the lookup helper for the legacy
// commodity-/location-scoped upload handlers, which kept the parent
// entity id in `entityIDKey` so the handler body could read it back
// without re-parsing the URL. Both the helper and its callers were
// removed under #1421; the surviving `commodityCtx` / `locationCtx`
// middlewares still write the same context value because deletion
// would touch surviving handlers (`getCommodity`, `updateCommodity`,
// `deleteCommodity`, location equivalents) that read the entity from
// their respective `commodityFromContext` / `locationFromContext`
// helpers — leaving the write side intact keeps that wiring local.

func commodityCtx() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			commodityID := chi.URLParam(r, "commodityID")

			// Add debug logging for CI debugging
			slog.Info("CommodityCtx: Loading commodity", "commodity_id", commodityID, "method", r.Method, "path", r.URL.Path)

			regSet := RegistrySetFromContext(r.Context())
			if regSet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}
			comReg := regSet.CommodityRegistry

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
			regSet := RegistrySetFromContext(r.Context())
			if regSet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}
			locReg := regSet.LocationRegistry
			location, err := locReg.Get(r.Context(), locationID)
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), locationCtxKey, location)
			ctx = context.WithValue(ctx, entityIDKey, locationID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func areaCtx(areaRegistry registry.AreaRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			regSet := RegistrySetFromContext(r.Context())
			if regSet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}
			areaReg := regSet.AreaRegistry
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
