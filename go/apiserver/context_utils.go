package apiserver

import (
	"net/http"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
)

// GetUserFromRequest extracts user from request context
// Returns nil if no user context is available
func GetUserFromRequest(r *http.Request) *models.User {
	return appctx.UserFromContext(r.Context())
}
