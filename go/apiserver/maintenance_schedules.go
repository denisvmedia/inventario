package apiserver

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

const maintenanceScheduleCtxKey ctxValueKey = "maintenance_schedule"

func maintenanceScheduleFromContext(ctx context.Context) *models.MaintenanceSchedule {
	schedule, ok := ctx.Value(maintenanceScheduleCtxKey).(*models.MaintenanceSchedule)
	if !ok {
		return nil
	}
	return schedule
}

// maintenanceScheduleCtx loads the schedule referenced by the
// {scheduleID} URL param into the request context. When mounted under
// a /commodities/{commodityID}/maintenance/{scheduleID} prefix it also
// enforces that the schedule belongs to the named commodity (404 on
// mismatch — same hardening as loanCtx).
func maintenanceScheduleCtx() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			regSet := RegistrySetFromContext(r.Context())
			if regSet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}
			scheduleID := chi.URLParam(r, "scheduleID")
			schedule, err := regSet.MaintenanceScheduleRegistry.Get(r.Context(), scheduleID)
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			if commodityID := chi.URLParam(r, "commodityID"); commodityID != "" && schedule.CommodityID != commodityID {
				renderEntityError(w, r, registry.ErrNotFound)
				return
			}
			ctx := context.WithValue(r.Context(), maintenanceScheduleCtxKey, schedule)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type maintenanceSchedulesAPI struct {
	factorySet *registry.FactorySet
	service    *services.MaintenanceScheduleService
	clock      func() time.Time
}

func newMaintenanceSchedulesAPI(params Params) *maintenanceSchedulesAPI {
	return &maintenanceSchedulesAPI{
		factorySet: params.FactorySet,
		service:    services.NewMaintenanceScheduleService(params.FactorySet),
		clock:      time.Now,
	}
}

// listForCommodity returns all schedules for the commodity in the URL,
// ordered by next_due_at ascending.
//
// @Summary List maintenance schedules for a commodity
// @Description All maintenance schedules for the commodity in the URL.
// @Tags maintenance_schedules
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Success 200 {object} jsonapi.MaintenanceSchedulesResponse "OK"
// @Router /g/{groupSlug}/commodities/{commodityID}/maintenance [get].
func (api *maintenanceSchedulesAPI) listForCommodity(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	commodityID := chi.URLParam(r, "commodityID")
	schedules, err := regSet.MaintenanceScheduleRegistry.ListByCommodity(r.Context(), commodityID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewMaintenanceSchedulesResponse(schedules, len(schedules))); err != nil {
		internalServerError(w, r, err)
	}
}

// createForCommodity opens a new maintenance schedule for the commodity
// in the URL.
//
// @Summary Create a maintenance schedule
// @Description Create a new schedule for the commodity in the URL.
// @Tags maintenance_schedules
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param schedule body jsonapi.MaintenanceScheduleRequest true "Maintenance schedule attributes"
// @Success 201 {object} jsonapi.MaintenanceScheduleResponse "Schedule created"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities/{commodityID}/maintenance [post].
func (api *maintenanceSchedulesAPI) createForCommodity(w http.ResponseWriter, r *http.Request) {
	commodityID := chi.URLParam(r, "commodityID")

	var input jsonapi.MaintenanceScheduleRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Enabled defaults to true at the BE — pointer captures
	// "omitted vs explicit false".
	enabled := true
	if input.Data.Attributes.Enabled != nil {
		enabled = *input.Data.Attributes.Enabled
	}

	schedule := models.MaintenanceSchedule{
		CommodityID:  commodityID,
		Title:        input.Data.Attributes.Title,
		IntervalDays: input.Data.Attributes.IntervalDays,
		NextDueAt:    input.Data.Attributes.NextDueAt,
		LastDoneAt:   input.Data.Attributes.LastDoneAt,
		Notes:        input.Data.Attributes.Notes,
		Enabled:      enabled,
	}

	created, err := api.service.Create(r.Context(), schedule, api.clock())
	if err != nil {
		if errors.Is(err, services.ErrCommodityNotTrackable) {
			unprocessableEntityError(w, r, err)
			return
		}
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewMaintenanceScheduleResponse(created).WithStatusCode(http.StatusCreated)); err != nil {
		internalServerError(w, r, err)
	}
}

// update patches a schedule's mutable fields.
//
// @Summary Update a maintenance schedule
// @Description Patch title / interval / next_due_at / last_done_at / notes / enabled.
// @Tags maintenance_schedules
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param scheduleID path string true "Maintenance schedule ID"
// @Param schedule body jsonapi.MaintenanceScheduleUpdateRequest true "Schedule patch payload"
// @Success 200 {object} jsonapi.MaintenanceScheduleResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Schedule not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/maintenance/{scheduleID} [patch].
func (api *maintenanceSchedulesAPI) update(w http.ResponseWriter, r *http.Request) {
	schedule := maintenanceScheduleFromContext(r.Context())
	if schedule == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.MaintenanceScheduleUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	updated, err := api.service.Update(r.Context(), schedule.ID, services.MaintenanceScheduleUpdate{
		Title:        input.Data.Attributes.Title,
		IntervalDays: input.Data.Attributes.IntervalDays,
		NextDueAt:    input.Data.Attributes.NextDueAt,
		LastDoneAt:   input.Data.Attributes.LastDoneAt,
		Notes:        input.Data.Attributes.Notes,
		Enabled:      input.Data.Attributes.Enabled,
	})
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewMaintenanceScheduleResponse(updated)); err != nil {
		internalServerError(w, r, err)
	}
}

