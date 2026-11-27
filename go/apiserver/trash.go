package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// trashAPI serves the group's Trash: deleted items and files wait there until
// someone restores them, deletes them for good, or the retention runs out.
//
// Roles: every member sees the Trash; a member who can edit items (user and
// above) can restore; deleting for good, one entry or the whole Trash, needs
// admin or owner, the same role that deletes locations and areas.
//
// Every restore and delete for good writes one audit row (#2893): the member,
// the group, the operator when the member is impersonated, and the items and
// files it restored or deleted. A failed request writes its row too, with
// what it did before failing.
type trashAPI struct {
	trash     *services.TrashService
	audit     services.AuditLogger
	retention time.Duration
}

// logTrash writes the audit row of a Trash request. failedIDs are the entries
// of a bulk request that did not go through; err is the request's error, or
// for a bulk request a summary of its failures. Nil-safe like the other audit
// helpers.
func (api *trashAPI) logTrash(r *http.Request, action string, outcome services.TrashOutcome, failedIDs []string, err error) {
	if api.audit == nil {
		return
	}
	ctx := r.Context()
	user := appctx.UserFromContext(ctx)
	group := appctx.GroupFromContext(ctx)
	if user == nil || group == nil {
		return
	}
	extra := outcome.AuditExtra()
	if len(failedIDs) > 0 {
		failed := failedIDs
		if len(failed) > services.TrashAuditIDLimit {
			failed = failed[:services.TrashAuditIDLimit]
			extra["ids_truncated"] = true
		}
		extra["failed"] = len(failedIDs)
		extra["failed_ids"] = failed
	}
	var errMsg *string
	if err != nil {
		errMsg = new(err.Error())
	}
	api.audit.LogAuth(ctx, services.AuthEvent{
		Action:     action,
		UserID:     new(user.ID),
		TenantID:   new(group.TenantID),
		Success:    err == nil,
		Request:    r,
		ErrMsg:     errMsg,
		EntityType: new("location_group"),
		EntityID:   new(group.ID),
		Extra:      extra,
	})
}

// restoreOutcome is what an item restore brought back: the item when it
// succeeded, and the files restored, also those restored before a failure.
func restoreOutcome(id string, result *services.CommodityRestoreResult, err error) services.TrashOutcome {
	var out services.TrashOutcome
	if result != nil {
		out.FileIDs = result.FileIDs
	}
	if err == nil {
		out.ItemIDs = []string{id}
	}
	return out
}

// bulkError summarizes the failures of a bulk request for its audit row.
func bulkError(failed, total int) error {
	if failed == 0 {
		return nil
	}
	return fmt.Errorf("%d of %d entries failed", failed, total)
}

func (api *trashAPI) renderError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, services.ErrTrashedWithItem) {
		jsErr := NewUnprocessableEntityError(err)
		jsErr.Code = "trash.file_with_item"
		_ = render.Render(w, r, jsonapi.NewErrors(jsErr))
		return
	}
	_ = renderEntityError(w, r, err)
}

