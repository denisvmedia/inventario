package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

const commodityReminderCtxKey ctxValueKey = "commodity_reminder"

func commodityReminderFromContext(ctx context.Context) *models.CommodityReminder {
	reminder, ok := ctx.Value(commodityReminderCtxKey).(*models.CommodityReminder)
	if !ok {
		return nil
	}
	return reminder
}

// commodityReminderCtx loads the reminder named by {reminderID} into the
// request context.
func commodityReminderCtx() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			regSet := RegistrySetFromContext(r.Context())
			if regSet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}
			reminder, err := regSet.CommodityReminderRegistry.Get(r.Context(), chi.URLParam(r, "reminderID"))
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			// The reminder of an item in Trash is read-only until the item
			// is restored, as the item's own routes answer 404 for it.
			if _, err := regSet.CommodityRegistry.Get(r.Context(), reminder.CommodityID); err != nil {
				renderEntityError(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), commodityReminderCtxKey, reminder)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type commodityRemindersAPI struct {
	service *services.CommodityReminderService
	clock   func() time.Time
}

func newCommodityRemindersAPI(params Params) *commodityRemindersAPI {
	return &commodityRemindersAPI{
		service: services.NewCommodityReminderService(params.FactorySet),
		clock:   time.Now,
	}
}

// renderReminderWriteError maps a failed create or update: a validation
// error of the merged reminder is the user's to fix (422), everything else
// goes through the shared mapping.
func renderReminderWriteError(w http.ResponseWriter, r *http.Request, err error) {
	if _, ok := errors.AsType[validation.Errors](err); ok {
		unprocessableEntityError(w, r, err)
		return
	}
	renderEntityError(w, r, err)
}

// listForCommodity returns the reminders of the item in the URL.
//
// @Summary List reminders for a commodity
// @Description The reminders set on the commodity in the URL, ordered by date. Each carries its next occurrence.
// @Tags commodity_reminders
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Success 200 {object} jsonapi.CommodityRemindersResponse "OK"
// @Router /g/{groupSlug}/commodities/{commodityID}/reminders [get].
func (api *commodityRemindersAPI) listForCommodity(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	reminders, err := regSet.CommodityReminderRegistry.ListByCommodity(r.Context(), chi.URLParam(r, "commodityID"))
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCommodityRemindersResponse(reminders, api.clock())); err != nil {
		internalServerError(w, r, err)
	}
}

// createForCommodity adds a reminder to the item in the URL.
//
// @Summary Create a reminder for a commodity
// @Description Create a dated reminder with one or more lead times and an optional repeat rule.
// @Tags commodity_reminders
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param reminder body jsonapi.CommodityReminderRequest true "Reminder attributes"
// @Success 201 {object} jsonapi.CommodityReminderResponse "Reminder created"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities/{commodityID}/reminders [post].
func (api *commodityRemindersAPI) createForCommodity(w http.ResponseWriter, r *http.Request) {
	var input jsonapi.CommodityReminderRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes
	created, err := api.service.Create(r.Context(), models.CommodityReminder{
		CommodityID:      chi.URLParam(r, "commodityID"),
		Title:            attrs.Title,
		DueOn:            attrs.DueOn,
		RemindDaysBefore: attrs.RemindDaysBefore,
		RepeatUnit:       attrs.RepeatUnit,
		RepeatInterval:   attrs.RepeatInterval,
	})
	if err != nil {
		renderReminderWriteError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCommodityReminderResponse(created, api.clock()).WithStatusCode(http.StatusCreated)); err != nil {
		internalServerError(w, r, err)
	}
}

