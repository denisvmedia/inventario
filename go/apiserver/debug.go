package apiserver

import (
	"encoding/json"
	"net/http"

	"github.com/denisvmedia/inventario/debug"
)

type debugAPI struct {
	debugInfo *debug.Info
}

// getDebugInfo returns debug information about the application configuration.
//
// Moved behind the back-office auth plane at /admin/debug (issue #2113, L-4):
// the response leaks operational config (file-storage driver, database driver,
// OS) that must not be readable by an ordinary tenant user. Registered in
// apiserver/admin_routes.go under RequireBackofficeAuth.
// @Summary Get debug information
// @Description get debug information about file storage, database driver, and operating system (back-office only)
// @Tags admin
// @Accept  json
// @Produce  json
// @Success 200 {object} debug.Info "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - back-office authentication required"
// @Router /admin/debug [get]
func (api *debugAPI) getDebugInfo(w http.ResponseWriter, _r *http.Request) { //revive:disable-line:get-return
	// Set the content type to application/json
	w.Header().Set("Content-Type", "application/json")

	// Return the debug information
	if err := json.NewEncoder(w).Encode(api.debugInfo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