// listTrashedCommodities lists the items in Trash.
// @Summary List items in Trash
// @Description Items deleted in this group, most recently deleted first, with
// @Description who deleted them, when they expire and the files that went
// @Description with them.
// @Tags trash
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param page query int false "Page number"
// @Param per_page query int false "Items per page"
// @Success 200 {object} jsonapi.TrashedCommoditiesResponse "OK"
// @Router /g/{groupSlug}/trash/commodities [get].
func (api *trashAPI) listTrashedCommodities(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r.URL.Query().Get("page"), r.URL.Query().Get("per_page"))
	entries, total, err := api.trash.ListTrashedCommodities(r.Context(), (page-1)*perPage, perPage, api.retention)
	if err != nil {
		api.renderError(w, r, err)
		return
	}
	data := make([]jsonapi.TrashedCommodityData, 0, len(entries))
	for _, e := range entries {
		data = append(data, jsonapi.TrashedCommodityData{
			ID:         e.Commodity.ID,
			Type:       "commodities",
			Attributes: e.Commodity,
			Meta: jsonapi.TrashedCommodityMeta{
				TrashEntryMeta: jsonapi.TrashEntryMeta{DeletedByName: e.DeletedByName, ExpiresAt: e.ExpiresAt},
				FileCount:      e.Files.Count,
				FileBytes:      e.Files.Bytes,
			},
		})
	}
	setPaginationHeaders(w, page, perPage, total)
	resp := &jsonapi.TrashedCommoditiesResponse{Data: data, Meta: jsonapi.NewTrashListMeta(total, page, perPage, api.retention)}
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// listTrashedFiles lists the files deleted on their own.
// @Summary List files in Trash
// @Description Files deleted on their own in this group, most recently
// @Description deleted first. Files that went to Trash with an item are
// @Description listed with that item instead.
// @Tags trash
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param page query int false "Page number"
// @Param per_page query int false "Items per page"
// @Success 200 {object} jsonapi.TrashedFilesResponse "OK"
// @Router /g/{groupSlug}/trash/files [get].
func (api *trashAPI) listTrashedFiles(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r.URL.Query().Get("page"), r.URL.Query().Get("per_page"))
	entries, total, err := api.trash.ListTrashedFiles(r.Context(), (page-1)*perPage, perPage, api.retention)
	if err != nil {
		api.renderError(w, r, err)
		return
	}
	files := make([]*models.FileEntity, 0, len(entries))
	for _, e := range entries {
		files = append(files, e.File)
	}
	// The names of the places each file was attached to, for "was in".
	fillLinkNames(r.Context(), RegistrySetFromContext(r.Context()), files...)
	data := make([]jsonapi.TrashedFileData, 0, len(entries))
	for _, e := range entries {
		data = append(data, jsonapi.TrashedFileData{
			ID:         e.File.ID,
			Type:       "files",
			Attributes: e.File,
			Meta:       jsonapi.TrashEntryMeta{DeletedByName: e.DeletedByName, ExpiresAt: e.ExpiresAt},
		})
	}
	setPaginationHeaders(w, page, perPage, total)
	resp := &jsonapi.TrashedFilesResponse{Data: data, Meta: jsonapi.NewTrashListMeta(total, page, perPage, api.retention)}
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// restoreCommodity brings an item back from Trash.
// @Summary Restore an item from Trash
// @Description Brings the item back with its history, loans, service records,
// @Description maintenance schedules and the files that went to Trash with
// @Description it. An item whose area was deleted meanwhile comes back
// @Description without an area (meta.area_removed).
// @Tags trash
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Success 200 {object} jsonapi.CommodityRestoreResponse "Restored item"
// @Failure 404 {object} jsonapi.Errors "Item not in Trash"
// @Router /g/{groupSlug}/trash/commodities/{commodityID}/restore [post].
func (api *trashAPI) restoreCommodity(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "commodityID")
	result, err := api.trash.RestoreCommodity(r.Context(), id)
	api.logTrash(r, services.AuditActionTrashRestore, restoreOutcome(id, result, err), nil, err)
	if err != nil {
		api.renderError(w, r, err)
		return
	}
	resp := jsonapi.NewCommodityRestoreResponse(result.Commodity, jsonapi.CommodityRestoreMeta{
		AreaRemoved:   result.AreaRemoved,
		FilesRestored: result.FilesRestored,
	})
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// bulkRestoreCommodities brings several items back from Trash.
// @Summary Restore items from Trash
// @Description Restores every item whose id appears in the body. The response
// @Description lists the restored ids, the failures, and the restored items
// @Description that came back without an area. A failure's code is not_found
// @Description when the item is not in Trash (restored already, never deleted,
// @Description or not in the group), and failed for any other error.
// @Tags trash
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.BulkIDsRequest true "Item IDs"
// @Success 200 {object} jsonapi.TrashBulkRestoreResponse "Per-id outcome"
// @Failure 422 {object} jsonapi.Errors "Bad request body"
// @Router /g/{groupSlug}/trash/commodities/restore [post].
func (api *trashAPI) bulkRestoreCommodities(w http.ResponseWriter, r *http.Request) {
	var input jsonapi.BulkIDsRequest
	if err := render.Bind(r, &input); err != nil {
		_ = unprocessableEntityError(w, r, err)
		return
	}
	var succeeded, areaRemoved, failedIDs []string
	var failed []jsonapi.BulkResultFail
	var outcome services.TrashOutcome
	for _, id := range input.Data.Attributes.IDs {
		result, err := api.trash.RestoreCommodity(r.Context(), id)
		done := restoreOutcome(id, result, err)
		outcome.ItemIDs = append(outcome.ItemIDs, done.ItemIDs...)
		outcome.FileIDs = append(outcome.FileIDs, done.FileIDs...)
		if err != nil {
			failed = append(failed, bulkResultFail(r.Context(), "commodities", services.BulkFailure{ID: id, Err: err}))
			failedIDs = append(failedIDs, id)
			continue
		}
		succeeded = append(succeeded, id)
		if result.AreaRemoved {
			areaRemoved = append(areaRemoved, id)
		}
	}
	api.logTrash(r, services.AuditActionTrashRestore, outcome, failedIDs, bulkError(len(failedIDs), len(input.Data.Attributes.IDs)))
	if err := render.Render(w, r, jsonapi.NewTrashBulkRestoreResponse("commodities", succeeded, failed, areaRemoved)); err != nil {
		internalServerError(w, r, err)
	}
}

