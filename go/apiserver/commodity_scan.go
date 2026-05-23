package apiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/aivision"
	"github.com/denisvmedia/inventario/internal/errormarshal"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/services"
)

// JSON:API codes the FE branches on for the /commodities/scan endpoint
// (issue #1720). The values are part of the wire contract — adding or
// renaming requires a coordinated FE change.
const (
	commodityScanRateLimitedCode      = "commodity_scan.rate_limited"
	commodityScanTooManyPhotosCode    = "commodity_scan.too_many_photos"
	commodityScanPhotoTooLargeCode    = "commodity_scan.photo_too_large"
	commodityScanUnsupportedMIMECode  = "commodity_scan.unsupported_mime"
	commodityScanProviderDisabledCode = "commodity_scan.provider_disabled"
	commodityScanProviderTimeoutCode  = "commodity_scan.provider_timeout"
	commodityScanProviderErrorCode    = "commodity_scan.provider_error"
	commodityScanNoPhotosCode         = "commodity_scan.no_photos"
)

// scanFormField is the multipart form field name the FE uses for each
// photo. Multiple values are allowed; the handler reads them in the
// order they were appended.
const scanFormField = "photos"

// commodityScanAPI is the small handler struct for POST
// /api/v1/g/{groupSlug}/commodities/scan.
type commodityScanAPI struct {
	service *services.CommodityScanService
	// maxFormBytes caps the entire multipart body. We compute it from
	// the per-photo cap + max photo count + a generous fudge factor
	// (base64 overhead, headers). Zero means "no cap" — the chi
	// router's parent body-size middleware still applies.
	maxFormBytes int64
	// maxPhotoBytes caps a SINGLE part. A request whose total stays
	// inside maxFormBytes but whose individual photo exceeds the
	// per-photo cap is rejected with 413 + commodity_scan.photo_too_large
	// before the entire part is read into memory — the body cap alone
	// would let one hostile part allocate up to maxFormBytes before
	// the service-layer validator catches it.
	maxPhotoBytes int
}

// CommodityScan returns the chi router function that mounts the scan
// endpoint inside the existing /g/{groupSlug}/commodities tree. It is
// kept on its own (not folded into Commodities()) because the multipart
// content-type bypasses the default JSON:API content-type guard and
// because the handler depends on services.CommodityScanService which
// the parent Commodities handler doesn't need.
//
// scanService may be nil — the route stays mounted (so the FE doesn't
// see 404 for a misconfigured deployment) and every call returns
// 503 commodity_scan.provider_disabled.
func CommodityScan(scanService *services.CommodityScanService, maxFormBytes int64, maxPhotoBytes int) func(r chi.Router) {
	api := &commodityScanAPI{
		service:       scanService,
		maxFormBytes:  maxFormBytes,
		maxPhotoBytes: maxPhotoBytes,
	}
	return func(r chi.Router) {
		r.Post("/", api.handleScan)
	}
}

