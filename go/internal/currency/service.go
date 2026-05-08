// Package currency provides scoped commodity price conversion helpers
// used by the currency-migration apiserver endpoints (preview / start)
// and, in PR 3 of issue #202, by the migration worker.
//
// The conversion logic is split in two:
//
//   - ApplyConversion is a pure function over a single Commodity. It
//     decides Case A / B / C from #202 §2 and produces the post-conversion
//     row image plus a flag indicating whether this is the first time
//     acquisition_price / acquisition_currency would be filled.
//
//   - ConvertGroup is the orchestrator. It reads every commodity in the
//     given (tenant_id, group_id) inside the caller-supplied transaction,
//     applies the pure function, persists the mutated rows, and (when
//     migrationID is non-empty) writes acquisition columns + audit rows
//     in the same tx. The caller decides whether to commit (worker /
//     start handler) or roll back (preview handler).
//
// What this rewrite removes versus the previous service
// (https://github.com/denisvmedia/inventario/issues/202 §4.4):
//
//   - RateProvider, StaticRateProvider, defaultRates — the user types
//     the rate; there is no provider, no fallback table.
//
//   - Compensating per-row rollback — the surrounding PG transaction
//     does that atomically.
//
//   - List() over the global commodity set — replaced by an explicit
//     group-scoped read so the worker (which bypasses RLS) cannot
//     mutate other tenants' rows.
//
// The Case C bug from the previous service (where ConvertedOriginalPrice
// was multiplied by the rate even when OriginalPriceCurrency already
// equalled the target currency, leaving the row in violation of
// PriceRule) is fixed: Case C now collapses ConvertedOriginalPrice to
// zero.
package currency

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var (
	// ErrSameCurrency is returned when from == to. Apiserver maps this
	// to 422 — same-currency runs are rejected before any migration row
	// is written, before any audit work happens, and before any
	// daily-quota slot is consumed (#202 §2).
	ErrSameCurrency = errx.NewSentinel("from and to currencies must differ")

	// ErrInvalidExchangeRate is returned for rates that are not strictly
	// positive, NaN, or above the §2 sanity bound (1e10). Apiserver maps
	// this to 422.
	ErrInvalidExchangeRate = errx.NewSentinel("exchange rate must be positive, finite, and at most 1e10")
)

const (
	// convertedMoneyScale is the decimal scale we round to when converting
	// a price field. Matches the DECIMAL(15,2) column precision.
	convertedMoneyScale = 2

	// maxExchangeRateExp is the §2 sanity bound (1e10) on user-supplied
	// rates. Rejects fat-fingered "1.234e15"-style typos before any
	// commodity rows are read. Stored as decimal so the comparison stays
	// exact.
	maxExchangeRateExp = 10
)

// ApplyOutcome is the case of the per-row decision tree from #202 §2.
// Used by the audit row writer to decide which transitions counted.
type ApplyOutcome int

const (
	// ApplyOutcomeCaseA — original was in old group currency. Live
	// OriginalPrice / OriginalPriceCurrency are overwritten;
	// AcquisitionPrice / AcquisitionCurrency are filled if they were
	// still NULL.
	ApplyOutcomeCaseA ApplyOutcome = iota
	// ApplyOutcomeCaseB — original in a third currency X.
	// OriginalPrice / OriginalPriceCurrency unchanged; the converted /
	// current G-side amounts are scaled by the rate. Acquisition columns
	// untouched.
	ApplyOutcomeCaseB
	// ApplyOutcomeCaseC — original already in target currency. Edge case:
	// the previous service would multiply ConvertedOriginalPrice by the
	// rate, violating PriceRule. We collapse it to zero. Acquisition
	// columns untouched (no original lost — original is already in G_new).
	ApplyOutcomeCaseC
)

