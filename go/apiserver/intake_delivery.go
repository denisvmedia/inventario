package apiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/render"
	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/apiserver/middleware"
	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/errormarshal"
	"go.5x5.cz/inventario/internal/filekit"
	"go.5x5.cz/inventario/internal/mimekit"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// Error codes of the intake endpoint (#2809).
const (
	// IntakeAPITokenRequiredCode: the endpoint takes an API token, not a
	// browser session.
	IntakeAPITokenRequiredCode = "intake.api_token_required"
	// IntakeDocumentChangedCode: the group has the document from an earlier
	// delivery with other values or another file. Nothing was changed.
	IntakeDocumentChangedCode = "intake.document_changed"
	// IntakeDocumentConflictCode: an intake session the group's record of the
	// document did not make carries the document. Nothing was made.
	IntakeDocumentConflictCode = "intake.document_conflict"
	// IntakeSourceDeliveredCode: the source was delivered by another system
	// and has nothing to scan.
	IntakeSourceDeliveredCode = "intake.source_delivered"
)

// Limits of a delivery besides the file, whose size the upload limit caps.
const (
	// intakeDeliveryMaxFieldBytes caps the fields of a delivery together.
	intakeDeliveryMaxFieldBytes = 64 << 10
	// intakeDeliveryMaxParts caps the parts of a multipart delivery.
	intakeDeliveryMaxParts = 32
	// intakeDeliveryFilePart is the multipart field of the file, the name
	// Paperless-ngx gives the document it attaches to a webhook.
	intakeDeliveryFilePart = "file"
)

var (
	errIntakeAPITokenRequired = errx.NewDisplayable("the intake endpoint takes an API token")
	errIntakeFieldsTooLarge   = errx.NewDisplayable("the delivery's fields exceed the size limit")
	errIntakeJSONObject       = errx.NewDisplayable("the body must be one JSON object")
	errIntakeJSONText         = errx.NewDisplayable("the body must be valid UTF-8")
)

// intakeDeliveryFields maps each field of a delivery to its place in
// services.IntakeDelivery.
var intakeDeliveryFields = map[string]func(*services.IntakeDelivery) *string{
	"title":           func(d *services.IntakeDelivery) *string { return &d.Title },
	"source_system":   func(d *services.IntakeDelivery) *string { return &d.SourceSystem },
	"source_instance": func(d *services.IntakeDelivery) *string { return &d.SourceInstance },
	"document_id":     func(d *services.IntakeDelivery) *string { return &d.DocumentID },
	"source_url":      func(d *services.IntakeDelivery) *string { return &d.SourceURL },
	"source_name":     func(d *services.IntakeDelivery) *string { return &d.SourceName },
	"seller":          func(d *services.IntakeDelivery) *string { return &d.Seller },
	"purchase_date":   func(d *services.IntakeDelivery) *string { return &d.PurchaseDate },
	"amount":          func(d *services.IntakeDelivery) *string { return &d.Amount },
	"currency":        func(d *services.IntakeDelivery) *string { return &d.Currency },
}

// intakeDeliveryAPI serves POST /g/{groupSlug}/intake: another system, such
// as a Paperless-ngx workflow, delivers a document and gets a draft in the
// group's inbox.
type intakeDeliveryAPI struct {
	service *services.IntakeService
	uploads *uploadsAPI
}

func newIntakeDeliveryAPI(params Params) *intakeDeliveryAPI {
	return &intakeDeliveryAPI{
		service: services.NewIntakeService(params.FactorySet, params.CommodityScanService, services.IntakeConfig{}),
		uploads: newUploadsAPI(params),
	}
}

