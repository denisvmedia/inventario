package services

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"math"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/internal/aivision"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// clampInt16 saturates v into the int16 range. Used when packing
// per-request photo counts into the audit row: the configured
// MaxPhotos cap guarantees the value fits in practice, but a hostile
// caller could send millions of multipart parts before the handler's
// part loop bails; clamping keeps the audit insert from failing on
// overflow.
func clampInt16(v int) int16 {
	switch {
	case v > math.MaxInt16:
		return math.MaxInt16
	case v < math.MinInt16:
		return math.MinInt16
	default:
		return int16(v)
	}
}

// clampInt32 saturates v into the int32 range. Used for photo byte
// totals and latency: same rationale as clampInt16.
func clampInt32(v int) int32 {
	switch {
	case v > math.MaxInt32:
		return math.MaxInt32
	case v < math.MinInt32:
		return math.MinInt32
	default:
		return int32(v)
	}
}

// clampInt32FromInt64 saturates v into the int32 range. Used for
// latency conversions from time.Duration math.
func clampInt32FromInt64(v int64) int32 {
	switch {
	case v > math.MaxInt32:
		return math.MaxInt32
	case v < math.MinInt32:
		return math.MinInt32
	default:
		return int32(v)
	}
}

// Sentinels surfaced by CommodityScanService. The apiserver maps each
// one to a stable HTTP status + JSON:API code in toJSONAPIError so the
// FE can branch deterministically; the service writes an audit row with
// the matching Status before returning.
var (
	// ErrScanRateLimited fires when the per-user hourly cap is hit.
	ErrScanRateLimited = errx.NewSentinel("commodity scan rate limit exceeded")

	// ErrScanTooManyPhotos fires when the request carried more photos
	// than the configured per-call cap.
	ErrScanTooManyPhotos = errx.NewSentinel("commodity scan request contains too many photos")

	// ErrScanPhotoTooLarge fires when at least one photo is over the
	// configured per-photo size cap.
	ErrScanPhotoTooLarge = errx.NewSentinel("commodity scan photo exceeds the configured size limit")

	// ErrScanUnsupportedMIME fires when at least one photo carries a
	// MIME type outside the supported allowlist.
	ErrScanUnsupportedMIME = errx.NewSentinel("commodity scan photo has an unsupported MIME type")

	// ErrScanProviderDisabled fires when the configured provider is
	// "none" — the route is mounted but always returns 503.
	ErrScanProviderDisabled = errx.NewSentinel("commodity scan provider is disabled")

	// ErrScanProviderTimeout fires when the upstream provider exceeded
	// the per-call deadline.
	ErrScanProviderTimeout = errx.NewSentinel("commodity scan provider timed out")

	// ErrScanProviderUnavailable fires when the upstream provider was
	// reachable but returned a 4xx/5xx that's not retryable from here.
	ErrScanProviderUnavailable = errx.NewSentinel("commodity scan provider is unavailable")

	// ErrScanProviderError fires when the upstream provider returned
	// an unparseable response or some other generic failure.
	ErrScanProviderError = errx.NewSentinel("commodity scan provider returned an error")

	// ErrScanNoPhotos fires when the request had zero photos.
	ErrScanNoPhotos = errx.NewSentinel("commodity scan request contains no photos")

	// ErrScanIdentityMissing fires when Scan is invoked without a
	// tenant/user identity — a deployment-wiring bug, not a client
	// error. The handler maps it to 500.
	ErrScanIdentityMissing = errx.NewSentinel("commodity scan called without tenant or user identity")

	// ErrScanProviderMisconfigured fires when the upstream provider
	// rejected the configured API key (401/403). Treated as an internal
	// 500 rather than the 502 ErrScanProviderUnavailable path: the user
	// has no retry that helps, and surfacing "bad gateway" misleads
	// operators when the real cause is a server-side credential rotation
	// or env-var mishap. The aivision package doc explicitly classifies
	// auth failures as server-side misconfig for this reason.
	ErrScanProviderMisconfigured = errx.NewSentinel("commodity scan provider is misconfigured")
)