// handleScan is the POST handler. It parses the multipart body,
// validates the basics, and delegates to CommodityScanService.Scan,
// then renders a JSON:API response with type "commodity_scan".
//
// @Summary Run an AI vision scan on uploaded photos
// @Description Extract structured commodity field guesses from 1..N product photos. The handler does not persist any commodity — it only returns structured suggestions for the Add Item dialog to pre-fill.
// @Tags commodities
// @Accept multipart/form-data
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param photos formData file true "Product photo(s); image/jpeg|jpg|png|webp|heic|heif. Repeat the form field to upload up to 5 photos in a single request (multipart/form-data with multiple `photos` parts)."
// @Param hint formData string false "Optional free-form hint (brand, category guess)"
// @Success 200 {object} jsonapi.CommodityScanResponse "OK"
// @Failure 413 {object} jsonapi.Errors "Photo too large"
// @Failure 415 {object} jsonapi.Errors "Unsupported MIME type"
// @Failure 422 {object} jsonapi.Errors "Too many photos / no photos"
// @Failure 429 {object} jsonapi.Errors "Rate limited"
// @Failure 502 {object} jsonapi.Errors "Provider unavailable / parse error"
// @Failure 503 {object} jsonapi.Errors "Provider disabled"
// @Failure 504 {object} jsonapi.Errors "Provider timed out"
// @Router /g/{groupSlug}/commodities/scan [post]
func (api *commodityScanAPI) handleScan(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "User context required", http.StatusInternalServerError)
		return
	}

	// Enforce the body cap before the multipart parse so a malicious
	// caller can't OOM the process by sending an unbounded multipart
	// stream. We intentionally use io.LimitReader rather than
	// http.MaxBytesReader so this handler keeps sole control over the
	// eventual 413 JSON:API response shape.
	if err := api.bufferBody(r); err != nil {
		if errors.As(err, new(errBodyTooLarge)) {
			api.recordOversizeAudit(r.Context(), user.TenantID, user.ID)
			_ = render.Render(w, r, jsonapi.NewErrors(scanError(
				services.ErrScanPhotoTooLarge,
				http.StatusRequestEntityTooLarge,
				"Payload Too Large",
				commodityScanPhotoTooLargeCode,
			)))
			return
		}
		renderScanError(w, r, classifyMultipartReadErr(err))
		return
	}

	in, err := api.readPhotos(r, int64(api.maxPhotoBytes))
	if err != nil {
		// Body-cap or per-part-cap overflows are routed through the
		// service so the audit row is written and the FE banner sees a
		// 413 with the same `commodity_scan.photo_too_large` code the
		// validator emits.
		if errors.As(err, new(errBodyTooLarge)) || errors.As(err, new(errOversizedPart)) {
			api.recordOversizeAudit(r.Context(), user.TenantID, user.ID)
			_ = render.Render(w, r, jsonapi.NewErrors(scanError(
				services.ErrScanPhotoTooLarge,
				http.StatusRequestEntityTooLarge,
				"Payload Too Large",
				commodityScanPhotoTooLargeCode,
			)))
			return
		}
		renderScanError(w, r, err)
		return
	}

	in.PreferredCurrencyCode = preferredCurrencyFromContext(r.Context())

	// The constructor permits a nil service so the route can stay
	// mounted in a "feature gated off" deployment; calling Scan on it
	// would panic. Short-circuit to the typed 503 the FE banner already
	// knows how to render.
	if api.service == nil {
		renderScanError(w, r, services.ErrScanProviderDisabled)
		return
	}

	result, err := api.service.Scan(r.Context(), user.TenantID, user.ID, in)
	if err != nil {
		renderScanError(w, r, err)
		return
	}

	resp := newCommodityScanResponse(result)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// readPhotos walks the multipart stream and collects every part named
// "photos" into a ScanInput. The hint (if present) is read from the
// "hint" form field. Empty parts and unrelated fields are ignored so
// the FE doesn't need to be strict about which extra fields it appends.
//
// `perPartCap` bounds a single part so a hostile caller can't allocate
// more than the per-photo cap before the service-layer validator
// rejects it. Zero disables the per-part cap (the body-level
// MaxBytesReader is still in effect).
func (api *commodityScanAPI) readPhotos(r *http.Request, perPartCap int64) (services.ScanInput, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return services.ScanInput{}, classifyMultipartReadErr(err)
	}

	var in services.ScanInput
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return services.ScanInput{}, classifyMultipartReadErr(err)
		}

		switch part.FormName() {
		case scanFormField:
			data, err := readPartBounded(part, perPartCap)
			_ = part.Close()
			if err != nil {
				return services.ScanInput{}, classifyMultipartReadErr(err)
			}
			if len(data) == 0 {
				continue
			}
			ct := part.Header.Get("Content-Type")
			if ct == "" {
				ct = http.DetectContentType(data)
			}
			in.Photos = append(in.Photos, services.ScanPhotoInput{
				Filename:    part.FileName(),
				ContentType: ct,
				Data:        data,
			})
		case "hint":
			data, err := readPartBounded(part, perPartCap)
			_ = part.Close()
			if err == nil {
				in.HintFromUser = string(data)
			}
		default:
			_ = part.Close()
		}
	}
	return in, nil
}

func (api *commodityScanAPI) bufferBody(r *http.Request) error {
	if api.maxFormBytes <= 0 {
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, api.maxFormBytes+1))
	_ = r.Body.Close()
	if err != nil {
		return err
	}
	if int64(len(body)) > api.maxFormBytes {
		return errBodyTooLarge{}
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	return nil
}