// purgeCommodity deletes an item in Trash for good.
// @Summary Delete an item in Trash for good
// @Description Deletes the item, its history and the files that went to Trash
// @Description with it. A file another item or place still uses is kept.
// @Description Requires the admin or owner role.
// @Tags trash
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Success 204 "No content"
// @Failure 403 {object} jsonapi.Errors "Admin role required"
// @Failure 404 {object} jsonapi.Errors "Item not in Trash"
// @Router /g/{groupSlug}/trash/commodities/{commodityID} [delete].
func (api *trashAPI) purgeCommodity(w http.ResponseWriter, r *http.Request) {
	outcome, err := api.trash.PurgeCommodity(r.Context(), chi.URLParam(r, "commodityID"))
	api.logTrash(r, services.AuditActionTrashPurge, outcome, nil, err)
	if err != nil {
		api.renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// bulkPurgeCommodities deletes several items in Trash for good.
// @Summary Delete items in Trash for good
// @Description Requires the admin or owner role.
// @Tags trash
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.BulkIDsRequest true "Item IDs"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 403 {object} jsonapi.Errors "Admin role required"
// @Failure 422 {object} jsonapi.Errors "Bad request body"
// @Router /g/{groupSlug}/trash/commodities/purge [post].
func (api *trashAPI) bulkPurgeCommodities(w http.ResponseWriter, r *http.Request) {
	api.bulk(w, r, "commodities", services.AuditActionTrashPurge, api.trash.PurgeCommodity)
}

// restoreFile brings a file back from Trash.
// @Summary Restore a file from Trash
// @Description Only a file deleted on its own can be restored here; a file
// @Description that went to Trash with an item comes back with that item.
// @Tags trash
// @Param groupSlug path string true "Group slug"
// @Param fileID path string true "File ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "File not in Trash"
// @Failure 422 {object} jsonapi.Errors "File went to Trash with an item"
// @Router /g/{groupSlug}/trash/files/{fileID}/restore [post].
func (api *trashAPI) restoreFile(w http.ResponseWriter, r *http.Request) {
	outcome, err := fileOutcome(api.trash.RestoreFile)(r.Context(), chi.URLParam(r, "fileID"))
	api.logTrash(r, services.AuditActionTrashRestore, outcome, nil, err)
	if err != nil {
		api.renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// bulkRestoreFiles brings several files back from Trash.
// @Summary Restore files from Trash
// @Tags trash
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.BulkIDsRequest true "File IDs"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 422 {object} jsonapi.Errors "Bad request body"
// @Router /g/{groupSlug}/trash/files/restore [post].
func (api *trashAPI) bulkRestoreFiles(w http.ResponseWriter, r *http.Request) {
	api.bulk(w, r, "files", services.AuditActionTrashRestore, fileOutcome(api.trash.RestoreFile))
}

// purgeFile deletes a file in Trash for good.
// @Summary Delete a file in Trash for good
// @Description Requires the admin or owner role. A file that went to Trash
// @Description with an item is deleted with that item.
// @Tags trash
// @Param groupSlug path string true "Group slug"
// @Param fileID path string true "File ID"
// @Success 204 "No content"
// @Failure 403 {object} jsonapi.Errors "Admin role required"
// @Failure 404 {object} jsonapi.Errors "File not in Trash"
// @Failure 422 {object} jsonapi.Errors "File went to Trash with an item"
// @Router /g/{groupSlug}/trash/files/{fileID} [delete].
func (api *trashAPI) purgeFile(w http.ResponseWriter, r *http.Request) {
	outcome, err := fileOutcome(api.trash.PurgeFile)(r.Context(), chi.URLParam(r, "fileID"))
	api.logTrash(r, services.AuditActionTrashPurge, outcome, nil, err)
	if err != nil {
		api.renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// bulkPurgeFiles deletes several files in Trash for good.
// @Summary Delete files in Trash for good
// @Description Requires the admin or owner role.
// @Tags trash
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.BulkIDsRequest true "File IDs"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 403 {object} jsonapi.Errors "Admin role required"
// @Failure 422 {object} jsonapi.Errors "Bad request body"
// @Router /g/{groupSlug}/trash/files/purge [post].
func (api *trashAPI) bulkPurgeFiles(w http.ResponseWriter, r *http.Request) {
	api.bulk(w, r, "files", services.AuditActionTrashPurge, fileOutcome(api.trash.PurgeFile))
}

// emptyTrash deletes everything in the group's Trash for good.
// @Summary Empty Trash
// @Description Deletes every item and file in the group's Trash for good.
// @Description Requires the admin or owner role.
// @Tags trash
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} jsonapi.EmptyTrashResponse "What was deleted"
// @Failure 403 {object} jsonapi.Errors "Admin role required"
// @Router /g/{groupSlug}/trash [delete].
func (api *trashAPI) emptyTrash(w http.ResponseWriter, r *http.Request) {
	result, err := api.trash.EmptyTrash(r.Context())
	api.logTrash(r, services.AuditActionTrashEmpty, result.TrashOutcome, nil, err)
	if err != nil {
		api.renderError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewEmptyTrashResponse(len(result.ItemIDs), result.FileEntries)); err != nil {
		internalServerError(w, r, err)
	}
}

// trashOp restores or deletes one Trash entry and reports what it did.
type trashOp func(ctx context.Context, id string) (services.TrashOutcome, error)

// fileOutcome adapts a file restore or delete, which affects that file alone.
func fileOutcome(op func(ctx context.Context, id string) error) trashOp {
	return func(ctx context.Context, id string) (services.TrashOutcome, error) {
		if err := op(ctx, id); err != nil {
			return services.TrashOutcome{}, err
		}
		return services.TrashOutcome{FileIDs: []string{id}}, nil
	}
}

// bulk applies op to every id in a BulkIDsRequest body and writes one audit
// row for the request.
func (api *trashAPI) bulk(w http.ResponseWriter, r *http.Request, entityType, action string, op trashOp) {
	var input jsonapi.BulkIDsRequest
	if err := render.Bind(r, &input); err != nil {
		_ = unprocessableEntityError(w, r, err)
		return
	}
	var succeeded, failedIDs []string
	var failed []jsonapi.BulkResultFail
	var outcome services.TrashOutcome
	for _, id := range input.Data.Attributes.IDs {
		done, err := op(r.Context(), id)
		outcome.ItemIDs = append(outcome.ItemIDs, done.ItemIDs...)
		outcome.FileIDs = append(outcome.FileIDs, done.FileIDs...)
		if err != nil {
			failed = append(failed, jsonapi.BulkResultFail{ID: id, Error: err.Error()})
			failedIDs = append(failedIDs, id)
			continue
		}
		succeeded = append(succeeded, id)
	}
	api.logTrash(r, action, outcome, failedIDs, bulkError(len(failedIDs), len(input.Data.Attributes.IDs)))
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, jsonapi.NewBulkResultResponse(entityType, succeeded, failed)); err != nil {
		internalServerError(w, r, err)
	}
}

// Trash mounts the Trash routes. groupService backs the admin gate on the
// routes that delete for good; audit records every restore and delete for
// good.
func Trash(params Params, groupService *services.GroupService, audit services.AuditLogger) func(r chi.Router) {
	retention := params.TrashRetention
	if retention <= 0 {
		retention = services.DefaultTrashRetention
	}
	api := &trashAPI{
		trash:     services.NewTrashService(params.FactorySet, params.UploadLocation),
		audit:     audit,
		retention: retention,
	}
	adminOnly := requireGroupRole(groupService, models.GroupRoleAdmin)

	return func(r chi.Router) {
		r.With(adminOnly).Delete("/", api.emptyTrash)
		r.Route("/commodities", func(r chi.Router) {
			r.Get("/", api.listTrashedCommodities)
			r.Post("/restore", api.bulkRestoreCommodities)
			r.With(adminOnly).Post("/purge", api.bulkPurgeCommodities)
			r.Post("/{commodityID}/restore", api.restoreCommodity)
			r.With(adminOnly).Delete("/{commodityID}", api.purgeCommodity)
		})
		r.Route("/files", func(r chi.Router) {
			r.Get("/", api.listTrashedFiles)
			r.Post("/restore", api.bulkRestoreFiles)
			r.With(adminOnly).Post("/purge", api.bulkPurgeFiles)
			r.Post("/{fileID}/restore", api.restoreFile)
			r.With(adminOnly).Delete("/{fileID}", api.purgeFile)
		})
	}
}