// CommodityScanConfig carries the runtime tunables read from
// AIVision* config fields. The service applies them server-side
// regardless of what the FE sent.
type CommodityScanConfig struct {
	// MaxPhotos is the cap on how many photos a single scan request
	// can carry. Zero means "no limit"; production deployments should
	// always set this.
	MaxPhotos int

	// MaxPhotoBytes is the cap per photo. Zero means "no limit".
	MaxPhotoBytes int

	// RateLimitPerHour is the per-user hourly cap. Zero means "no
	// limit"; production deployments should always set this.
	RateLimitPerHour int

	// Timeout is the upstream provider deadline. The service enforces it
	// by wrapping the incoming context; callers may also set their own
	// deadline, in which case whichever deadline expires first wins.
	Timeout time.Duration
}

// AllowedMIMETypes is the closed list of source MIME types accepted by
// CommodityScanService. The set matches what every supported provider
// can handle — HEIC/HEIF are included because phones still upload them
// even when "share as JPEG" is the default.
//
// "image/jpg" is a noncanonical alias some browsers (notably older
// Safari builds and some Android camera intents) emit instead of the
// canonical "image/jpeg". Accepting it here is a one-line ergonomic
// fix that avoids surprising the FE with a 415 on perfectly valid JPEG
// bytes; the providers normalise the byte stream anyway.
//
// "application/pdf" (#1983 Part B) lets a user prefill from a document —
// a receipt, invoice, or manual — not just a product photo. Both real
// providers (Anthropic Messages, OpenAI Chat Completions) accept PDFs
// natively as a document/file content block, so the bytes flow through
// the same pipeline; only the per-vendor content-block shape differs
// (see each provider's buildPayload).
var AllowedMIMETypes = map[string]bool{
	"image/jpeg":          true,
	"image/jpg":           true, // noncanonical alias some browsers emit for JPEGs
	"image/png":           true,
	"image/webp":          true,
	"image/heic":          true,
	"image/heif":          true,
	aivision.PDFMediaType: true, // receipts / invoices / manuals (#1983)
}

// CommodityScanService coordinates the photo-scan flow: rate limit, →
// validate, → provider call, → audit row. The provider is the boundary
// to a specific vendor; the audit registry is the source of truth for
// the rate limiter and operations dashboards.
type CommodityScanService struct {
	provider aivision.Provider
	audit    registry.CommodityScanAuditRegistry
	cfg      CommodityScanConfig
	now      func() time.Time
}

// NewCommodityScanService constructs the service. A nil provider is
// permitted: the route stays mounted and every call returns
// ErrScanProviderDisabled, recording a "disabled" audit row. The
// callers wire this from cfg.AIVisionProvider="none".
func NewCommodityScanService(provider aivision.Provider, audit registry.CommodityScanAuditRegistry, cfg CommodityScanConfig) *CommodityScanService {
	return &CommodityScanService{
		provider: provider,
		audit:    audit,
		cfg:      cfg,
		now:      time.Now,
	}
}

// SetClock overrides the time source. Used by tests to drive the rate
// limiter and CreatedAt deterministically.
func (s *CommodityScanService) SetClock(now func() time.Time) {
	if now != nil {
		s.now = now
	}
}

// ScanInput is the request shape passed into Scan. It deliberately
// mirrors aivision.ScanRequest but in a service-friendly form (the
// caller doesn't need to know about the aivision package).
type ScanInput struct {
	Photos                []ScanPhotoInput
	HintFromUser          string
	PreferredCurrencyCode string
}

// ScanPhotoInput is a single photo plus its detected MIME type.
type ScanPhotoInput struct {
	Filename    string
	ContentType string
	Data        []byte
}