// readPartBounded reads up to `cap+1` bytes from r and reports
// errOversizedPart if the limit was exceeded. Zero `cap` disables the
// guard. Returning errOversizedPart lets readPhotos map a single
// hostile part to ErrScanPhotoTooLarge (413) rather than OOMing.
func readPartBounded(r io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return io.ReadAll(r)
	}
	data, err := io.ReadAll(io.LimitReader(r, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, errOversizedPart{}
	}
	return data, nil
}

// classifyMultipartReadErr maps a multipart-read error into either an
// oversized-part sentinel (audit-worthy 413) or the generic malformed-body
// sentinel (400). The overall body-cap check happens earlier in handleScan
// via bufferBody so the handler can keep sole control over the 413 response.
func classifyMultipartReadErr(err error) error {
	var oversized errOversizedPart
	if errors.As(err, &oversized) {
		return oversized
	}
	return errBadMultipart{cause: err}
}

// preferredCurrencyFromContext reads the active group's currency from
// the request context. Returns "" when no group context is present —
// the prompt simply skips the currency hint in that case.
func preferredCurrencyFromContext(ctx context.Context) string {
	group := appctx.GroupFromContext(ctx)
	if group == nil {
		return ""
	}
	return string(group.GroupCurrency)
}

// errBadMultipart is the sentinel wrapper for multipart-parsing errors.
// The handler maps it to 400 rather than 422 — the body was malformed,
// not a business-rule violation.
type errBadMultipart struct{ cause error }

func (e errBadMultipart) Error() string {
	if e.cause == nil {
		return "malformed multipart body"
	}
	return "malformed multipart body: " + e.cause.Error()
}

// errBodyTooLarge is set when the pre-parse body cap in bufferBody is
// exceeded. Distinct from errBadMultipart so the handler can map it to
// 413 + commodity_scan.photo_too_large rather than the generic 400 path,
// and so the audit row is written.
type errBodyTooLarge struct{}

func (errBodyTooLarge) Error() string { return "multipart body exceeds the configured size limit" }

// errOversizedPart is set when a single multipart part exceeds the
// per-photo cap. Routed to the same 413 path as errBodyTooLarge — the
// FE only differentiates "the upload is too big" from the other scan
// errors, not "which part" was the cause.
type errOversizedPart struct{}

func (errOversizedPart) Error() string {
	return "multipart part exceeds the configured per-photo size limit"
}

// recordOversizeAudit writes a best-effort audit row for the
// body-cap / per-part-cap 413 path. Wraps CommodityScanService.RecordOversize
// so the handler doesn't need to know the audit registry directly.
func (api *commodityScanAPI) recordOversizeAudit(ctx context.Context, tenantID, userID string) {
	if api.service == nil {
		return
	}
	api.service.RecordOversize(ctx, tenantID, userID)
}

// renderScanError maps a sentinel from CommodityScanService into the
// expected JSON:API status + code shape. Unmapped errors fall through
// to the generic 500 path via renderEntityError so they're visible in
// logs.
func renderScanError(w http.ResponseWriter, r *http.Request, err error) {
	var bad errBadMultipart
	if errors.As(err, &bad) {
		_ = badRequest(w, r, err)
		return
	}

	// Identity-missing is a deployment-wiring bug — surface as 500 so
	// operators see the loud failure mode rather than the misleading
	// "provider error" 502 a generic mapping would produce.
	if errors.Is(err, services.ErrScanIdentityMissing) {
		internalServerError(w, r, err)
		return
	}

	// Provider-misconfigured (upstream rejected our API key) is also a
	// server-side problem the user can't help with. Mirror the
	// identity-missing handling so operators get a 500 + the loud log
	// trail rather than a 502 that points at the wrong layer.
	if errors.Is(err, services.ErrScanProviderMisconfigured) {
		internalServerError(w, r, err)
		return
	}

	jsErr, ok := mapCommodityScanError(err)
	if !ok {
		_ = renderEntityError(w, r, err)
		return
	}
	_ = render.Render(w, r, jsonapi.NewErrors(jsErr))
}