// RowImage is the price-related slice of a Commodity captured for the
// audit table. Stored verbatim before and after the conversion so the
// audit row is self-describing.
type RowImage struct {
	OriginalPrice          decimal.Decimal
	OriginalPriceCurrency  models.Currency
	ConvertedOriginalPrice decimal.Decimal
	CurrentPrice           decimal.Decimal
}

// ApplyResult is the outcome of ApplyConversion for one commodity.
//
// FillAcquisition is true iff this is Case A AND the row's acquisition
// columns were still NULL — i.e. this is the migration that captures the
// "as purchased" amount for that commodity. The caller writes the
// columns via migrationops.SetAcquisition only when FillAcquisition is
// true.
type ApplyResult struct {
	Outcome         ApplyOutcome
	Before          RowImage
	After           RowImage
	FillAcquisition bool
	// AcquisitionPrice / AcquisitionCurrency carry the values to fill in
	// when FillAcquisition is true. They are the pre-conversion live
	// original (the acquisition amount, by definition), not the migrated
	// values.
	AcquisitionPrice    decimal.Decimal
	AcquisitionCurrency models.Currency
}

// ApplyConversion is the pure decision tree from #202 §2. It does not
// touch acquisition_price / acquisition_currency on the input — the
// caller writes those via migrationops.SetAcquisition when
// FillAcquisition is true.
func ApplyConversion(commodity models.Commodity, fromCurrency, toCurrency models.Currency, rate decimal.Decimal) ApplyResult {
	before := imageFromCommodity(commodity)

	switch commodity.OriginalPriceCurrency {
	case fromCurrency:
		// Case A — original was in old group currency.
		mutated := commodity
		mutated.OriginalPrice = quantizeConvertedMoney(commodity.OriginalPrice.Mul(rate))
		mutated.OriginalPriceCurrency = toCurrency
		mutated.ConvertedOriginalPrice = decimal.Zero
		mutated.CurrentPrice = quantizeConvertedMoney(commodity.CurrentPrice.Mul(rate))
		return ApplyResult{
			Outcome:             ApplyOutcomeCaseA,
			Before:              before,
			After:               imageFromCommodity(mutated),
			FillAcquisition:     commodity.AcquisitionPrice == nil && commodity.AcquisitionCurrency == nil,
			AcquisitionPrice:    commodity.OriginalPrice,
			AcquisitionCurrency: commodity.OriginalPriceCurrency,
		}

	case toCurrency:
		// Case C — original already in target currency. Collapse
		// ConvertedOriginalPrice to zero so PriceRule's
		// "OriginalCurrency == GroupCurrency ⇒ ConvertedOriginalPrice == 0"
		// invariant continues to hold. CurrentPrice scales by rate
		// (it was previously denominated in G_old).
		mutated := commodity
		mutated.ConvertedOriginalPrice = decimal.Zero
		mutated.CurrentPrice = quantizeConvertedMoney(commodity.CurrentPrice.Mul(rate))
		return ApplyResult{
			Outcome: ApplyOutcomeCaseC,
			Before:  before,
			After:   imageFromCommodity(mutated),
		}

	default:
		// Case B — original in some third currency X. OriginalPrice /
		// OriginalPriceCurrency stay put; the converted / current
		// G-side amounts move.
		mutated := commodity
		mutated.ConvertedOriginalPrice = quantizeConvertedMoney(commodity.ConvertedOriginalPrice.Mul(rate))
		mutated.CurrentPrice = quantizeConvertedMoney(commodity.CurrentPrice.Mul(rate))
		return ApplyResult{
			Outcome: ApplyOutcomeCaseB,
			Before:  before,
			After:   imageFromCommodity(mutated),
		}
	}
}

// PerRowDiff is one entry in RunResult.Diffs. The preview endpoint
// renders the largest |delta| entries to the user as the "biggest
// individual changes" panel. For the worker run we keep the full list
// so audit / debugging stays trivial.
type PerRowDiff struct {
	CommodityID    string
	CommodityName  string
	Outcome        ApplyOutcome
	Before         RowImage
	After          RowImage
	CurrentDelta   decimal.Decimal
	FilledFromCase bool
}