// deliver creates a draft from a document in another system.
// @Summary Deliver a document from another system
// @Description Creates a draft in the group's inbox from a document kept in another system, such as an invoice in Paperless-ngx (#2809). The request needs an API token that may write and a role in the group that can edit items.
// @Description The fields come as a flat JSON object, as form fields, or as the fields of a multipart form whose part "file" carries the document.
// @Description Swagger 2.0 cannot describe a body and form fields together, so the fields are listed once, as the JSON body; a form field has the same name.
// @Description No AI scan runs: the draft holds the values sent, and what is left out stays unknown. The source URL is stored and shown on the draft, never fetched.
// @Description source_system, source_instance and document_id identify the document in the group.
// @Description Delivering it again with the same values and file returns the document it made (200) and makes nothing, also after its draft was discarded or became an item.
// @Description Delivering it with other values is refused with 409 intake.document_changed and changes nothing, also when the first delivery failed half way; sending the first values again finishes it.
// @Description A delivery is refused with 409 intake.document_conflict when an intake session of the group carries the document but the group has no record of the delivery that made it.
// @Tags intake
// @Accept json
// @Accept x-www-form-urlencoded
// @Accept multipart/form-data
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param request body jsonapi.IntakeDeliveryRequest true "The delivered document"
// @Success 201 {object} jsonapi.IntakeDocumentResponse "The document was delivered and its draft made"
// @Success 200 {object} jsonapi.IntakeDocumentResponse "The document was delivered before; nothing was made"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "No API token, a read-only token, a token for another group, or a role that cannot edit items"
// @Failure 409 {object} jsonapi.Errors "intake.document_changed: the document was delivered before with other values; intake.document_conflict: an intake session carries the document without a record of its delivery"
// @Failure 413 {object} jsonapi.Errors "The file or the fields are too large"
// @Failure 415 {object} jsonapi.Errors "Unsupported Media Type"
// @Failure 422 {object} jsonapi.Errors "A field is missing, unknown or malformed"
// @Failure 423 {object} jsonapi.Errors "A currency migration is running in the group"
// @Failure 429 {object} jsonapi.Errors "Too Many Requests"
// @Router /g/{groupSlug}/intake [post].
func (api *intakeDeliveryAPI) deliver(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if appctx.APITokenFromContext(ctx) == nil {
		_ = codedForbiddenError(w, r, errIntakeAPITokenRequired, IntakeAPITokenRequiredCode)
		return
	}
	user := appctx.UserFromContext(ctx)
	if user == nil || user.TenantID == "" {
		_ = internalServerError(w, r, errxtrace.Classify(errx.NewDisplayable("intake requires an authenticated tenant context")))
		return
	}

	delivery, file, err := api.readDelivery(w, r, user.TenantID)
	// The blob is written before the fields are checked, since a multipart
	// form may carry the file first. Whatever does not become a file of the
	// group is deleted, also when the client went away.
	var stored *models.FileEntity
	defer func() {
		if file != nil && stored == nil {
			api.deleteBlob(context.WithoutCancel(ctx), file.BlobKey)
		}
	}()
	if err != nil {
		renderIntakeDeliveryReadError(w, r, err)
		return
	}

	result, err := api.service.Deliver(ctx, delivery, file)
	if result != nil {
		stored = result.File
	}
	if err != nil {
		renderIntakeDeliveryError(w, r, result, err)
		return
	}
	if stored != nil {
		api.uploads.generateThumbnailInline(ctx, stored, user.ID)
	}
	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}
	resp := jsonapi.NewIntakeDocumentResponse(result.Document, result.State).WithStatusCode(status)
	if err := render.Render(w, r, resp); err != nil {
		_ = internalServerError(w, r, err)
	}
}

// handler wraps deliver in the concurrent-upload limit the upload routes
// have, since a delivery can carry a file.
func (api *intakeDeliveryAPI) handler() http.HandlerFunc {
	limited := middleware.SetUploadOperation("intake_delivery")(
		middleware.UploadLimiter(services.NewConcurrentUploadService(services.LoadConcurrentUploadConfig()))(
			http.HandlerFunc(api.deliver)))
	return limited.ServeHTTP
}

func (api *intakeDeliveryAPI) deleteBlob(ctx context.Context, key string) {
	if err := api.uploads.fileService.DeletePhysicalFile(ctx, key); err != nil {
		slog.WarnContext(ctx, "Failed to delete an unused delivered blob", "blob_key", key, "error", err)
	}
}

