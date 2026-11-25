package apiserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// JSON:API error codes of the intake endpoints (#2810).
const (
	// IntakeQueueFullCode: the group's scan queue cannot take the batch.
	IntakeQueueFullCode = "intake.queue_full"
	// IntakeSourceBusyCode: the source is being scanned and cannot be
	// rescanned now.
	IntakeSourceBusyCode = "intake.source_busy"
	// IntakeSourceHasDraftsCode: a product of the source has its draft, so a
	// rescan would orphan it.
	IntakeSourceHasDraftsCode = "intake.source_has_drafts"
	// IntakeCandidateFinishedCode: the product has its draft; its review can
	// no longer change.
	IntakeCandidateFinishedCode = "intake.candidate_finished"
)

// intakeAPI serves batch intake: files dropped together, scanned in the
// background, and turned into one draft per kept product. Every member can
// read the sessions; changing them needs a role that can edit items.
type intakeAPI struct {
	service *services.IntakeService
	signing *services.FileSigningService
}

// Intake mounts /g/{groupSlug}/intake.
func Intake(params Params) func(r chi.Router) {
	api := &intakeAPI{
		service: services.NewIntakeService(params.FactorySet, params.CommodityScanService, services.IntakeConfig{}),
		signing: services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration),
	}
	return func(r chi.Router) {
		r.Get("/limits", api.limits)
		r.Get("/sessions", api.list)
		r.Post("/sessions", api.create)
		r.Route("/sessions/{sessionID}", func(r chi.Router) {
			r.Get("/", api.get)
			r.Delete("/", api.delete)
			r.Post("/cancel", api.cancel)
			r.Post("/resume", api.resume)
			r.Post("/drafts", api.createDrafts)
			r.Patch("/candidates/{candidateID}", api.reviewCandidate)
			r.Post("/sources/{sourceID}/rescan", api.rescan)
		})
	}
}

// renderIntakeError maps the intake errors to responses.
func renderIntakeError(w http.ResponseWriter, r *http.Request, err error) {
	if jsErr, ok := mapCommodityScanError(err); ok {
		_ = render.Render(w, r, jsonapi.NewErrors(jsErr))
		return
	}
	switch {
	case errors.Is(err, services.ErrIntakeQueueFull):
		_ = codedConflictError(w, r, err, IntakeQueueFullCode, nil)
	case errors.Is(err, registry.ErrIntakeSourceBusy):
		_ = codedConflictError(w, r, err, IntakeSourceBusyCode, nil)
	case errors.Is(err, registry.ErrIntakeSourceHasDrafts):
		_ = codedConflictError(w, r, err, IntakeSourceHasDraftsCode, nil)
	case errors.Is(err, registry.ErrIntakeCandidateFinished):
		_ = codedConflictError(w, r, err, IntakeCandidateFinishedCode, nil)
	default:
		if _, ok := errors.AsType[validation.Errors](err); ok {
			_ = unprocessableEntityError(w, r, err)
			return
		}
		_ = renderEntityError(w, r, err)
	}
}

func sessionView(session *models.IntakeSession, summary models.IntakeSessionSummary) jsonapi.IntakeSessionView {
	return jsonapi.IntakeSessionView{IntakeSession: session, State: summary.State(), Summary: summary}
}

// detailResponse builds the response of a session with signed URLs for its
// files, valid for the requesting browser session.
func (api *intakeAPI) detailResponse(r *http.Request, detail *services.IntakeSessionDetail) *jsonapi.IntakeSessionResponse {
	user := appctx.UserFromContext(r.Context())
	binding := services.ExtractSessionBinding(r)
	attrs := jsonapi.IntakeSessionDetailAttrs{
		IntakeSessionView: sessionView(detail.Session, detail.Summary),
		Sources:           make([]jsonapi.IntakeSourceView, 0, len(detail.Sources)),
		Candidates:        detail.Candidates,
	}
	if attrs.Candidates == nil {
		attrs.Candidates = []*models.IntakeCandidate{}
	}
	for _, src := range detail.Sources {
		view := jsonapi.IntakeSourceView{IntakeSource: src.Source, Files: make([]jsonapi.IntakeSourceFile, 0, len(src.Files))}
		for _, file := range src.Files {
			f := jsonapi.IntakeSourceFile{
				ID:        file.ID,
				Title:     file.Title,
				SizeBytes: file.SizeBytes,
			}
			if file.File != nil {
				f.Path = file.Path
				f.Ext = file.Ext
				f.MIMEType = file.MIMEType
				if user != nil {
					if url, thumbs, err := api.signing.GenerateSignedURLsWithThumbnails(file, user.ID, binding); err == nil {
						f.URL = url
						f.Thumbs = thumbs
						f.InlineURL = bestEffortInlineURL(api.signing, file, user.ID, binding)
					}
				}
			}
			view.Files = append(view.Files, f)
		}
		attrs.Sources = append(attrs.Sources, view)
	}
	return &jsonapi.IntakeSessionResponse{
		Data: jsonapi.IntakeSessionResponseData{ID: detail.Session.ID, Type: "intake_sessions", Attributes: attrs},
	}
}