// RunResult is the aggregate output of ConvertGroup. It is the source
// of truth for both the preview response (rendered to the user, then
// rolled back) and the migration row update (written by the worker on
// commit).
type RunResult struct {
	CommodityCount        int
	TotalCurrentBefore    decimal.Decimal
	TotalCurrentAfter     decimal.Decimal
	AcquisitionFillsCount int
	Diffs                 []PerRowDiff
}

// AcquisitionFiller writes the write-once provenance columns for a
// commodity inside the caller-supplied tx. Pass nil to skip the fill
// step (preview, or any caller that does not have the registry-internal
// migrationops package available).
//
// Callers in the worker (PR 3) wire this to migrationops.SetAcquisition.
// The apiserver preview path leaves it nil — preview never writes
// provenance, since the surrounding tx is rolled back.
type AcquisitionFiller func(ctx context.Context, tx *sqlx.Tx, commodityID string, price decimal.Decimal, currency models.Currency) error

// AuditWriter writes one currency_migration_audit_rows row in the
// caller-supplied tx. Pass nil to skip audit writes (preview).
type AuditWriter func(ctx context.Context, row models.CurrencyMigrationAuditRow) error

// RunOptions parameterises one invocation of ConvertGroup.
type RunOptions struct {
	TenantID     string
	GroupID      string
	FromCurrency models.Currency
	ToCurrency   models.Currency
	Rate         decimal.Decimal

	// MigrationID is the currency_migrations.id this run is recording
	// against. Empty string means "preview" — no audit rows are written
	// and no acquisition columns are filled (the surrounding tx will be
	// rolled back anyway, but skipping audit writes keeps the preview
	// path's hot loop tight).
	MigrationID string

	// FillAcquisition is the worker-supplied callback that writes the
	// commodities.acquisition_price / acquisition_currency pair on the
	// first Case-A overwrite. Required when MigrationID is non-empty
	// (worker mode) and never called when MigrationID is empty.
	FillAcquisition AcquisitionFiller

	// Audit writes one row per mutated commodity. Required when
	// MigrationID is non-empty.
	Audit AuditWriter
}

// ConversionService converts commodity prices for a single group.
//
// Stateless wrt. data — every call passes the tx and the (tenantID,
// groupID, rate, …) parameters. The service holds only the table-name
// resolver so it can talk to the right physical tables in tests that
// override DefaultTableNames.
type ConversionService struct {
	tableNames store.TableNames
}

// NewConversionService creates a service backed by the default
// table-name resolver (store.DefaultTableNames). Tests that reroute
// tables can swap this for NewConversionServiceWithTableNames.
func NewConversionService() *ConversionService {
	return &ConversionService{tableNames: store.DefaultTableNames}
}

// NewConversionServiceWithTableNames is the test seam for renaming the
// commodities / audit tables.
func NewConversionServiceWithTableNames(t store.TableNames) *ConversionService {
	return &ConversionService{tableNames: t}
}

