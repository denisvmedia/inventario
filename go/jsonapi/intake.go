package jsonapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
)

// IntakeLimitsResponse carries the limits of a new intake batch.
type IntakeLimitsResponse struct {
	Data IntakeLimitsData `json:"data"`
}

// IntakeLimitsData is the resource object of the limits.
type IntakeLimitsData struct {
	Type       string            `json:"type" example:"intake_limits" enums:"intake_limits"`
	Attributes IntakeLimitsAttrs `json:"attributes"`
}

// IntakeLimitsAttrs are the limits. Enabled is false when no AI scan provider
// is configured; a batch cannot be created then. MaxFilesPerSource is how
// many files one purchase may have, MaxFileBytes the size of one file, Queued
// how many of the group's purchases wait for a scan now, out of
// MaxQueuedPerGroup. RateLimitPerHour is the scans one member may run per
// hour, shared with the scan in the item form; zero means no limit.
type IntakeLimitsAttrs struct {
	Enabled           bool     `json:"enabled" example:"true"`
	MaxFilesPerBatch  int      `json:"max_files_per_batch" example:"50"`
	MaxFilesPerSource int      `json:"max_files_per_source" example:"5"`
	MaxFileBytes      int      `json:"max_file_bytes" example:"10485760"`
	MaxQueuedPerGroup int      `json:"max_queued_per_group" example:"200"`
	Queued            int      `json:"queued" example:"3"`
	RateLimitPerHour  int      `json:"rate_limit_per_hour" example:"30"`
	AcceptedMIMETypes []string `json:"accepted_mime_types" example:"image/jpeg,image/png,application/pdf"`
}

func (*IntakeLimitsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// IntakeSessionView is a session with its counts and the stage it is in.
type IntakeSessionView struct {
	*models.IntakeSession
	State   models.IntakeSessionState   `json:"state" enums:"processing,review,done" example:"review"`
	Summary models.IntakeSessionSummary `json:"summary"`
}

// IntakeSourceFile is a file of a source with its signed URLs.
type IntakeSourceFile struct {
	ID        string            `json:"id"`
	Title     string            `json:"title"`
	Path      string            `json:"path"`
	Ext       string            `json:"ext"`
	MIMEType  string            `json:"mime_type"`
	SizeBytes int64             `json:"size_bytes"`
	URL       string            `json:"url,omitempty"`
	InlineURL string            `json:"inline_url,omitempty"`
	Thumbs    map[string]string `json:"thumbnails,omitempty"`
}

// IntakeSourceView is a source with its files.
type IntakeSourceView struct {
	*models.IntakeSource
	Files []IntakeSourceFile `json:"files"`
}

// IntakeSessionDetailAttrs is a session with its sources and candidates.
type IntakeSessionDetailAttrs struct {
	IntakeSessionView
	Sources    []IntakeSourceView        `json:"sources"`
	Candidates []*models.IntakeCandidate `json:"candidates"`
}

// IntakeSessionResponse is the JSON:API envelope for one session.
type IntakeSessionResponse struct {
	HTTPStatusCode int                       `json:"-"`
	Data           IntakeSessionResponseData `json:"data"`
	Meta           *IntakeDraftsMeta         `json:"meta,omitempty"`
}

// IntakeSessionResponseData is the resource object of a session.
type IntakeSessionResponseData struct {
	ID         string                   `json:"id"`
	Type       string                   `json:"type" example:"intake_sessions" enums:"intake_sessions"`
	Attributes IntakeSessionDetailAttrs `json:"attributes"`
}

// IntakeDraftsMeta reports what a call that creates drafts did: drafts made
// or found made, products left out, products whose draft failed.
type IntakeDraftsMeta struct {
	Created int `json:"created" example:"2"`
	Skipped int `json:"skipped" example:"1"`
	Failed  int `json:"failed" example:"0"`
}

func (sr *IntakeSessionResponse) WithStatusCode(code int) *IntakeSessionResponse {
	tmp := *sr
	tmp.HTTPStatusCode = code
	return &tmp
}

func (sr *IntakeSessionResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(sr.HTTPStatusCode, http.StatusOK))
	return nil
}

// IntakeSessionsResponse lists the sessions of a group.
type IntakeSessionsResponse struct {
	Data []IntakeSessionView `json:"data"`
	Meta IntakeSessionsMeta  `json:"meta"`
}

