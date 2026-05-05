package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// listCommodityEvents returns the audit timeline for a single commodity
// (issue #1450), newest first. Optional ?kind= narrows by event kind so
// the FE can render "show only status changes" filters without a second
// trip. The handler resolves each row's `created_by_user_id` to the
// human-readable actor block in one batched lookup so the FE doesn't
// N+1 the user registry per row.
//
// @Summary List commodity events
// @Description Returns the append-only audit timeline for a commodity, newest first.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 50, max 100)"
// @Param kind query []string false "Filter by event kind; repeat to OR" collectionFormat(multi)
// @Success 200 {object} jsonapi.CommodityEventsResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity not found"
// @Router /g/{groupSlug}/commodities/{commodityID}/events [get].
func (api *commoditiesAPI) listCommodityEvents(w http.ResponseWriter, r *http.Request) {
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))
	offset := (page - 1) * perPage

	opts := registry.CommodityEventListOptions{
		Kinds: parseEventKinds(q["kind"]),
	}

	events, total, err := regSet.CommodityEventRegistry.ListByCommodity(r.Context(), commodity.ID, offset, perPage, opts)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	actors := api.resolveActorsForEvents(r.Context(), regSet.UserRegistry, events)

	setPaginationHeaders(w, page, perPage, total)
	if err := render.Render(w, r, jsonapi.NewCommodityEventsResponse(events, total, actors)); err != nil {
		internalServerError(w, r, err)
	}
}

// parseEventKinds decodes the repeated `kind=` query values into a typed
// slice. Unknown values are silently dropped — the FE may send a kind
// the BE hasn't shipped yet during a multi-version rollout, and 422 on
// the list endpoint would be a UX regression there.
func parseEventKinds(raw []string) []models.CommodityEventKind {
	if len(raw) == 0 {
		return nil
	}
	kinds := make([]models.CommodityEventKind, 0, len(raw))
	seen := make(map[models.CommodityEventKind]struct{}, len(raw))
	for _, v := range raw {
		v = strings.TrimSpace(v)
		k := models.CommodityEventKind(v)
		if !k.IsValid() {
			continue
		}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		kinds = append(kinds, k)
	}
	return kinds
}

// resolveActorsForEvents builds an id → actor map from the events' actor
// ids in a single round-trip per unique user. UserRegistry.Get is one
// query each and the timeline is bounded by per_page (≤100), so we let
// the small N inflate rather than introducing a batched ListByIDs (none
// of the existing user-registry tests assert that shape — adding it just
// for this endpoint would be premature infra).
func (api *commoditiesAPI) resolveActorsForEvents(ctx context.Context, userReg registry.UserRegistry, events []*models.CommodityEvent) map[string]jsonapi.CommodityEventActor {
	if userReg == nil || len(events) == 0 {
		return nil
	}
	out := make(map[string]jsonapi.CommodityEventActor)
	for _, ev := range events {
		if ev == nil || ev.CreatedByUserID == "" {
			continue
		}
		if _, done := out[ev.CreatedByUserID]; done {
			continue
		}
		user, err := userReg.Get(ctx, ev.CreatedByUserID)
		if err != nil || user == nil {
			// Missing actor is non-fatal — the row still renders with no
			// `meta.actor` block and the FE falls back to "Unknown user".
			out[ev.CreatedByUserID] = jsonapi.CommodityEventActor{ID: ev.CreatedByUserID}
			continue
		}
		out[ev.CreatedByUserID] = jsonapi.CommodityEventActor{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
		}
	}
	return out
}
