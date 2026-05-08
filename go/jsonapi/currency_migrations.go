package jsonapi

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/go-extras/errx"
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
)

// jsonapi types for currency migrations (issue #202 / #1551).
//
// Three families:
//   - CurrencyMigration{,s}Response — read responses (list / get / start)
//   - CurrencyMigrationStartRequest — POST /currency-migrations
//   - CurrencyMigrationPreview{Request,Response} — POST .../preview
//
// Same shape as RestoreOperation* so the FE codegen sees a familiar
// pattern. Type discriminator is "currency-migrations".

// CurrencyMigrationResponse is the single-row response wrapper.
type CurrencyMigrationResponse struct {
	HTTPStatusCode int                            `json:"-"`
	Data           *CurrencyMigrationResponseData `json:"data"`
}

// CurrencyMigrationResponseData wraps a CurrencyMigration as JSON:API attributes.
type CurrencyMigrationResponseData struct {
	ID         string                   `json:"id"`
	Type       string                   `json:"type" example:"currency-migrations" enums:"currency-migrations"`
	Attributes models.CurrencyMigration `json:"attributes"`
}

// CurrencyMigrationsResponse is the multi-row response wrapper.
type CurrencyMigrationsResponse struct {
	HTTPStatusCode int                              `json:"-"`
	Data           []*CurrencyMigrationResponseData `json:"data"`
}

func NewCurrencyMigrationResponse(m *models.CurrencyMigration) *CurrencyMigrationResponse {
	return &CurrencyMigrationResponse{
		HTTPStatusCode: http.StatusOK,
		Data: &CurrencyMigrationResponseData{
			ID:         m.ID,
			Type:       "currency-migrations",
			Attributes: *m,
		},
	}
}

func NewCurrencyMigrationsResponse(operations []*models.CurrencyMigration) *CurrencyMigrationsResponse {
	data := make([]*CurrencyMigrationResponseData, len(operations))
	for i, op := range operations {
		data[i] = &CurrencyMigrationResponseData{
			ID:         op.ID,
			Type:       "currency-migrations",
			Attributes: *op,
		}
	}
	return &CurrencyMigrationsResponse{HTTPStatusCode: http.StatusOK, Data: data}
}

