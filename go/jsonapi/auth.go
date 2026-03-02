package jsonapi

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var _ render.Binder = (*UpdateProfileRequest)(nil)

// UpdateProfileRequest is the body for PUT /auth/me.
type UpdateProfileRequest struct {
	Name string `json:"name"`
}

// Bind implements render.Binder. It normalizes and validates the request fields.
// The Name field is trimmed of surrounding whitespace before validation.
func (req *UpdateProfileRequest) Bind(r *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	return req.ValidateWithContext(r.Context())
}

// ValidateWithContext validates the UpdateProfileRequest using the established
// validation pattern. Name must not be blank and must not exceed 100 characters.
func (req *UpdateProfileRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, req,
		validation.Field(&req.Name, rules.NotEmpty, validation.Length(1, 100)),
	)
}