// RecordOversize writes an audit row for the handler-level 413 path
// (body cap or single-part cap exceeded). The handler can't reuse
// validateInput because the photos were never accumulated into a
// ScanInput; this entry point exists so the audit-on-every-outcome
// invariant still holds for the streaming-multipart short-circuit.
//
// Best effort: empty tenantID/userID is a no-op (the row would fail
// the FK constraint).
func (s *CommodityScanService) RecordOversize(ctx context.Context, tenantID, userID string) {
	if s == nil || tenantID == "" || userID == "" {
		return
	}
	s.writeAudit(ctx, models.CommodityScanAudit{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenantID, UserID: userID},
		Provider:                s.providerName(),
		Model:                   s.providerModel(),
		Status:                  models.CommodityScanStatusValidation,
		ErrorCode:               "commodity_scan.photo_too_large",
	})
}

// Scan runs the full flow. tenantID and userID identify the caller —
// they are written verbatim into the audit row and consulted for the
// rate limiter.
//
// Precondition: tenantID and userID MUST be non-empty. The handler
// enforces JWT presence (and therefore a populated user context) before
// calling Scan, so reaching the identity-missing branch is a
// deployment-wiring bug, not a client error. That branch is logged
// (so operators see the loud failure mode) but does NOT write an audit
// row: the audit table has FK constraints onto users / tenants and the
// insert would simply fail. Apart from that single programmer-error
// path, the error result is always one of the declared sentinels (or
// nil) and the audit row is written before return on every path.
func (s *CommodityScanService) Scan(ctx context.Context, tenantID, userID string, in ScanInput) (*aivision.ScanResult, error) {
	if tenantID == "" || userID == "" {
		// Internal-only path: see the doc-comment above. Log loudly so
		// the wiring failure is visible to operators even though no
		// audit row is written (the FK constraint would reject it).
		slog.Error("commodity scan called without tenant or user identity; deployment-wiring bug", "tenant_id_empty", tenantID == "", "user_id_empty", userID == "", "provider", s.providerName())
		return nil, errxtrace.Classify(ErrScanIdentityMissing)
	}

	totalBytes := 0
	for _, p := range in.Photos {
		totalBytes += len(p.Data)
	}

	// 1) Provider-disabled short circuit. Record the row first so the
	// rate limiter still sees the call (an attacker can't spin the
	// endpoint forever by exploiting the disabled-fast-path).
	if s.provider == nil {
		s.writeAudit(ctx, models.CommodityScanAudit{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenantID, UserID: userID},
			Provider:                "none",
			PhotoCount:              clampInt16(len(in.Photos)),
			TotalPhotoBytes:         clampInt32(totalBytes),
			Status:                  models.CommodityScanStatusDisabled,
			ErrorCode:               "commodity_scan.provider_disabled",
		})
		return nil, errxtrace.Classify(ErrScanProviderDisabled)
	}

	// 2) Validation: photo count, sizes, mime types. Each rejection
	// classifies a sentinel and records an audit row with the matching
	// error code.
	if err := s.validateInput(ctx, tenantID, userID, in, totalBytes); err != nil {
		return nil, err
	}

	// 3) Rate limit. Counted *after* validation so a malformed request
	// doesn't burn budget.
	if err := s.checkRateLimit(ctx, tenantID, userID, len(in.Photos), totalBytes); err != nil {
		return nil, err
	}

	// 4) Provider call. The provider already applies its own deadline
	// via the context the handler injected; we re-apply the configured
	// timeout as a server-side guard.
	callCtx := ctx
	if s.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, s.cfg.Timeout)
		defer cancel()
	}

	photos := make([]aivision.PhotoInput, 0, len(in.Photos))
	for _, p := range in.Photos {
		photos = append(photos, aivision.PhotoInput{
			Filename:    p.Filename,
			ContentType: p.ContentType,
			Data:        p.Data,
		})
	}

	start := s.now()
	result, providerErr := s.provider.Scan(callCtx, aivision.ScanRequest{
		Photos:                photos,
		HintFromUser:          in.HintFromUser,
		PreferredCurrencyCode: in.PreferredCurrencyCode,
	})
	latencyMS := clampInt32FromInt64(int64(s.now().Sub(start) / time.Millisecond))

	audit := models.CommodityScanAudit{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenantID, UserID: userID},
		Provider:                s.provider.Name(),
		Model:                   s.provider.Model(),
		PhotoCount:              clampInt16(len(in.Photos)),
		TotalPhotoBytes:         clampInt32(totalBytes),
		LatencyMS:               latencyMS,
	}

	if providerErr != nil {
		switch {
		case errors.Is(providerErr, aivision.ErrProviderTimeout):
			audit.Status = models.CommodityScanStatusTimeout
			audit.ErrorCode = "commodity_scan.provider_timeout"
			s.writeAudit(ctx, audit)
			return nil, errxtrace.Classify(ErrScanProviderTimeout)
		case errors.Is(providerErr, aivision.ErrProviderAuth):
			// Upstream rejected our credentials — a server-side
			// misconfiguration (rotated key, env-var mishap), not a
			// user-recoverable condition. Route to a dedicated sentinel
			// the handler maps to 500 so operators see the right failure
			// mode rather than a misleading "bad gateway" 502.
			audit.Status = models.CommodityScanStatusError
			audit.ErrorCode = "commodity_scan.provider_misconfigured"
			s.writeAudit(ctx, audit)
			return nil, errxtrace.Classify(ErrScanProviderMisconfigured)
		case errors.Is(providerErr, aivision.ErrProviderUnavailable):
			audit.Status = models.CommodityScanStatusError
			// The handler maps ErrScanProviderUnavailable to the
			// "commodity_scan.provider_error" JSON:API code so that
			// audits/analytics correlate with the client response.
			audit.ErrorCode = "commodity_scan.provider_error"
			s.writeAudit(ctx, audit)
			return nil, errxtrace.Classify(ErrScanProviderUnavailable)
		default:
			audit.Status = models.CommodityScanStatusError
			audit.ErrorCode = "commodity_scan.provider_error"
			s.writeAudit(ctx, audit)
			return nil, errxtrace.Classify(ErrScanProviderError)
		}
	}

	if result == nil {
		audit.Status = models.CommodityScanStatusError
		audit.ErrorCode = "commodity_scan.provider_error"
		s.writeAudit(ctx, audit)
		return nil, errxtrace.Classify(ErrScanProviderError)
	}

	audit.Status = models.CommodityScanStatusOK
	audit.TokensUsed = clampInt32(result.UsedTokens)
	// The provider may have a more accurate latency reading (e.g. it
	// excludes JSON marshalling). Prefer it when present.
	if result.LatencyMS > 0 {
		audit.LatencyMS = clampInt32FromInt64(result.LatencyMS)
	}
	if blob, err := json.Marshal(result); err == nil {
		audit.ResultJSON = blob
	}
	s.writeAudit(ctx, audit)

	return result, nil
}

