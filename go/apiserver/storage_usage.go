package apiserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/services"
)

// StorageUsage mounts GET /g/{groupSlug}/storage-usage. The route is
// group-scoped via the parent middleware chain — RegistrySet on the
// context is already filtered to the active group.
func StorageUsage() func(r chi.Router) {
	return func(r chi.Router) {
		r.Get("/", handleStorageUsage)
	}
}

// handleStorageUsage returns per-group storage totals (#1388).
// @Summary Get storage usage for a group
// @Description Returns the per-group blob byte total with a per-category breakdown and the active quota. Used by the Settings → Data & storage card.
// @Tags storage
// @Produce json
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} services.StorageUsage "OK"
// @Failure 401 {string} string "Unauthorized"
// @Failure 500 {string} string "Internal Server Error"
// @Router /g/{groupSlug}/storage-usage [get].
func handleStorageUsage(w http.ResponseWriter, r *http.Request) {
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	usage, err := services.NewStorageUsageService(registrySet.FileRegistry).GetUsage(r.Context())
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	render.JSON(w, r, usage)
}
