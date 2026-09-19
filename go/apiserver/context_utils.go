package apiserver

import (
	"net/http"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
)

// GetUserFromRequest extracts user from request context
// Returns nil if no user context is available
func GetUserFromRequest(r *http.Request) *models.User {
	return appctx.UserFromContext(r.Context())
}