// ScanAnonymous runs the same vision pipeline as Scan but with NO
// identity and NO persistence: it writes no audit row and consults no
// per-user rate limiter. It backs the public, unauthenticated landing-page
// "add your first item" CTA (#1988), where there is no tenant/user to
// attribute a row to and the abuse controls live entirely in the HTTP
// layer (per-IP + global daily cap middleware) plus the feature flag.
//
// The MaxPhotos / MaxPhotoBytes / Timeout caps and the provider-disabled
// short-circuit are still enforced — they are server-side spend guards,
// not identity-scoped. Validation reuses the pure validatePhotos helper so
// the rejection sentinels (and therefore the handler's JSON:API mapping)
// are identical to the authenticated path.
//
// Returns suggestions only; it never creates a commodity or any DB row.
func (s *CommodityScanService) ScanAnonymous(ctx context.Context, in ScanInput) (*aivision.ScanResult, error) {
	// Provider-disabled short circuit. No audit row to write here — the
	// anonymous path is audit-free by design.
	if s.provider == nil {
		return nil, errxtrace.Classify(ErrScanProviderDisabled)
	}

	if err := s.validatePhotos(in); err != nil {
		return nil, err
	}

	callCtx := ctx
	if s.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, s.cfg.Timeout)
		defer cancel()
	}

	photos := make([]aivision.PhotoInput, 0, len(in.Photos))
	for _, p := range in.Photos {
		photos = append(photos, aivision.PhotoInput{
			Filename:    p.Filename,
			ContentType: p.ContentType,
			Data:        p.Data,
		})
	}

	result, providerErr := s.provider.Scan(callCtx, aivision.ScanRequest{
		Photos:                photos,
		HintFromUser:          in.HintFromUser,
		PreferredCurrencyCode: in.PreferredCurrencyCode,
	})
	if providerErr != nil {
		return nil, classifyProviderErr(providerErr)
	}
	if result == nil {
		return nil, errxtrace.Classify(ErrScanProviderError)
	}
	return result, nil
}

