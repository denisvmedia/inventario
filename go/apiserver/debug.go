package apiserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/debug"
)

type debugAPI struct {
	debugInfo *debug.Info
}

// getDebugInfo returns debug information about the application configuration.
// @Summary Get debug information
// @Description get debug information about file storage, database driver, and operating system
// @Tags debug
// @Accept  json
// @Produce  json
// @Success 200 {object} DebugInfo "OK"
// @Router /debug [get]
func (api *debugAPI) getDebugInfo(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	// Set the content type to application/json
	w.Header().Set("Content-Type", "application/json")

	// Return the debug information
	if err := json.NewEncoder(w).Encode(api.debugInfo); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Debug returns a handler for debug information.
func Debug(params Params) func(r chi.Router) {
	api := &debugAPI{
		debugInfo: params.DebugInfo,
	}

	return func(r chi.Router) {
		r.Get("/", api.getDebugInfo) // GET /debug
	}
}
