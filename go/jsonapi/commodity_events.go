package jsonapi

import (
	"net/http"
	"time"

	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/models"
)

// CommodityEventActor is the human-readable identity of the user who
// triggered an event. Resolved server-side from the row's
// created_by_user_id so the FE doesn't have to N+1 the user registry.
//
// Email is exposed alongside Name because Name can be empty-ish if the
// user hasn't filled it in; the FE falls back to email for display in
// that case.
type CommodityEventActor struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Email string `json:"email,omitempty"`
}

// CommodityEventListItem is a single row in the events stream returned by
// GET /commodities/{id}/events. The row mirrors the project's "FLAT in
// data" envelope (event fields hoisted to the top level) plus a `meta`
// block with the resolved actor.
type CommodityEventListItem struct {
	ID          string                       `json:"id"`
	Type        string                       `json:"type" example:"commodity_events" enums:"commodity_events"`
	CommodityID string                       `json:"commodity_id"`
	Kind        models.CommodityEventKind    `json:"kind"`
	OccurredAt  time.Time                    `json:"occurred_at"`
	Before      models.CommodityEventPayload `json:"before,omitempty"`
	After       models.CommodityEventPayload `json:"after,omitempty"`
	Note        string                       `json:"note,omitempty"`
	Meta        *CommodityEventListItemMeta  `json:"meta,omitempty"`
}

// CommodityEventListItemMeta is the per-row actor block.
type CommodityEventListItemMeta struct {
	Actor *CommodityEventActor `json:"actor,omitempty"`
}

// CommodityEventsMeta is the meta block on the paginated events list.
type CommodityEventsMeta struct {
	Events int `json:"events" example:"10" format:"int64"`
	Total  int `json:"total" example:"100" format:"int64"`
}

// CommodityEventsResponse is the JSON:API envelope for GET
// /commodities/{id}/events.
type CommodityEventsResponse struct {
	Data []*CommodityEventListItem `json:"data"`
	Meta CommodityEventsMeta       `json:"meta"`
}

// NewCommodityEventsResponse builds the response from a list of events
// and a per-actor identity map. Missing actors render without a meta
// block; the FE falls back to "Unknown user" in that case.
func NewCommodityEventsResponse(events []*models.CommodityEvent, total int, actors map[string]CommodityEventActor) *CommodityEventsResponse {
	items := make([]*CommodityEventListItem, 0, len(events))
	for _, ev := range events {
		item := &CommodityEventListItem{
			ID:          ev.ID,
			Type:        "commodity_events",
			CommodityID: ev.CommodityID,
			Kind:        ev.Kind,
			OccurredAt:  ev.OccurredAt,
			Before:      ev.Before,
			After:       ev.After,
			Note:        ev.Note,
		}
		if actors != nil {
			if a, ok := actors[ev.CreatedByUserID]; ok {
				actor := a
				item.Meta = &CommodityEventListItemMeta{Actor: &actor}
			}
		}
		items = append(items, item)
	}
	return &CommodityEventsResponse{
		Data: items,
		Meta: CommodityEventsMeta{Events: len(items), Total: total},
	}
}

func (*CommodityEventsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}