// ConvertGroup runs the conversion across every commodity in (tenant,
// group) inside the caller-supplied transaction. The caller commits
// (worker) or rolls back (preview). Returns ErrSameCurrency / ErrInvalidExchangeRate
// before any read happens; both are mapped by the apiserver to 422.
func (s *ConversionService) ConvertGroup(ctx context.Context, tx *sqlx.Tx, opts RunOptions) (RunResult, error) {
	var zero RunResult

	if tx == nil {
		return zero, errxtrace.Wrap("tx is required", registry.ErrFieldRequired)
	}
	if opts.FromCurrency == "" || opts.ToCurrency == "" {
		return zero, errxtrace.Wrap("from and to currencies are required", registry.ErrFieldRequired)
	}
	if opts.FromCurrency == opts.ToCurrency {
		return zero, ErrSameCurrency
	}
	if err := ValidateRate(opts.Rate); err != nil {
		return zero, err
	}
	if opts.TenantID == "" || opts.GroupID == "" {
		return zero, errxtrace.Wrap("tenant id and group id are required", registry.ErrFieldRequired)
	}
	if opts.MigrationID != "" {
		if opts.Audit == nil {
			return zero, errors.New("currency: Audit writer is required when MigrationID is set")
		}
		if opts.FillAcquisition == nil {
			return zero, errors.New("currency: FillAcquisition is required when MigrationID is set")
		}
	}

	commodities, err := s.listGroupCommodities(ctx, tx, opts.TenantID, opts.GroupID)
	if err != nil {
		return zero, errxtrace.Wrap("failed to list commodities for currency conversion", err)
	}

	result := RunResult{
		CommodityCount: len(commodities),
		Diffs:          make([]PerRowDiff, 0, len(commodities)),
	}

	for _, commodity := range commodities {
		result.TotalCurrentBefore = result.TotalCurrentBefore.Add(commodity.CurrentPrice)

		applyResult := ApplyConversion(*commodity, opts.FromCurrency, opts.ToCurrency, opts.Rate)

		// Persist the new commodity row image. We update unconditionally
		// — even Case C with no actual delta benefits from the
		// last_modified-style invariant of "everything in this group
		// touched on this run". The cost is negligible and keeps the
		// audit trail consistent. The UPDATE is defensively scoped to
		// (tenant_id, group_id) so a service-mode worker that bypasses
		// RLS can never mutate a row outside the migration's group even
		// if a wrong ID leaks into the loop.
		if err := s.updateCommodityImage(ctx, tx, opts.TenantID, opts.GroupID, commodity.GetID(), applyResult.After); err != nil {
			return zero, errxtrace.Wrap(fmt.Sprintf("failed to update commodity %s", commodity.GetID()), err)
		}

		// Acquisition fill on first Case-A only when the run is a
		// real migration (MigrationID set). Preview never fills —
		// preview rolls back, and skipping the registry-side guard
		// keeps the preview hot loop tight. The actual SetAcquisition
		// call lives in the worker package (PR 3) so this package
		// stays free of the registry/internal/migrationops import.
		filled := false
		if opts.MigrationID != "" && applyResult.FillAcquisition {
			err := opts.FillAcquisition(
				ctx, tx,
				commodity.GetID(),
				applyResult.AcquisitionPrice,
				applyResult.AcquisitionCurrency,
			)
			switch {
			case err == nil:
				filled = true
				result.AcquisitionFillsCount++
			case errors.Is(err, registry.ErrAcquisitionAlreadySet):
				// Concurrent worker race or a duplicate run — we
				// observed NULL but a sibling tx wrote first. Honour
				// write-once: leave the columns alone, do not count
				// the fill, and continue.
				filled = false
			default:
				return zero, errxtrace.Wrap(fmt.Sprintf("failed to set acquisition columns for %s", commodity.GetID()), err)
			}
		}

		result.TotalCurrentAfter = result.TotalCurrentAfter.Add(applyResult.After.CurrentPrice)
		result.Diffs = append(result.Diffs, PerRowDiff{
			CommodityID:    commodity.GetID(),
			CommodityName:  commodity.Name,
			Outcome:        applyResult.Outcome,
			Before:         applyResult.Before,
			After:          applyResult.After,
			CurrentDelta:   applyResult.After.CurrentPrice.Sub(applyResult.Before.CurrentPrice),
			FilledFromCase: filled,
		})

		if opts.MigrationID != "" {
			row := buildAuditRow(opts, commodity.GetID(), applyResult, filled)
			if err := opts.Audit(ctx, row); err != nil {
				return zero, errxtrace.Wrap("failed to write currency migration audit row", err)
			}
		}
	}

	return result, nil
}

