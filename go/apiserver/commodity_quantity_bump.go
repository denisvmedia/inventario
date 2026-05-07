package apiserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/services"
)

// quantityBumpBlockerError adapts services.QuantityBumpBlocker to the
// error / json.Marshaler contract jsonapi.Error expects. Each blocker
// becomes one entry in the multi-error 422 response: the FE iterates
// `errors[]` and renders one localised hint per blocker, keyed by
// `kind` (warranty / loan / service).
type quantityBumpBlockerError struct {
	Kind   services.QuantityBumpBlockerKind `json:"kind"`
	Detail string                           `json:"detail"`
	// Source mimics the JSON:API source.pointer convention so the FE can
	// route the message to the right form field (Quantity vs the gated
	// step) without having to special-case `kind`. Always points at
	// /data/attributes/count for now since every blocker fires from the
	// same form input; future blockers might point elsewhere.
	Source quantityBumpSource `json:"source"`
}

type quantityBumpSource struct {
	Pointer string `json:"pointer"`
}

func (e *quantityBumpBlockerError) Error() string {
	return string(e.Kind) + ": " + e.Detail
}

// renderQuantityBumpBlockers emits a JSON:API 422 multi-error envelope —
// one entry per blocker — when a 1 → >1 quantity bump still has open
// per-instance state. Mirrors unprocessableEntityError's shape but
// stacks N errors under `errors[]` so the FE can list every blocker
// at once.
func renderQuantityBumpBlockers(w http.ResponseWriter, r *http.Request, blockers []services.QuantityBumpBlocker) {
	apiErrors := make([]jsonapi.Error, 0, len(blockers))
	for _, b := range blockers {
		blocker := &quantityBumpBlockerError{
			Kind:   b.Kind,
			Detail: b.Detail,
			Source: quantityBumpSource{Pointer: "/data/attributes/count"},
		}
		apiErrors = append(apiErrors, jsonapi.Error{
			Err:            blocker,
			UserError:      json.RawMessage(mustMarshalBlocker(blocker)),
			HTTPStatusCode: http.StatusUnprocessableEntity,
			StatusText:     "Unprocessable Entity",
		})
	}
	envelope := jsonapi.NewErrors(apiErrors...)
	envelope.HTTPStatusCode = http.StatusUnprocessableEntity
	if err := render.Render(w, r, envelope); err != nil {
		internalServerError(w, r, err)
	}
}

// mustMarshalBlocker is a tiny shim that never fails — the only reason
// json.Marshal would error here is a programming bug (e.g. a struct
// loop), which we'd rather see in tests than mask. The fallback returns
// a minimal payload so the response still validates.
func mustMarshalBlocker(b *quantityBumpBlockerError) []byte {
	data, err := json.Marshal(b)
	if err != nil {
		return []byte(`{"kind":"unknown","detail":"failed to marshal blocker"}`)
	}
	return data
}