// errIntakeUnsupportedMediaType refuses a body that is neither JSON nor a
// form.
var errIntakeUnsupportedMediaType = errx.NewDisplayable("the body must be application/json, application/x-www-form-urlencoded or multipart/form-data")

// readDelivery reads the fields and the file of a delivery. The file is
// stored as a blob as it streams in.
func (api *intakeDeliveryAPI) readDelivery(w http.ResponseWriter, r *http.Request, tenantID string) (services.IntakeDelivery, *services.IntakeDeliveredFile, error) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return services.IntakeDelivery{}, nil, &uploadHTTPError{status: http.StatusUnsupportedMediaType, err: errIntakeUnsupportedMediaType}
	}
	fields := intakeFieldReader{}
	switch mediaType {
	case "application/json":
		// The decoder turns invalid UTF-8 into U+FFFD, which would make
		// another document's identity of it, so the body is checked first.
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, intakeDeliveryMaxFieldBytes))
		switch {
		case err != nil:
			return services.IntakeDelivery{}, nil, mapIntakeBodyError(err)
		case !utf8.Valid(body):
			return services.IntakeDelivery{}, nil, &uploadHTTPError{status: http.StatusUnprocessableEntity, err: errIntakeJSONText}
		}
		err = fields.readJSON(bytes.NewReader(body))
		return fields.delivery, nil, err
	case "application/x-www-form-urlencoded":
		r.Body = http.MaxBytesReader(w, r.Body, intakeDeliveryMaxFieldBytes)
		if err := r.ParseForm(); err != nil {
			return services.IntakeDelivery{}, nil, mapIntakeBodyError(err)
		}
		for name, values := range r.PostForm {
			for _, value := range values {
				fields.set(name, value)
			}
		}
		return fields.delivery, nil, fields.err()
	case "multipart/form-data":
		return api.readMultipart(r, tenantID)
	default:
		return services.IntakeDelivery{}, nil, &uploadHTTPError{status: http.StatusUnsupportedMediaType, err: errIntakeUnsupportedMediaType}
	}
}

// readMultipart reads a multipart delivery part by part. The file streams to
// the blob store under the upload size limit; the other parts are fields.
func (api *intakeDeliveryAPI) readMultipart(r *http.Request, tenantID string) (services.IntakeDelivery, *services.IntakeDeliveredFile, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return services.IntakeDelivery{}, nil, &uploadHTTPError{status: http.StatusUnprocessableEntity, err: err}
	}
	fields := intakeFieldReader{}
	var file *services.IntakeDeliveredFile
	budget := int64(intakeDeliveryMaxFieldBytes)
	for parts := 0; ; parts++ {
		part, err := reader.NextPart()
		// The end of the parts is io.EOF itself. A body that stops before
		// the closing delimiter gives an error that wraps io.EOF.
		if err == io.EOF {
			return fields.delivery, file, fields.err()
		}
		if err != nil {
			return fields.delivery, file, mapIntakeBodyError(err)
		}
		if parts >= intakeDeliveryMaxParts {
			return fields.delivery, file, &uploadHTTPError{status: http.StatusRequestEntityTooLarge, err: errIntakeFieldsTooLarge}
		}
		if part.FileName() == "" {
			value, err := io.ReadAll(io.LimitReader(part, budget+1))
			if err != nil {
				return fields.delivery, file, mapIntakeBodyError(err)
			}
			if budget -= int64(len(value)); budget < 0 {
				return fields.delivery, file, &uploadHTTPError{status: http.StatusRequestEntityTooLarge, err: errIntakeFieldsTooLarge}
			}
			fields.set(part.FormName(), string(value))
			continue
		}
		// A field the delivery refuses fails it before the file is stored.
		if err := fields.err(); err != nil {
			return fields.delivery, file, err
		}
		if part.FormName() != intakeDeliveryFilePart || file != nil {
			fields.fail(part.FormName(), validation.NewError("validation_intake_file", "only one file, in the part \"file\", is accepted"))
			return fields.delivery, file, fields.err()
		}
		basename := filekit.UploadFileName(part.FileName())
		saved, err := api.uploads.saveOnePart(r.Context(), part, basename, tenantID, uploadKindFile, mimekit.AllContentTypes())
		if err != nil {
			return fields.delivery, nil, err
		}
		file = &services.IntakeDeliveredFile{
			BlobKey:  saved.FilePath,
			Name:     saved.Basename,
			MIMEType: saved.MIMEType,
			Size:     saved.Size,
			SHA256:   saved.SHA256,
		}
	}
}