// classifyProviderErr maps a provider-layer error to the matching scan
// sentinel. ScanAnonymous calls it directly (it writes no audit row). The
// authenticated Scan keeps its own inline switch because each case also
// writes a per-case audit row (distinct Status + ErrorCode) before returning
// the same sentinel; the two switches are deliberately kept in lock-step so
// the anonymous and authenticated paths classify provider errors identically.
func classifyProviderErr(providerErr error) error {
	switch {
	case errors.Is(providerErr, aivision.ErrProviderTimeout):
		return errxtrace.Classify(ErrScanProviderTimeout)
	case errors.Is(providerErr, aivision.ErrProviderAuth):
		return errxtrace.Classify(ErrScanProviderMisconfigured)
	case errors.Is(providerErr, aivision.ErrProviderUnavailable):
		return errxtrace.Classify(ErrScanProviderUnavailable)
	default:
		return errxtrace.Classify(ErrScanProviderError)
	}
}

// validatePhotos runs the per-photo guards and returns the matching
// sentinel on rejection. It is pure — no context, no audit, no identity —
// so both the authenticated Scan path (which still writes an audit row on
// failure, via validateInput) and the anonymous ScanAnonymous path (which
// writes nothing) can share the exact same validation rules. The returned
// error is one of the declared ErrScan* sentinels (already classified with
// a stacktrace) so callers can route it to the shared handler mapping
// verbatim.
func (s *CommodityScanService) validatePhotos(in ScanInput) error {
	if len(in.Photos) == 0 {
		return errxtrace.Classify(ErrScanNoPhotos)
	}

	if s.cfg.MaxPhotos > 0 && len(in.Photos) > s.cfg.MaxPhotos {
		return errxtrace.Classify(ErrScanTooManyPhotos)
	}

	for _, p := range in.Photos {
		if !AllowedMIMETypes[p.ContentType] {
			return errxtrace.Classify(ErrScanUnsupportedMIME)
		}
		if s.cfg.MaxPhotoBytes > 0 && len(p.Data) > s.cfg.MaxPhotoBytes {
			return errxtrace.Classify(ErrScanPhotoTooLarge)
		}
	}
	return nil
}

// errorCodeForScanSentinel maps a validation sentinel to the JSON:API
// error code recorded in the audit row. Used by validateInput so the
// audit-on-failure write stays in lock-step with validatePhotos.
func errorCodeForScanSentinel(err error) string {
	switch {
	case errors.Is(err, ErrScanNoPhotos):
		return "commodity_scan.no_photos"
	case errors.Is(err, ErrScanTooManyPhotos):
		return "commodity_scan.too_many_photos"
	case errors.Is(err, ErrScanUnsupportedMIME):
		return "commodity_scan.unsupported_mime"
	case errors.Is(err, ErrScanPhotoTooLarge):
		return "commodity_scan.photo_too_large"
	default:
		return "commodity_scan.validation"
	}
}