// IntakeSessionsMeta is the count block on a list response.
type IntakeSessionsMeta struct {
	Sessions int `json:"sessions" example:"4"`
}

func (*IntakeSessionsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// IntakeCandidateResponse is the JSON:API envelope for one candidate.
type IntakeCandidateResponse struct {
	Data IntakeCandidateResponseData `json:"data"`
}

// IntakeCandidateResponseData is the resource object of a candidate.
type IntakeCandidateResponseData struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type" example:"intake_candidates" enums:"intake_candidates"`
	Attributes models.IntakeCandidate `json:"attributes"`
}

// NewIntakeCandidateResponse builds the envelope of c.
func NewIntakeCandidateResponse(c *models.IntakeCandidate) *IntakeCandidateResponse {
	return &IntakeCandidateResponse{Data: IntakeCandidateResponseData{ID: c.ID, Type: "intake_candidates", Attributes: *c}}
}

func (*IntakeCandidateResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// IntakeSessionCreateRequest is the JSON:API payload for
// POST .../intake/sessions.
type IntakeSessionCreateRequest struct {
	Data *IntakeSessionCreateRequestData `json:"data"`
}

// IntakeSessionCreateRequestData wraps the attributes of a new session.
type IntakeSessionCreateRequestData struct {
	ID         string                        `json:"id,omitempty"`
	Type       string                        `json:"type" example:"intake_sessions" enums:"intake_sessions"`
	Attributes IntakeSessionCreateAttributes `json:"attributes"`
}

// IntakeSessionCreateAttributes carries the client key and the purchases of
// a new batch. Each purchase lists the ids of uploaded files the scan reads
// together; one_item says that its files show one item.
type IntakeSessionCreateAttributes struct {
	ClientKey string                      `json:"client_key" example:"5f0c8a7e-3f43-4f1e-9a43-6a1fbb8f2c11"`
	Sources   []IntakeSourceCreateRequest `json:"sources"`
}

// IntakeSourceCreateRequest is one purchase of a new batch.
type IntakeSourceCreateRequest struct {
	FileIDs []string `json:"file_ids"`
	OneItem bool     `json:"one_item"`
}

func (d *IntakeSessionCreateRequestData) ValidateWithContext(ctx context.Context) error {
	if d.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}
	return validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.Type, validation.Required, validation.In("intake_sessions")),
	)
}

func (cr *IntakeSessionCreateRequest) Bind(r *http.Request) error {
	return cr.ValidateWithContext(r.Context())
}

func (cr *IntakeSessionCreateRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, cr,
		validation.Field(&cr.Data, validation.Required),
	)
}

// IntakeCandidateReviewRequest is the JSON:API payload for
// PATCH .../intake/sessions/{id}/candidates/{candidateID}. Every attribute
// is optional; an absent one keeps its value.
type IntakeCandidateReviewRequest struct {
	Data *IntakeCandidateReviewRequestData `json:"data"`
}

// IntakeCandidateReviewRequestData wraps the review attributes.
type IntakeCandidateReviewRequestData struct {
	Type       string                     `json:"type" example:"intake_candidates" enums:"intake_candidates"`
	Attributes IntakeCandidateReviewAttrs `json:"attributes"`
}

// IntakeCandidateReviewAttrs is a member's review of a detected product:
// whether to keep it, the scanned fields and custom fields that go into its
// draft, and the number of identical units it covers.
type IntakeCandidateReviewAttrs struct {
	Selected       *bool     `json:"selected,omitempty" example:"true"`
	AcceptedFields *[]string `json:"accepted_fields,omitempty"`
	// AcceptedCustomFields are the IDs of the custom fields whose scanned
	// value goes into the draft; each must be one the scan read a value for.
	AcceptedCustomFields *[]string `json:"accepted_custom_fields,omitempty"`
	Quantity             *int      `json:"quantity,omitempty" example:"1"`
}

func (d *IntakeCandidateReviewRequestData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.Type, validation.Required, validation.In("intake_candidates")),
	)
}

func (cr *IntakeCandidateReviewRequest) Bind(r *http.Request) error {
	return cr.ValidateWithContext(r.Context())
}

func (cr *IntakeCandidateReviewRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, cr,
		validation.Field(&cr.Data, validation.Required),
	)
}