// mapIntakeBodyError turns a body over the size limit into 413 and any other
// read error into 422.
func mapIntakeBodyError(err error) error {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return &uploadHTTPError{status: http.StatusRequestEntityTooLarge, err: errIntakeFieldsTooLarge}
	}
	return &uploadHTTPError{status: http.StatusUnprocessableEntity, err: errxtrace.Wrap("failed to read the delivery", err)}
}

// intakeFieldReader collects the fields of a delivery and the problems with
// them: an unknown field and a field given twice are refused.
type intakeFieldReader struct {
	delivery services.IntakeDelivery
	seen     map[string]bool
	errs     validation.Errors
}

func (f *intakeFieldReader) fail(name string, err error) {
	if f.errs == nil {
		f.errs = validation.Errors{}
	}
	f.errs[name] = err
}

func (f *intakeFieldReader) set(name, value string) {
	target, ok := intakeDeliveryFields[name]
	if !ok {
		f.fail(name, validation.NewError("validation_intake_unknown_field", "unknown field"))
		return
	}
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	if f.seen[name] {
		f.fail(name, validation.NewError("validation_intake_repeated_field", "the field is given more than once"))
		return
	}
	f.seen[name] = true
	*target(&f.delivery) = value
}

// readJSON reads a flat object whose values are strings, numbers or null.
// It reads the members one at a time, so a member given twice is refused as
// a repeated form field is, and it refuses anything but white space after
// the object.
func (f *intakeFieldReader) readJSON(body io.Reader) error {
	dec := json.NewDecoder(body)
	dec.UseNumber()
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return intakeJSONError(err)
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return intakeJSONError(err)
		}
		name, _ := tok.(string)
		var value any
		if err := dec.Decode(&value); err != nil {
			return intakeJSONError(err)
		}
		switch v := value.(type) {
		case string:
			f.set(name, v)
		case json.Number:
			if intakeIdentityFields[name] && !intakeIntegerPattern.MatchString(v.String()) {
				// 300 and 300.0 are one number but two document ids.
				f.fail(name, validation.NewError("validation_intake_field_type", "must be a string or an integer"))
				continue
			}
			f.set(name, v.String())
		case nil:
			f.set(name, "")
		default:
			f.fail(name, validation.NewError("validation_intake_field_type", "must be a string"))
		}
	}
	if _, err := dec.Token(); err != nil {
		return intakeJSONError(err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return intakeJSONError(err)
	}
	return f.err()
}

// intakeIdentityFields are the fields that identify the document. A JSON
// number in one must be an integer, whose text is the same however it is
// written.
var intakeIdentityFields = map[string]bool{"source_system": true, "source_instance": true, "document_id": true}

// intakeIntegerPattern leaves out -0, which is the number 0 written as
// other text.
var intakeIntegerPattern = regexp.MustCompile(`^(0|-?[1-9][0-9]*)$`)

// intakeJSONError turns a JSON body over the size limit into 413 and any
// other problem with it, including content after the object, into 422.
func intakeJSONError(err error) error {
	if err != nil {
		if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
			return mapIntakeBodyError(err)
		}
	}
	var cause error = errIntakeJSONObject
	if err != nil {
		cause = errxtrace.Wrap("the body must be one JSON object", err)
	}
	return &uploadHTTPError{status: http.StatusUnprocessableEntity, err: cause}
}

func (f *intakeFieldReader) err() error {
	if len(f.errs) > 0 {
		return f.errs
	}
	return nil
}