// markDone advances a schedule's next_due_at by interval_days from the
// supplied (or default-today) date and records last_done_at.
//
// @Summary Mark a maintenance schedule as done
// @Description Advance next_due_at by interval_days and record last_done_at.
// @Tags maintenance_schedules
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param scheduleID path string true "Maintenance schedule ID"
// @Param payload body jsonapi.MaintenanceScheduleDoneRequest false "Optional explicit done_at"
// @Success 200 {object} jsonapi.MaintenanceScheduleResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Schedule not found"
// @Router /g/{groupSlug}/maintenance/{scheduleID}/done [post].
func (api *maintenanceSchedulesAPI) markDone(w http.ResponseWriter, r *http.Request) {
	schedule := maintenanceScheduleFromContext(r.Context())
	if schedule == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.MaintenanceScheduleDoneRequest
	// Bind whenever a real body exists. The earlier `ContentLength != 0`
	// gate silently skipped binding for chunked / unknown-length payloads
	// (ContentLength == -1), so an explicit `done_at` could be dropped on
	// the floor. The Binder itself short-circuits when Data is nil, so a
	// truly empty body still reaches the default-today path.
	hasBody := r.Body != nil && r.Body != http.NoBody &&
		(r.ContentLength > 0 || len(r.TransferEncoding) > 0)
	if hasBody {
		if err := render.Bind(r, &input); err != nil {
			unprocessableEntityError(w, r, err)
			return
		}
	}
	var doneAt models.PDate
	if input.Data != nil {
		doneAt = input.Data.Attributes.DoneAt
	}

	updated, err := api.service.MarkDone(r.Context(), schedule.ID, doneAt, api.clock())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewMaintenanceScheduleResponse(updated)); err != nil {
		internalServerError(w, r, err)
	}
}

// remove permanently removes a maintenance schedule row.
//
// @Summary Delete a maintenance schedule
// @Description Hard-delete a maintenance schedule row.
// @Tags maintenance_schedules
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param scheduleID path string true "Maintenance schedule ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.Errors "Schedule not found"
// @Router /g/{groupSlug}/maintenance/{scheduleID} [delete].
func (api *maintenanceSchedulesAPI) remove(w http.ResponseWriter, r *http.Request) {
	schedule := maintenanceScheduleFromContext(r.Context())
	if schedule == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	if err := regSet.MaintenanceScheduleRegistry.Delete(r.Context(), schedule.ID); err != nil {
		renderEntityError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listGroup returns the group-wide upcoming maintenance list, ordered
// by next_due_at ascending. Supports ?due_before=YYYY-MM-DD and
// ?enabled_only=true filters.
//
// @Summary List group-wide maintenance schedules
// @Description Upcoming maintenance across the current group.
// @Tags maintenance_schedules
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param due_before query string false "Filter to schedules whose next_due_at is on or before this date"
// @Param enabled_only query bool false "Restrict to enabled schedules"
// @Param page query int false "Page number (1-based)" default(1)
// @Param per_page query int false "Items per page" default(50)
// @Success 200 {object} jsonapi.MaintenanceScheduleListResponse "OK"
// @Router /g/{groupSlug}/maintenance [get].
func (api *maintenanceSchedulesAPI) listGroup(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))
	offset := (page - 1) * perPage

	opts := registry.MaintenanceListOptions{
		DueBefore:   q.Get("due_before"),
		EnabledOnly: q.Get("enabled_only") == "true",
	}

	schedules, total, err := regSet.MaintenanceScheduleRegistry.ListPaginated(r.Context(), offset, perPage, opts)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	commoditiesByID := make(map[string]*models.Commodity, len(schedules))
	for _, s := range schedules {
		if _, ok := commoditiesByID[s.CommodityID]; ok {
			continue
		}
		c, cerr := regSet.CommodityRegistry.Get(r.Context(), s.CommodityID)
		if cerr != nil {
			if errors.Is(cerr, registry.ErrNotFound) {
				commoditiesByID[s.CommodityID] = nil
				continue
			}
			renderEntityError(w, r, cerr)
			return
		}
		commoditiesByID[s.CommodityID] = c
	}

	setPaginationHeaders(w, page, perPage, total)
	if err := render.Render(w, r, jsonapi.NewMaintenanceScheduleListResponse(schedules, total, commoditiesByID)); err != nil {
		internalServerError(w, r, err)
	}
}

// CommodityMaintenance returns the chi sub-router mounted under the
// per-commodity prefix `/commodities/{commodityID}/maintenance`.
func CommodityMaintenance(params Params) func(r chi.Router) {
	api := newMaintenanceSchedulesAPI(params)
	return func(r chi.Router) {
		r.Get("/", api.listForCommodity)
		r.Post("/", api.createForCommodity)
	}
}

// GroupMaintenance returns the chi sub-router for the group-wide
// /maintenance surface — list, plus per-row PATCH / DELETE / done.
func GroupMaintenance(params Params) func(r chi.Router) {
	api := newMaintenanceSchedulesAPI(params)
	return func(r chi.Router) {
		r.Get("/", api.listGroup)
		r.Route("/{scheduleID}", func(r chi.Router) {
			r.Use(maintenanceScheduleCtx())
			r.Patch("/", api.update)
			r.Delete("/", api.remove)
			r.Post("/done", api.markDone)
		})
	}
}
