package apiserver

import (
	"net/http"

	"github.com/go-chi/render"

	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/services"
)

// bulkUpdateFiles adds tags to and removes tags from several files.
// @Summary Bulk-update files
// @Description Add tags to and remove tags from the files whose ids (at most 100) the body lists. Each file is saved as PUT /files/{id} saves it.
// @Description `tags.add` takes slugs or the text of a new tag, which is created as a single edit creates it; `tags.remove` takes slugs.
// @Description Tags the patch does not name stay on each file. A tag cannot be in both lists, and each list holds at most 50 tags.
// @Description
// @Description The response lists, per file, whether it changed (succeeded), already had what the patch sets (skipped) or could not be changed (failed, with a code and the reason). A retry can send the failed ids alone.
// @Tags files
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.FileBulkUpdateRequest true "The files and the patch"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 422 {object} jsonapi.Errors "Bad request body or patch"
// @Router /g/{groupSlug}/files/bulk-update [post].
func (api *filesAPI) bulkUpdateFiles(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	var input jsonapi.FileBulkUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes

	patch, err := services.ParseFileTagPatch(attrs.Patch.Tags.Add, attrs.Patch.Tags.Remove)
	if err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	if err := api.bulkService.EnsureTags(r.Context(), &patch); err != nil {
		renderEntityError(w, r, err)
		return
	}
	renderBulkOutcome(w, r, "files", api.bulkService.UpdateTags(r.Context(), regSet.FileRegistry, attrs.IDs, patch))
}