func renderIntakeDeliveryReadError(w http.ResponseWriter, r *http.Request, err error) {
	if ue, ok := errors.AsType[*uploadHTTPError](err); ok && ue.status == http.StatusUnsupportedMediaType {
		_ = render.Render(w, r, jsonapi.NewErrors(jsonapi.Error{
			Err:            ue.err,
			UserError:      errormarshal.Marshal(ue.err),
			HTTPStatusCode: http.StatusUnsupportedMediaType,
			StatusText:     "Unsupported Media Type",
		}))
		return
	}
	if _, ok := errors.AsType[validation.Errors](err); ok {
		_ = unprocessableEntityError(w, r, err)
		return
	}
	renderUploadError(w, r, err)
}

func renderIntakeDeliveryError(w http.ResponseWriter, r *http.Request, result *services.IntakeDeliveryResult, err error) {
	if errors.Is(err, services.ErrIntakeDocumentConflict) {
		_ = codedConflictError(w, r, err, IntakeDocumentConflictCode, nil)
		return
	}
	if errors.Is(err, services.ErrIntakeDocumentChanged) {
		// The group may have no record of the first delivery to name.
		var meta map[string]any
		if result != nil && result.Document != nil {
			doc := result.Document
			meta = map[string]any{"document_id": doc.ID}
			if doc.DraftID != nil {
				meta["draft_id"] = *doc.DraftID
			}
			if doc.CommodityID != nil {
				meta["commodity_id"] = *doc.CommodityID
			}
		}
		_ = codedConflictError(w, r, err, IntakeDocumentChangedCode, meta)
		return
	}
	if _, ok := errors.AsType[validation.Errors](err); ok {
		_ = unprocessableEntityError(w, r, err)
		return
	}
	_ = renderEntityError(w, r, err)
}

// intakeDocuments lists the delivered documents of drafts or items, so a draft
// and an item show the system they came from.
// @Summary List delivered documents
// @Description Lists the documents other systems delivered (#2809) whose draft is one of draft_id or whose item is one of commodity_id. Both take a comma-separated list of ids; at least one is required.
// @Tags intake
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param draft_id query string false "Draft ids, comma-separated"
// @Param commodity_id query string false "Item ids, comma-separated"
// @Success 200 {object} jsonapi.IntakeDocumentsResponse "OK"
// @Failure 422 {object} jsonapi.Errors "Neither draft_id nor commodity_id is given"
// @Router /g/{groupSlug}/intake/documents [get].
func (api *intakeAPI) documents(w http.ResponseWriter, r *http.Request) {
	draftIDs := splitIDs(r.URL.Query().Get("draft_id"))
	commodityIDs := splitIDs(r.URL.Query().Get("commodity_id"))
	if len(draftIDs) == 0 && len(commodityIDs) == 0 {
		_ = unprocessableEntityError(w, r, validation.Errors{
			"draft_id": validation.NewError("validation_required", "draft_id or commodity_id is required"),
		})
		return
	}
	if len(draftIDs)+len(commodityIDs) > intakeDocumentsMaxIDs {
		_ = unprocessableEntityError(w, r, validation.Errors{
			"draft_id": validation.NewError("validation_intake_too_many_ids", "at most {{.max}} ids are accepted").
				SetParams(map[string]any{"max": intakeDocumentsMaxIDs}),
		})
		return
	}
	docs, err := api.service.ListDocuments(r.Context(), draftIDs, commodityIDs)
	if err != nil {
		renderIntakeError(w, r, err)
		return
	}
	states, err := api.service.DocumentStates(r.Context(), docs)
	if err != nil {
		renderIntakeError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewIntakeDocumentsResponse(docs, states)); err != nil {
		_ = internalServerError(w, r, err)
	}
}

// intakeDocumentsMaxIDs caps the ids one documents request names, a page of
// the inbox and then some.
const intakeDocumentsMaxIDs = 200

func splitIDs(raw string) []string {
	var ids []string
	for id := range strings.SplitSeq(raw, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