func (api *intakeAPI) renderDetail(w http.ResponseWriter, r *http.Request, detail *services.IntakeSessionDetail, status int) {
	if err := render.Render(w, r, api.detailResponse(r, detail).WithStatusCode(status)); err != nil {
		internalServerError(w, r, err)
	}
}

// limits returns the limits of a new batch.
//
// @Summary Get intake limits
// @Description The limits of a new intake batch and how full the group's scan queue is.
// @Description enabled is false when no AI scan provider is configured.
// @Tags intake
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} jsonapi.IntakeLimitsResponse "OK"
// @Router /g/{groupSlug}/intake/limits [get].
func (api *intakeAPI) limits(w http.ResponseWriter, r *http.Request) {
	limits, err := api.service.Limits(r.Context())
	if err != nil {
		renderIntakeError(w, r, err)
		return
	}
	resp := &jsonapi.IntakeLimitsResponse{Data: jsonapi.IntakeLimitsData{
		Type: "intake_limits",
		Attributes: jsonapi.IntakeLimitsAttrs{
			Enabled:           limits.Enabled,
			MaxFilesPerBatch:  limits.MaxFilesPerBatch,
			MaxFilesPerSource: limits.MaxFilesPerSource,
			MaxFileBytes:      limits.MaxFileBytes,
			MaxQueuedPerGroup: limits.MaxQueuedPerGroup,
			Queued:            limits.Queued,
			RateLimitPerHour:  limits.RateLimitPerHour,
			AcceptedMIMETypes: limits.AcceptedMIMETypes,
		},
	}}
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// list returns the group's intake sessions.
//
// @Summary List intake sessions
// @Description The group's intake sessions with their counts, newest first.
// @Tags intake
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param page query int false "Page number"
// @Param per_page query int false "Sessions per page (at most 100)"
// @Success 200 {object} jsonapi.IntakeSessionsResponse "OK"
// @Router /g/{groupSlug}/intake/sessions [get].
func (api *intakeAPI) list(w http.ResponseWriter, r *http.Request) {
	page, perPage := parsePagination(r.URL.Query().Get("page"), r.URL.Query().Get("per_page"))
	items, total, err := api.service.ListSessions(r.Context(), (page-1)*perPage, perPage)
	if err != nil {
		renderIntakeError(w, r, err)
		return
	}
	resp := &jsonapi.IntakeSessionsResponse{
		Data: make([]jsonapi.IntakeSessionView, 0, len(items)),
		Meta: jsonapi.IntakeSessionsMeta{Sessions: total},
	}
	for _, item := range items {
		resp.Data = append(resp.Data, sessionView(item.Session, item.Summary))
	}
	setPaginationHeaders(w, page, perPage, total)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// create stores a new batch and queues it for scanning.
//
// @Summary Create an intake session
// @Description Queue uploaded files for the AI scan. Each source is one purchase: the files the
// @Description scan reads together. Files dropped together are separate purchases unless the
// @Description client puts them in one source with one_item set, which keeps a single product.
// @Description The files must be uploaded first (POST /g/{groupSlug}/uploads/files) and be of a
// @Description type and size the scan accepts; GET .../intake/limits gives the limits. client_key
// @Description is the client's identity for the batch: a key the group already has returns that
// @Description session with 200, so a retry after a lost response does not queue the files
// @Description twice. The replay does not compare the sources, so a client takes a new key for
// @Description every batch. Without an AI scan provider the request answers 503
// @Description commodity_scan.provider_disabled; a full queue answers 409 intake.queue_full.
// @Tags intake
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param session body jsonapi.IntakeSessionCreateRequest true "Batch"
// @Success 201 {object} jsonapi.IntakeSessionResponse "Session created"
// @Success 200 {object} jsonapi.IntakeSessionResponse "A session with this client key already exists"
// @Failure 404 {object} jsonapi.Errors "A file was not found"
// @Failure 409 {object} jsonapi.Errors "The scan queue is full"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Failure 503 {object} jsonapi.Errors "No AI scan provider is configured"
// @Router /g/{groupSlug}/intake/sessions [post].
func (api *intakeAPI) create(w http.ResponseWriter, r *http.Request) {
	var input jsonapi.IntakeSessionCreateRequest
	if err := render.Bind(r, &input); err != nil {
		_ = unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes
	sources := make([]services.IntakeSourceRequest, 0, len(attrs.Sources))
	for _, src := range attrs.Sources {
		sources = append(sources, services.IntakeSourceRequest{FileIDs: src.FileIDs, OneItem: src.OneItem})
	}
	detail, created, err := api.service.CreateSession(r.Context(), attrs.ClientKey, sources)
	if err != nil {
		renderIntakeError(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	api.renderDetail(w, r, detail, status)
}

// get returns one session.
//
// @Summary Get an intake session
// @Description One session with its sources, their files and the products the scan detected.
// @Description Reading a session never calls the AI provider.
// @Tags intake
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param sessionID path string true "Session ID"
// @Success 200 {object} jsonapi.IntakeSessionResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Session not found"
// @Router /g/{groupSlug}/intake/sessions/{sessionID} [get].
func (api *intakeAPI) get(w http.ResponseWriter, r *http.Request) {
	detail, err := api.service.GetSession(r.Context(), chi.URLParam(r, "sessionID"))
	if err != nil {
		renderIntakeError(w, r, err)
		return
	}
	api.renderDetail(w, r, detail, http.StatusOK)
}

// delete deletes a session.
//
// @Summary Delete an intake session
// @Description Delete a session. Drafts made from it stay. Its files that nothing else uses go
// @Description to Trash; files a draft or an item uses only lose the link to the session.
// @Tags intake
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param sessionID path string true "Session ID"
// @Success 204 "Deleted"
// @Failure 404 {object} jsonapi.Errors "Session not found"
// @Router /g/{groupSlug}/intake/sessions/{sessionID} [delete].
func (api *intakeAPI) delete(w http.ResponseWriter, r *http.Request) {
	if err := api.service.Delete(r.Context(), chi.URLParam(r, "sessionID")); err != nil {
		renderIntakeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// cancel stops the queued scans of a session.
//
// @Summary Cancel an intake session
// @Description Stop the session's queued scans. Detected products and drafts already made stay;
// @Description a scan that is running still lands. Resume queues the canceled purchases again.
// @Tags intake
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param sessionID path string true "Session ID"
// @Success 200 {object} jsonapi.IntakeSessionResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Session not found"
// @Router /g/{groupSlug}/intake/sessions/{sessionID}/cancel [post].
func (api *intakeAPI) cancel(w http.ResponseWriter, r *http.Request) {
	detail, err := api.service.Cancel(r.Context(), chi.URLParam(r, "sessionID"))
	if err != nil {
		renderIntakeError(w, r, err)
		return
	}
	api.renderDetail(w, r, detail, http.StatusOK)
}

// resume queues the canceled and failed purchases of a session again.
//
// @Summary Resume an intake session
// @Description Queue the session's canceled and failed purchases again. A purchase canceled while
// @Description its scan runs goes back to that scan. Purchases that were scanned are not scanned
// @Description again.
// @Tags intake
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param sessionID path string true "Session ID"
// @Success 200 {object} jsonapi.IntakeSessionResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Session not found"
// @Failure 503 {object} jsonapi.Errors "No AI scan provider is configured"
// @Router /g/{groupSlug}/intake/sessions/{sessionID}/resume [post].
func (api *intakeAPI) resume(w http.ResponseWriter, r *http.Request) {
	detail, err := api.service.Resume(r.Context(), chi.URLParam(r, "sessionID"))
	if err != nil {
		renderIntakeError(w, r, err)
		return
	}
	api.renderDetail(w, r, detail, http.StatusOK)
}

// createDrafts makes one draft per kept product.
//
// @Summary Create drafts from an intake session
// @Description Make one draft for every kept product of the session that has none yet, and mark
// @Description the products left out as skipped. Each draft gets the files of its purchase as
// @Description links, so one receipt serves every item made from it. Repeating the call, from
// @Description another device or after a lost response, makes nothing twice; after a partial
// @Description failure it finishes only what is left. meta counts the drafts made or found made,
// @Description the skipped products and the products whose draft failed.
// @Tags intake
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param sessionID path string true "Session ID"
// @Success 200 {object} jsonapi.IntakeSessionResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Session not found"
// @Router /g/{groupSlug}/intake/sessions/{sessionID}/drafts [post].
func (api *intakeAPI) createDrafts(w http.ResponseWriter, r *http.Request) {
	// A client that goes away must not stop the call between making a draft
	// and recording it on its product.
	result, err := api.service.CreateDrafts(context.WithoutCancel(r.Context()), chi.URLParam(r, "sessionID"))
	if err != nil {
		renderIntakeError(w, r, err)
		return
	}
	resp := api.detailResponse(r, result.Detail)
	resp.Meta = &jsonapi.IntakeDraftsMeta{Created: result.Created, Skipped: result.Skipped, Failed: result.Failed}
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// reviewCandidate stores the review of a detected product.
//
// @Summary Review a detected product
// @Description Keep or leave out a detected product, choose the scanned fields that go into its
// @Description draft, and set the number of identical units it covers. Keeping a product that
// @Description was skipped puts it back among the products to create drafts for. A product that
// @Description has its draft answers 409 intake.candidate_finished.
// @Tags intake
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param sessionID path string true "Session ID"
// @Param candidateID path string true "Candidate ID"
// @Param review body jsonapi.IntakeCandidateReviewRequest true "Review"
// @Success 200 {object} jsonapi.IntakeCandidateResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Product not found"
// @Failure 409 {object} jsonapi.Errors "The product is finished"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/intake/sessions/{sessionID}/candidates/{candidateID} [patch].
func (api *intakeAPI) reviewCandidate(w http.ResponseWriter, r *http.Request) {
	var input jsonapi.IntakeCandidateReviewRequest
	if err := render.Bind(r, &input); err != nil {
		_ = unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes
	cand, err := api.service.ReviewCandidate(r.Context(), chi.URLParam(r, "sessionID"), chi.URLParam(r, "candidateID"),
		registry.IntakeCandidateReview{Selected: attrs.Selected, AcceptedFields: attrs.AcceptedFields, Quantity: attrs.Quantity})
	if err != nil {
		renderIntakeError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewIntakeCandidateResponse(cand)); err != nil {
		internalServerError(w, r, err)
	}
}

// rescan queues one purchase for a new scan.
//
// @Summary Rescan a purchase
// @Description Queue one purchase of the session for a new AI scan on the member's request. Its
// @Description detected products are replaced. A purchase being scanned, also one canceled while
// @Description its scan runs, answers 409 intake.source_busy; one with a product that has its
// @Description draft answers 409 intake.source_has_drafts.
// @Tags intake
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param sessionID path string true "Session ID"
// @Param sourceID path string true "Source ID"
// @Success 200 {object} jsonapi.IntakeSessionResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Purchase not found"
// @Failure 409 {object} jsonapi.Errors "The purchase is being scanned or has drafts"
// @Failure 503 {object} jsonapi.Errors "No AI scan provider is configured"
// @Router /g/{groupSlug}/intake/sessions/{sessionID}/sources/{sourceID}/rescan [post].
func (api *intakeAPI) rescan(w http.ResponseWriter, r *http.Request) {
	detail, err := api.service.Rescan(r.Context(), chi.URLParam(r, "sessionID"), chi.URLParam(r, "sourceID"))
	if err != nil {
		renderIntakeError(w, r, err)
		return
	}
	api.renderDetail(w, r, detail, http.StatusOK)
}