func (r *CurrencyMigrationResponse) WithStatusCode(statusCode int) *CurrencyMigrationResponse {
	tmp := *r
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

func (r *CurrencyMigrationResponse) Render(_w http.ResponseWriter, req *http.Request) error {
	render.Status(req, statusCodeDef(r.HTTPStatusCode, http.StatusOK))
	return nil
}

func (r *CurrencyMigrationsResponse) Render(_w http.ResponseWriter, req *http.Request) error {
	render.Status(req, statusCodeDef(r.HTTPStatusCode, http.StatusOK))
	return nil
}

// CurrencyMigrationStartAttributes is the user-input slice of the start
// request body. Everything not listed here is server-managed (status,
// timing, totals, audit ref). The FE sends the from/to/rate it just
// previewed plus the preview_token it received from the preview
// endpoint.
type CurrencyMigrationStartAttributes struct {
	FromCurrency models.Currency `json:"from_currency"`
	ToCurrency   models.Currency `json:"to_currency"`
	// ExchangeRate is the user-typed rate (1 from = rate to). FE clamps
	// to 6 decimals as a UX guard; BE validates per #202 §2 (positive,
	// finite, ≤ 1e10).
	ExchangeRate decimal.Decimal `json:"exchange_rate"`
	// PreviewToken is the HMAC-signed token the preview endpoint
	// returned. Required; the start handler verifies the signature and
	// then re-derives the state hash to detect group drift between
	// preview and commit.
	PreviewToken string `json:"preview_token"`
}

// CurrencyMigrationStartRequest is the POST /currency-migrations body.
type CurrencyMigrationStartRequest struct {
	Data *CurrencyMigrationStartRequestData `json:"data"`
}

type CurrencyMigrationStartRequestData struct {
	Type       string                            `json:"type" example:"currency-migrations" enums:"currency-migrations"`
	Attributes *CurrencyMigrationStartAttributes `json:"attributes"`
}

func (req *CurrencyMigrationStartRequest) Bind(r *http.Request) error {
	if req.Data == nil {
		return errx.NewDisplayable("missing required data field")
	}
	if req.Data.Type != "currency-migrations" {
		return errx.NewDisplayable("invalid type, expected 'currency-migrations'")
	}
	if req.Data.Attributes == nil {
		return errx.NewDisplayable("missing required attributes field")
	}
	return req.Data.Attributes.validateWithContext(r.Context())
}

func (a *CurrencyMigrationStartAttributes) validateWithContext(ctx context.Context) error {
	fields := []*validation.FieldRules{
		validation.Field(&a.FromCurrency, validation.Required),
		validation.Field(&a.ToCurrency, validation.Required, validation.By(func(any) error {
			if a.FromCurrency != "" && a.FromCurrency == a.ToCurrency {
				return validation.NewError("validation_currency_migration_same_currency", "from and to currencies must differ")
			}
			return nil
		})),
		validation.Field(&a.PreviewToken, validation.Required),
		validation.Field(&a.ExchangeRate, validation.By(func(any) error {
			if a.ExchangeRate.IsZero() || a.ExchangeRate.Sign() < 0 {
				return validation.NewError("validation_currency_migration_rate_positive", "exchange rate must be positive")
			}
			return nil
		})),
	}
	return validation.ValidateStructWithContext(ctx, a, fields...)
}

// CurrencyMigrationPreviewAttributes is the body of the preview
// request. No PreviewToken — the preview endpoint issues a fresh one.
type CurrencyMigrationPreviewAttributes struct {
	FromCurrency models.Currency `json:"from_currency"`
	ToCurrency   models.Currency `json:"to_currency"`
	ExchangeRate decimal.Decimal `json:"exchange_rate"`
}

// CurrencyMigrationPreviewRequest is the POST /currency-migrations/preview body.
type CurrencyMigrationPreviewRequest struct {
	Data *CurrencyMigrationPreviewRequestData `json:"data"`
}

type CurrencyMigrationPreviewRequestData struct {
	Type       string                              `json:"type" example:"currency-migrations" enums:"currency-migrations"`
	Attributes *CurrencyMigrationPreviewAttributes `json:"attributes"`
}

func (req *CurrencyMigrationPreviewRequest) Bind(r *http.Request) error {
	if req.Data == nil {
		return errx.NewDisplayable("missing required data field")
	}
	if req.Data.Type != "currency-migrations" {
		return errx.NewDisplayable("invalid type, expected 'currency-migrations'")
	}
	if req.Data.Attributes == nil {
		return errx.NewDisplayable("missing required attributes field")
	}
	return req.Data.Attributes.validateWithContext(r.Context())
}

func (a *CurrencyMigrationPreviewAttributes) validateWithContext(ctx context.Context) error {
	fields := []*validation.FieldRules{
		validation.Field(&a.FromCurrency, validation.Required),
		validation.Field(&a.ToCurrency, validation.Required, validation.By(func(any) error {
			if a.FromCurrency != "" && a.FromCurrency == a.ToCurrency {
				return validation.NewError("validation_currency_migration_same_currency", "from and to currencies must differ")
			}
			return nil
		})),
		validation.Field(&a.ExchangeRate, validation.By(func(any) error {
			if a.ExchangeRate.IsZero() || a.ExchangeRate.Sign() < 0 {
				return validation.NewError("validation_currency_migration_rate_positive", "exchange rate must be positive")
			}
			return nil
		})),
	}
	return validation.ValidateStructWithContext(ctx, a, fields...)
}

// CurrencyMigrationPreviewDiff is one entry in the preview response's
// diff list. The FE uses these to render the "biggest individual
// changes" table on the preview screen.
type CurrencyMigrationPreviewDiff struct {
	CommodityID            string          `json:"commodity_id"`
	CommodityName          string          `json:"commodity_name"`
	CurrentPriceBefore     decimal.Decimal `json:"current_price_before"`
	CurrentPriceAfter      decimal.Decimal `json:"current_price_after"`
	OriginalPriceBefore    decimal.Decimal `json:"original_price_before"`
	OriginalPriceAfter     decimal.Decimal `json:"original_price_after"`
	OriginalCurrencyBefore models.Currency `json:"original_currency_before"`
	OriginalCurrencyAfter  models.Currency `json:"original_currency_after"`
}

// CurrencyMigrationPreviewBody is the JSON:API attributes of the
// preview response.
type CurrencyMigrationPreviewBody struct {
	FromCurrency        models.Currency                 `json:"from_currency"`
	ToCurrency          models.Currency                 `json:"to_currency"`
	ExchangeRate        decimal.Decimal                 `json:"exchange_rate"`
	CommodityCount      int                             `json:"commodity_count"`
	TotalCurrentBefore  decimal.Decimal                 `json:"total_current_before"`
	TotalCurrentAfter   decimal.Decimal                 `json:"total_current_after"`
	AcquisitionFills    int                             `json:"acquisition_fills"`
	PreviewToken        string                          `json:"preview_token"`
	PreviewExpiresAt    time.Time                       `json:"preview_expires_at"`
	PreviewExpiresInSec int                             `json:"preview_expires_in_seconds"`
	Diffs               []CurrencyMigrationPreviewDiff  `json:"diffs"`
	StateHash           string                          `json:"state_hash"`
}

// CurrencyMigrationPreviewResponse wraps a preview body in the standard
// JSON:API envelope.
type CurrencyMigrationPreviewResponse struct {
	HTTPStatusCode int                                   `json:"-"`
	Data           *CurrencyMigrationPreviewResponseData `json:"data"`
}

type CurrencyMigrationPreviewResponseData struct {
	Type       string                       `json:"type" example:"currency-migration-previews" enums:"currency-migration-previews"`
	Attributes CurrencyMigrationPreviewBody `json:"attributes"`
}

func NewCurrencyMigrationPreviewResponse(body CurrencyMigrationPreviewBody) *CurrencyMigrationPreviewResponse {
	return &CurrencyMigrationPreviewResponse{
		HTTPStatusCode: http.StatusOK,
		Data: &CurrencyMigrationPreviewResponseData{
			Type:       "currency-migration-previews",
			Attributes: body,
		},
	}
}

func (r *CurrencyMigrationPreviewResponse) Render(_w http.ResponseWriter, req *http.Request) error {
	render.Status(req, statusCodeDef(r.HTTPStatusCode, http.StatusOK))
	return nil
}