// validateInput runs the shared per-photo guards (validatePhotos) and,
// on rejection, records the matching audit row with the right error code
// before returning the sentinel. This keeps the authenticated path's
// audit-on-every-outcome invariant intact while the validation rules
// themselves live in the pure validatePhotos helper.
func (s *CommodityScanService) validateInput(ctx context.Context, tenantID, userID string, in ScanInput, totalBytes int) error {
	err := s.validatePhotos(in)
	if err == nil {
		return nil
	}

	audit := models.CommodityScanAudit{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenantID, UserID: userID},
		Provider:                s.providerName(),
		Model:                   s.providerModel(),
		Status:                  models.CommodityScanStatusValidation,
		ErrorCode:               errorCodeForScanSentinel(err),
	}
	// The no-photos row carries no count/bytes (there's nothing to
	// count); every other validation row records what the caller sent so
	// the dashboard can see the offending magnitude.
	if !errors.Is(err, ErrScanNoPhotos) {
		audit.PhotoCount = clampInt16(len(in.Photos))
		audit.TotalPhotoBytes = clampInt32(totalBytes)
	}
	s.writeAudit(ctx, audit)
	return err
}

// checkRateLimit counts recent audit rows and rejects when the per-user
// hourly cap is hit. The cap is read from CommodityScanConfig; zero
// disables the limiter.
func (s *CommodityScanService) checkRateLimit(ctx context.Context, tenantID, userID string, photoCount, totalBytes int) error {
	if s.cfg.RateLimitPerHour <= 0 {
		return nil
	}
	since := s.now().Add(-1 * time.Hour)
	count, err := s.audit.CountRecentForUser(ctx, tenantID, userID, since)
	if err != nil {
		// Fail open on counter error — observability wins over a hard
		// block. Log loudly so operators notice an actual outage of
		// the audit store.
		slog.Error("commodity scan rate-limit counter failed; allowing request", "error", err.Error(), "user_id", userID, "provider", s.providerName())
		return nil
	}
	if count >= s.cfg.RateLimitPerHour {
		s.writeAudit(ctx, models.CommodityScanAudit{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenantID, UserID: userID},
			Provider:                s.providerName(),
			Model:                   s.providerModel(),
			PhotoCount:              clampInt16(photoCount),
			TotalPhotoBytes:         clampInt32(totalBytes),
			Status:                  models.CommodityScanStatusRateLimited,
			ErrorCode:               "commodity_scan.rate_limited",
		})
		return errxtrace.Classify(ErrScanRateLimited)
	}
	return nil
}

// providerName returns the configured provider's Name() or "none" when
// no provider is wired.
func (s *CommodityScanService) providerName() string {
	if s.provider == nil {
		return "none"
	}
	return s.provider.Name()
}

// providerModel returns the configured provider's Model() or "" when
// no provider is wired. Used so the audit row records the exact model
// id that handled (or would have handled) the request.
func (s *CommodityScanService) providerModel() string {
	if s.provider == nil {
		return ""
	}
	return s.provider.Model()
}

// writeAudit persists an audit row, swallowing the error after logging.
// Audit row failures must never bleed into the user response — the
// scan itself is the load-bearing operation; the audit is an
// observability concern.
func (s *CommodityScanService) writeAudit(ctx context.Context, audit models.CommodityScanAudit) {
	if s.audit == nil {
		return
	}
	if _, err := s.audit.Record(ctx, audit); err != nil {
		slog.Error("commodity scan audit write failed", "error", err.Error(), "user_id", audit.UserID, "status", audit.Status)
	}
}