// ValidateRate enforces the §2 rate guards (positive, finite, ≤ 1e10).
// Exposed so the apiserver can reject early — before opening the
// preview tx.
func ValidateRate(rate decimal.Decimal) error {
	if !rate.IsPositive() || rate.IsZero() {
		return ErrInvalidExchangeRate
	}
	maxRate := decimal.New(1, maxExchangeRateExp)
	if rate.GreaterThan(maxRate) {
		return ErrInvalidExchangeRate
	}
	return nil
}

func (s *ConversionService) listGroupCommodities(ctx context.Context, tx *sqlx.Tx, tenantID, groupID string) ([]*models.Commodity, error) {
	query := fmt.Sprintf(
		`SELECT * FROM %s WHERE tenant_id = $1 AND group_id = $2 ORDER BY id ASC`,
		s.tableNames.Commodities(),
	)
	rows, err := tx.QueryxContext(ctx, query, tenantID, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.Commodity
	for rows.Next() {
		var c models.Commodity
		if scanErr := rows.StructScan(&c); scanErr != nil {
			return nil, scanErr
		}
		cc := c
		out = append(out, &cc)
	}
	return out, rows.Err()
}

func (s *ConversionService) updateCommodityImage(ctx context.Context, tx *sqlx.Tx, tenantID, groupID, commodityID string, after RowImage) error {
	// Scope the UPDATE to (tenant_id, group_id) defensively. The
	// surrounding listGroupCommodities already filtered to the same
	// (tenant, group), so this is belt-and-braces: under service-mode
	// RLS bypass a wrong commodity ID coming from a tampered row read
	// must not be able to mutate a row outside the migration scope.
	// We also assert exactly one row was affected.
	query := fmt.Sprintf(
		`UPDATE %s
		    SET original_price = $4,
		        original_price_currency = $5,
		        converted_original_price = $6,
		        current_price = $7
		  WHERE id = $1 AND tenant_id = $2 AND group_id = $3`,
		s.tableNames.Commodities(),
	)
	res, err := tx.ExecContext(ctx, query, commodityID, tenantID, groupID, after.OriginalPrice, string(after.OriginalPriceCurrency), after.ConvertedOriginalPrice, after.CurrentPrice)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows != 1 {
		return fmt.Errorf("commodity %s: expected 1 row updated, got %d", commodityID, rows)
	}
	return nil
}

func buildAuditRow(opts RunOptions, commodityID string, ar ApplyResult, filled bool) models.CurrencyMigrationAuditRow {
	commodityRef := commodityID
	beforePrice := ar.Before.OriginalPrice
	afterPrice := ar.After.OriginalPrice
	beforeConverted := ar.Before.ConvertedOriginalPrice
	afterConverted := ar.After.ConvertedOriginalPrice
	beforeCurrent := ar.Before.CurrentPrice
	afterCurrent := ar.After.CurrentPrice
	beforeCurrency := ar.Before.OriginalPriceCurrency
	afterCurrency := ar.After.OriginalPriceCurrency

	return models.CurrencyMigrationAuditRow{
		MigrationID:                opts.MigrationID,
		CommodityID:                &commodityRef,
		OriginalPriceBefore:        &beforePrice,
		OriginalPriceAfter:         &afterPrice,
		OriginalCurrencyBefore:     &beforeCurrency,
		OriginalCurrencyAfter:      &afterCurrency,
		ConvertedBefore:            &beforeConverted,
		ConvertedAfter:             &afterConverted,
		CurrentBefore:              &beforeCurrent,
		CurrentAfter:               &afterCurrent,
		AcquisitionFilledInThisRun: filled,
	}
}

func imageFromCommodity(c models.Commodity) RowImage {
	return RowImage{
		OriginalPrice:          c.OriginalPrice,
		OriginalPriceCurrency:  c.OriginalPriceCurrency,
		ConvertedOriginalPrice: c.ConvertedOriginalPrice,
		CurrentPrice:           c.CurrentPrice,
	}
}

func quantizeConvertedMoney(amount decimal.Decimal) decimal.Decimal {
	return amount.Round(convertedMoneyScale)
}
