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

	// Timeout is the upstream provider deadline. The handler injects
	// it via context, but the service also enforces it server-side
	// against the audit-write path.
	Timeout time.Duration
}

// AllowedMIMETypes is the closed list of image MIME types accepted by
// CommodityScanService. The set matches what every supported provider
// can handle — HEIC/HEIF are included because phones still upload them
// even when "share as JPEG" is the default.
var AllowedMIMETypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/heic": true,
	"image/heif": true,
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

// Scan runs the full flow. tenantID and userID identify the caller —
// they are written verbatim into the audit row and consulted for the
// rate limiter. The error result is always one of the declared
// sentinels (or nil); the audit row is written before return on every
// path.
func (s *CommodityScanService) Scan(ctx context.Context, tenantID, userID string, in ScanInput) (*aivision.ScanResult, error) {
	if tenantID == "" || userID == "" {
		return nil, errxtrace.Classify(ErrScanProviderError)
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
		case errors.Is(providerErr, aivision.ErrProviderUnavailable),
			errors.Is(providerErr, aivision.ErrProviderAuth):
			audit.Status = models.CommodityScanStatusError
			audit.ErrorCode = "commodity_scan.provider_unavailable"
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

// validateInput runs the per-photo guards and records the relevant
// audit row + sentinel on rejection.
func (s *CommodityScanService) validateInput(ctx context.Context, tenantID, userID string, in ScanInput, totalBytes int) error {
	if len(in.Photos) == 0 {
		s.writeAudit(ctx, models.CommodityScanAudit{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenantID, UserID: userID},
			Provider:                s.providerName(),
			Status:                  models.CommodityScanStatusError,
			ErrorCode:               "commodity_scan.no_photos",
		})
		return errxtrace.Classify(ErrScanNoPhotos)
	}

	if s.cfg.MaxPhotos > 0 && len(in.Photos) > s.cfg.MaxPhotos {
		s.writeAudit(ctx, models.CommodityScanAudit{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenantID, UserID: userID},
			Provider:                s.providerName(),
			PhotoCount:              clampInt16(len(in.Photos)),
			TotalPhotoBytes:         clampInt32(totalBytes),
			Status:                  models.CommodityScanStatusError,
			ErrorCode:               "commodity_scan.too_many_photos",
		})
		return errxtrace.Classify(ErrScanTooManyPhotos)
	}

	for _, p := range in.Photos {
		if !AllowedMIMETypes[p.ContentType] {
			s.writeAudit(ctx, models.CommodityScanAudit{
				TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenantID, UserID: userID},
				Provider:                s.providerName(),
				PhotoCount:              clampInt16(len(in.Photos)),
				TotalPhotoBytes:         clampInt32(totalBytes),
				Status:                  models.CommodityScanStatusError,
				ErrorCode:               "commodity_scan.unsupported_mime",
			})
			return errxtrace.Classify(ErrScanUnsupportedMIME)
		}
		if s.cfg.MaxPhotoBytes > 0 && len(p.Data) > s.cfg.MaxPhotoBytes {
			s.writeAudit(ctx, models.CommodityScanAudit{
				TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenantID, UserID: userID},
				Provider:                s.providerName(),
				PhotoCount:              clampInt16(len(in.Photos)),
				TotalPhotoBytes:         clampInt32(totalBytes),
				Status:                  models.CommodityScanStatusError,
				ErrorCode:               "commodity_scan.photo_too_large",
			})
			return errxtrace.Classify(ErrScanPhotoTooLarge)
		}
	}
	return nil
}

// checkRateLimit counts recent audit rows and rejects when the per-user
// hourly cap is hit. The cap is read from CommodityScanConfig; zero
// disables the limiter.
func (s *CommodityScanService) checkRateLimit(ctx context.Context, tenantID, userID string, photoCount, totalBytes int) error {
	if s.cfg.RateLimitPerHour <= 0 {
		return nil
	}
	since := s.now().Add(-1 * time.Hour)
	count, err := s.audit.CountRecentForUser(ctx, userID, since)
	if err != nil {
		// Fail open on counter error — observability wins over a hard
		// block. Log loudly so operators notice an actual outage of
		// the audit store.
		slog.Error("commodity scan rate-limit counter failed; allowing request", "error", err.Error(), "user_id", userID)
		return nil
	}
	if count >= s.cfg.RateLimitPerHour {
		s.writeAudit(ctx, models.CommodityScanAudit{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenantID, UserID: userID},
			Provider:                s.providerName(),
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