// mapCommodityScanError returns the typed JSON:API error for a scan
// sentinel and (false, _) when err is something else. Pulled out so
// the wire mapping is in one place and the test asserts directly
// against the table.
func mapCommodityScanError(err error) (jsonapi.Error, bool) {
	switch {
	case errors.Is(err, services.ErrScanRateLimited):
		return scanError(err, http.StatusTooManyRequests, "Too Many Requests", commodityScanRateLimitedCode), true
	case errors.Is(err, services.ErrScanTooManyPhotos):
		return scanError(err, http.StatusUnprocessableEntity, "Unprocessable Entity", commodityScanTooManyPhotosCode), true
	case errors.Is(err, services.ErrScanPhotoTooLarge):
		return scanError(err, http.StatusRequestEntityTooLarge, "Payload Too Large", commodityScanPhotoTooLargeCode), true
	case errors.Is(err, services.ErrScanUnsupportedMIME):
		return scanError(err, http.StatusUnsupportedMediaType, "Unsupported Media Type", commodityScanUnsupportedMIMECode), true
	case errors.Is(err, services.ErrScanNoPhotos):
		return scanError(err, http.StatusUnprocessableEntity, "Unprocessable Entity", commodityScanNoPhotosCode), true
	case errors.Is(err, services.ErrScanProviderDisabled):
		return scanError(err, http.StatusServiceUnavailable, "Service Unavailable", commodityScanProviderDisabledCode), true
	case errors.Is(err, services.ErrScanProviderTimeout):
		return scanError(err, http.StatusGatewayTimeout, "Gateway Timeout", commodityScanProviderTimeoutCode), true
	case errors.Is(err, services.ErrScanProviderUnavailable),
		errors.Is(err, services.ErrScanProviderError):
		return scanError(err, http.StatusBadGateway, "Bad Gateway", commodityScanProviderErrorCode), true
	}
	return jsonapi.Error{}, false
}

func scanError(err error, status int, statusText, code string) jsonapi.Error {
	return jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: status,
		StatusText:     statusText,
		Code:           code,
	}
}

// commodityScanResponse is the JSON:API envelope for a successful scan.
// Type "commodity_scan" is intentionally distinct from "commodities" so
// the FE knows the body is a *suggestion*, not a persisted commodity.
type commodityScanResponse struct {
	Data       commodityScanResource `json:"data"`
	httpStatus int
}

type commodityScanResource struct {
	Type       string                `json:"type"`
	Attributes commodityScanResultDT `json:"attributes"`
}

// commodityScanResultDT is the wire shape. Each field carries an
// optional value + confidence; absent fields mean "no signal".
type commodityScanResultDT struct {
	Fields     map[string]commodityScanFieldDT `json:"fields"`
	Warnings   []aivision.Warning              `json:"warnings,omitempty"`
	UsedTokens int                             `json:"used_tokens,omitempty"`
	LatencyMS  int64                           `json:"latency_ms,omitempty"`
}

type commodityScanFieldDT struct {
	Value      json.RawMessage `json:"value"`
	Confidence float64         `json:"confidence"`
}

func newCommodityScanResponse(r *aivision.ScanResult) *commodityScanResponse {
	dt := commodityScanResultDT{
		Fields:     make(map[string]commodityScanFieldDT, len(r.Fields)),
		Warnings:   r.Warnings,
		UsedTokens: r.UsedTokens,
		LatencyMS:  r.LatencyMS,
	}
	for name, g := range r.Fields {
		raw, err := json.Marshal(g.Value)
		if err != nil {
			continue
		}
		dt.Fields[name] = commodityScanFieldDT{Value: raw, Confidence: g.Confidence}
	}
	return &commodityScanResponse{
		Data:       commodityScanResource{Type: "commodity_scan", Attributes: dt},
		httpStatus: http.StatusOK,
	}
}

// Render implements render.Renderer.
func (resp *commodityScanResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	if resp.httpStatus == 0 {
		resp.httpStatus = http.StatusOK
	}
	render.Status(r, resp.httpStatus)
	return nil
}

// MarshalJSON is unused but referenced indirectly by render; declared so
// future code that wraps the response with includes/meta can extend
// without changing the wire shape callers already see.
func (resp *commodityScanResponse) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Data commodityScanResource `json:"data"`
	}{Data: resp.Data})
}