// update patches a reminder.
//
// @Summary Update a reminder
// @Description Patch the title, date, lead times or repeat rule of a reminder.
// @Tags commodity_reminders
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param reminderID path string true "Reminder ID"
// @Param reminder body jsonapi.CommodityReminderUpdateRequest true "Reminder patch payload"
// @Success 200 {object} jsonapi.CommodityReminderResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Reminder not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/reminders/{reminderID} [patch].
func (api *commodityRemindersAPI) update(w http.ResponseWriter, r *http.Request) {
	reminder := commodityReminderFromContext(r.Context())
	if reminder == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	var input jsonapi.CommodityReminderUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes
	updated, err := api.service.Update(r.Context(), reminder.ID, services.CommodityReminderUpdate{
		Title:            attrs.Title,
		DueOn:            attrs.DueOn,
		RemindDaysBefore: attrs.RemindDaysBefore,
		RepeatUnit:       attrs.RepeatUnit,
		RepeatInterval:   attrs.RepeatInterval,
	})
	if err != nil {
		renderReminderWriteError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCommodityReminderResponse(updated, api.clock())); err != nil {
		internalServerError(w, r, err)
	}
}

// remove deletes a reminder.
//
// @Summary Delete a reminder
// @Description Delete a reminder and its send history.
// @Tags commodity_reminders
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param reminderID path string true "Reminder ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.Errors "Reminder not found"
// @Router /g/{groupSlug}/reminders/{reminderID} [delete].
func (*commodityRemindersAPI) remove(w http.ResponseWriter, r *http.Request) {
	reminder := commodityReminderFromContext(r.Context())
	if reminder == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	if err := regSet.CommodityReminderRegistry.Delete(r.Context(), reminder.ID); err != nil {
		renderEntityError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// CommodityReminders returns the sub-router mounted at
// /commodities/{commodityID}/reminders.
func CommodityReminders(params Params) func(r chi.Router) {
	api := newCommodityRemindersAPI(params)
	return func(r chi.Router) {
		r.Get("/", api.listForCommodity)
		r.Post("/", api.createForCommodity)
	}
}

// GroupReminders returns the sub-router mounted at /reminders: per-row
// PATCH and DELETE.
func GroupReminders(params Params) func(r chi.Router) {
	api := newCommodityRemindersAPI(params)
	return func(r chi.Router) {
		r.Route("/{reminderID}", func(r chi.Router) {
			r.Use(commodityReminderCtx())
			r.Patch("/", api.update)
			r.Delete("/", api.remove)
		})
	}
}

type upcomingAPI struct {
	service *services.UpcomingService
	clock   func() time.Time
}

// list returns the group-wide Upcoming list.
//
// @Summary List upcoming dates in the group
// @Description Item reminders, warranty expiries, loan due dates and maintenance due dates from today up to `days` ahead, plus overdue loans and maintenance, ordered by date.
// @Tags upcoming
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param days query int false "Days ahead to cover (1-365)" default(30)
// @Success 200 {object} jsonapi.UpcomingResponse "OK"
// @Failure 422 {object} jsonapi.Errors "Invalid days value"
// @Router /g/{groupSlug}/upcoming [get].
func (api *upcomingAPI) list(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	days := services.UpcomingDefaultDays
	if raw := r.URL.Query().Get("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > services.UpcomingMaxDays {
			unprocessableEntityError(w, r, errors.New("days must be a whole number between 1 and 365"))
			return
		}
		days = parsed
	}
	entries, err := api.service.List(r.Context(), regSet, api.clock(), days)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	items := make([]*jsonapi.UpcomingItem, 0, len(entries))
	for _, e := range entries {
		item := &jsonapi.UpcomingItem{
			Kind:     string(e.Kind),
			Date:     string(e.Date),
			DaysLeft: e.DaysLeft,
			Title:    e.Title,
			SourceID: e.SourceID,
		}
		if e.Commodity != nil {
			item.Commodity = &jsonapi.MaintenanceCommodityRef{
				ID:        e.Commodity.ID,
				Name:      e.Commodity.Name,
				ShortName: e.Commodity.ShortName,
			}
		}
		items = append(items, item)
	}
	if err := render.Render(w, r, jsonapi.NewUpcomingResponse(items, days)); err != nil {
		internalServerError(w, r, err)
	}
}

// Upcoming returns the sub-router mounted at /upcoming.
func Upcoming() func(r chi.Router) {
	api := &upcomingAPI{service: services.NewUpcomingService(), clock: time.Now}
	return func(r chi.Router) {
		r.Get("/", api.list)
	}
}
