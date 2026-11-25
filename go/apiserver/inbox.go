package apiserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/commodityquery"
)

// inboxReasonParam is the query parameter of GET /inbox that keeps the
// entries with one reason.
const inboxReasonParam = "reason"

// inboxAPI serves the inbox of a group (#2794): the records that still need
// attention, from every source of unfinished work. It only reads, so every
// member can open it; each entry is cleared through the record's own
// endpoints, which apply their own role checks.
type inboxAPI struct {
	service *services.InboxService
}

// Inbox mounts /g/{groupSlug}/inbox.
func Inbox() func(r chi.Router) {
	api := &inboxAPI{service: services.NewInboxService()}
	return func(r chi.Router) {
		r.Get("/", api.list)
		r.Get("/count", api.count)
	}
}

func inboxCounts(c services.InboxCounts) jsonapi.InboxCounts {
	out := jsonapi.InboxCounts{Total: c.Total, Reasons: map[string]int{}}
	for _, r := range services.InboxReasons() {
		out.Reasons[string(r)] = 0
	}
	for r, n := range c.Reasons {
		out.Reasons[string(r)] = n
	}
	return out
}

func inboxEntries(entries []services.InboxEntry) []jsonapi.InboxEntry {
	out := make([]jsonapi.InboxEntry, 0, len(entries))
	for _, e := range entries {
		reasons := make([]string, 0, len(e.Reasons))
		for _, r := range e.Reasons {
			reasons = append(reasons, string(r))
		}
		out = append(out, jsonapi.InboxEntry{
			Kind:      string(e.Kind),
			ID:        e.ID,
			Title:     e.Title,
			Reasons:   reasons,
			UpdatedAt: e.UpdatedAt,
			FileCount: e.FileCount,
		})
	}
	return out
}

// list returns a page of the inbox.
//
// @Summary List the inbox
// @Description The records of the group that still need attention, one entry per record with
// @Description every reason it is there for. The default inbox holds unfinished work only: open
// @Description drafts of new items (reason draft, kind draft) and items marked as draft, whose
// @Description review is not complete (reason item_draft, kind commodity). Missing information
// @Description such as no location, no photo or an unknown price does not put an item here.
// @Description Entries are ordered by their first reason in the order draft, item_draft; drafts
// @Description most recently saved first, items most recently added first. meta.entries counts
// @Description the entries the reason filter keeps; meta.counts describes the whole inbox and
// @Description counts each record once in total, whatever its number of reasons. Every member of
// @Description the group can read it.
// @Tags inbox
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param reason query string false "Keep the entries with this reason" Enums(draft, item_draft)
// @Param page query int false "Page number"
// @Param per_page query int false "Entries per page (at most 100)"
// @Success 200 {object} jsonapi.InboxResponse "OK"
// @Failure 400 {object} jsonapi.Errors "Unknown reason (query.invalid_parameter)"
// @Router /g/{groupSlug}/inbox [get].
func (api *inboxAPI) list(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	reason := services.InboxReason(r.URL.Query().Get(inboxReasonParam))
	if reason != "" && !reason.Valid() {
		_ = queryParamError(w, r, &commodityquery.Error{
			Param:  inboxReasonParam,
			Value:  string(reason),
			Reason: "not an inbox reason",
		})
		return
	}
	page, perPage := parsePagination(r.URL.Query().Get("page"), r.URL.Query().Get("per_page"))
	result, err := api.service.List(r.Context(), regSet, services.InboxListOptions{
		Reason: reason,
		Offset: (page - 1) * perPage,
		Limit:  perPage,
	})
	if err != nil {
		_ = renderEntityError(w, r, err)
		return
	}
	setPaginationHeaders(w, page, perPage, result.Total)
	resp := jsonapi.NewInboxResponse(inboxEntries(result.Entries), result.Total, inboxCounts(result.Counts))
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// count returns how many records the inbox holds.
//
// @Summary Count the inbox
// @Description How many records of the group need attention, for the sidebar and the dashboard.
// @Description total counts each record once, whatever its number of reasons; reasons counts,
// @Description per reason, the records that have it. The counts cover only what the group holds,
// @Description so they agree with GET /g/{groupSlug}/inbox.
// @Tags inbox
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} jsonapi.InboxCountResponse "OK"
// @Router /g/{groupSlug}/inbox/count [get].
func (api *inboxAPI) count(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	counts, err := api.service.Count(r.Context(), regSet)
	if err != nil {
		_ = renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewInboxCountResponse(inboxCounts(counts))); err != nil {
		internalServerError(w, r, err)
	}
}
