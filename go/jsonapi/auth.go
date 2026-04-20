package jsonapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var _ render.Binder = (*UpdateProfileRequest)(nil)

// UpdateProfileRequest is the body for PUT /auth/me.
//
// Name must always be supplied (the endpoint enforces a non-blank profile name).
//
// DefaultGroupID (#1263) is tri-state: absent, null, or a UUID. The
// DefaultGroupIDSet flag distinguishes "leave the stored value alone" (key
// absent) from "clear the default" (key present as null/empty). A plain *string
// collapses the first two cases into nil, which would silently clear a user's
// preference every time the profile form is saved without touching the field —
// hard to spot and hard to recover from.
type UpdateProfileRequest struct {
	Name string `json:"name"`

	// DefaultGroupID is nil when the request wants to clear the preference.
	// Non-nil holds the target group UUID.
	//
	// The tag uses "default_group_id,omitempty" rather than "-" so the
	// Swagger schema advertises the field (the API contract has to be
	// discoverable for frontend/API consumers). Custom UnmarshalJSON below
	// overrides the default decoding anyway, so the `omitempty` only affects
	// outgoing marshalling — which is never performed on request types.
	DefaultGroupID *string `json:"default_group_id,omitempty"`
	// DefaultGroupIDSet records whether "default_group_id" appeared in the
	// JSON body at all. When false, the handler must leave the stored value
	// untouched (back-compat with callers that only know about "name").
	DefaultGroupIDSet bool `json:"-"`
}

// UnmarshalJSON implements custom decoding so the handler can tell apart
// "key absent" (leave DB value alone) from "key = null" (clear the value).
// Decoding into a map[string]json.RawMessage first is the cleanest way to
// detect key presence — *json.RawMessage collapses absent and null into nil.
func (req *UpdateProfileRequest) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	if nameRaw, ok := raw["name"]; ok {
		var s string
		if err := json.Unmarshal(nameRaw, &s); err != nil {
			return err
		}
		req.Name = s
	}

	if defRaw, ok := raw["default_group_id"]; ok {
		req.DefaultGroupIDSet = true
		trimmed := bytes.TrimSpace(defRaw)
		if bytes.Equal(trimmed, []byte("null")) {
			req.DefaultGroupID = nil
			return nil
		}
		var s string
		if err := json.Unmarshal(defRaw, &s); err != nil {
			return err
		}
		s = strings.TrimSpace(s)
		if s == "" {
			// Empty string is treated the same as null — clear the preference.
			req.DefaultGroupID = nil
			return nil
		}
		req.DefaultGroupID = &s
	}
	return nil
}

// Bind implements render.Binder. It normalizes and validates the request fields.
// The Name field is trimmed of surrounding whitespace before validation.
func (req *UpdateProfileRequest) Bind(r *http.Request) error {
	req.Name = strings.TrimSpace(req.Name)
	return req.ValidateWithContext(r.Context())
}

// ValidateWithContext validates the UpdateProfileRequest using the established
// validation pattern. Name must not be blank and must not exceed 100 characters.
// When DefaultGroupID is provided (non-nil), it must look like a UUID so we fail
// fast before hitting the DB; membership checks happen in the handler because
// they need registry access.
func (req *UpdateProfileRequest) ValidateWithContext(ctx context.Context) error {
	fields := []*validation.FieldRules{
		validation.Field(&req.Name, rules.NotEmpty, validation.Length(1, 100)),
	}
	if req.DefaultGroupIDSet && req.DefaultGroupID != nil {
		fields = append(fields, validation.Field(&req.DefaultGroupID, validation.By(validateUUIDPointer)))
	}
	return validation.ValidateStructWithContext(ctx, req, fields...)
}

// validateUUIDPointer enforces that a *string holds a parseable UUID. We can't
// rely on validation/is.UUID without pulling govalidator into go.mod, and the
// rest of the codebase already depends on google/uuid.
func validateUUIDPointer(value any) error {
	ptr, ok := value.(*string)
	if !ok || ptr == nil {
		return nil
	}
	if _, err := uuid.Parse(*ptr); err != nil {
		return validation.NewError("validation_invalid_uuid", "must be a valid UUID")
	}
	return nil
}
