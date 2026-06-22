package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"
)

// CurrencyMigrationStatus is the state machine of a per-group currency
// migration, mirroring RestoreOperation. Transitions are
// pending → running → completed | failed (terminal).
type CurrencyMigrationStatus string

const (
	CurrencyMigrationStatusPending   CurrencyMigrationStatus = "pending"
	CurrencyMigrationStatusRunning   CurrencyMigrationStatus = "running"
	CurrencyMigrationStatusCompleted CurrencyMigrationStatus = "completed"
	CurrencyMigrationStatusFailed    CurrencyMigrationStatus = "failed"
)

// IsValid reports whether s is one of the documented statuses. Empty
// string is invalid.
func (s CurrencyMigrationStatus) IsValid() bool {
	switch s {
	case CurrencyMigrationStatusPending,
		CurrencyMigrationStatusRunning,
		CurrencyMigrationStatusCompleted,
		CurrencyMigrationStatusFailed:
		return true
	}
	return false
}

// IsTerminal reports whether s is a terminal state (no further transitions).
func (s CurrencyMigrationStatus) IsTerminal() bool {
	return s == CurrencyMigrationStatusCompleted || s == CurrencyMigrationStatusFailed
}

var (
	_ validation.Validatable            = (*CurrencyMigration)(nil)
	_ validation.ValidatableWithContext = (*CurrencyMigration)(nil)
	_ TenantGroupAwareIDable            = (*CurrencyMigration)(nil)
)

// CurrencyMigration is the operation row for a single re-pricing of a
// LocationGroup's commodities from one currency to another (issue #202).
// The lifecycle is two-tx (TX1 claim+running, TX2 work+complete) so a
// crashed worker leaves a `running` row that the periodic recovery sweep
// can flip to `failed`.
//
// Only group admins may create / list / get migrations; the API surface is
// gated behind the `feature.currency_migration` flag (introduced in [02/04]).
//
//migrator:schema:rls:enable table="currency_migrations" comment="Enable RLS for multi-tenant currency migration isolation"
//migrator:schema:rls:policy name="currency_migration_isolation" table="currency_migrations" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures currency migration rows can only be accessed and modified by their tenant and group with required contexts"
//migrator:schema:rls:policy name="currency_migration_background_worker_access" table="currency_migrations" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to claim, advance, and recover currency migration rows"

// Same-currency rows are rejected by ValidateWithContext below + the
// apiserver layer (422 before the row is ever inserted). A schema
// CHECK would be a nice defence-in-depth, but ptah's walker.go does
// NOT bubble Database.Constraints from per-file ParseFS results, so a
// `migrator:schema:constraint` annotation drifts vs the live DB on
// every drift check. Re-add when the upstream walker is fixed.
//
//migrator:schema:table name="currency_migrations"
type CurrencyMigration struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID

	// Status — see CurrencyMigrationStatus. Worker writes durable
	// transitions; status is never user-input.
	//migrator:schema:field name="status" type="TEXT" not_null="true"
	Status CurrencyMigrationStatus `json:"status" db:"status" userinput:"false"`

	// FromCurrency is the group currency at the moment the migration was
	// scheduled. The worker re-validates against the live group state
	// inside TX2 and aborts if it has drifted.
	//migrator:schema:field name="from_currency" type="TEXT" not_null="true"
	FromCurrency Currency `json:"from_currency" db:"from_currency"`

	// ToCurrency is the target group currency. CHECK constraint enforces
	// from_currency <> to_currency at the DB level.
	//migrator:schema:field name="to_currency" type="TEXT" not_null="true"
	ToCurrency Currency `json:"to_currency" db:"to_currency"`

	// ExchangeRate is the user-entered rate (1 from = rate to). DECIMAL(20,10)
	// gives plenty of headroom; the FE clamps to 6 decimals as a UX guard.
	//migrator:schema:field name="exchange_rate" type="DECIMAL(20,10)" not_null="true"
	ExchangeRate decimal.Decimal `json:"exchange_rate" db:"exchange_rate"`

	// CommodityCount is the number of rows actually mutated by the worker.
	//migrator:schema:field name="commodity_count" type="INTEGER" not_null="true" default="0"
	CommodityCount int `json:"commodity_count" db:"commodity_count" userinput:"false"`

	// TotalBefore / TotalAfter are sums of CurrentPrice across the
	// group's commodities, captured for audit. Nullable until TX2 has
	// computed them.
	//migrator:schema:field name="total_before" type="DECIMAL(20,2)"
	TotalBefore *decimal.Decimal `json:"total_before,omitempty" db:"total_before" userinput:"false"`
	//migrator:schema:field name="total_after" type="DECIMAL(20,2)"
	TotalAfter *decimal.Decimal `json:"total_after,omitempty" db:"total_after" userinput:"false"`

	// PreviewToken is the HMAC issued by the preview endpoint and posted
	// back at commit time. Persisted for audit only — verification is
	// stateless and recomputed from the same key.
	//migrator:schema:field name="preview_token" type="TEXT"
	PreviewToken *string `json:"preview_token,omitempty" db:"preview_token" userinput:"false"`

	// PreviewExpiresAt is the expiry timestamp embedded in PreviewToken.
	//migrator:schema:field name="preview_expires_at" type="TIMESTAMP"
	PreviewExpiresAt *time.Time `json:"preview_expires_at,omitempty" db:"preview_expires_at" userinput:"false"`

	// CreatedAt is the moment the row was inserted (still in pending status).
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	// StartedAt is set when the worker flips the row to `running` (TX1).
	//migrator:schema:field name="started_at" type="TIMESTAMP"
	StartedAt *time.Time `json:"started_at,omitempty" db:"started_at" userinput:"false"`

	// CompletedAt is set on either terminal status — TX2 commit on
	// success or the recovery sweep on failure. Captures the moment the
	// row left the `running` state.
	//migrator:schema:field name="completed_at" type="TIMESTAMP"
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at" userinput:"false"`

	// ErrorMessage is populated when status=failed.
	//migrator:schema:field name="error_message" type="TEXT"
	ErrorMessage string `json:"error_message,omitempty" db:"error_message" userinput:"false"`
}

// CurrencyMigrationIndexes defines indexes on currency_migrations.
//
// The partial unique index on `(group_id) WHERE status IN ('pending', 'running')`
// is the schema-level guard against the simultaneous-start race: two
// parallel start attempts cannot each insert a pending row, because the
// second INSERT trips the unique violation. The registry maps the SQLState
// to ErrMigrationInFlight so the apiserver can surface 409
// `migration_in_progress`.
type CurrencyMigrationIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore).
	//migrator:schema:index name="idx_currency_migrations_uuid" fields="uuid" unique="true" table="currency_migrations"
	_ int

	// Composite index for tenant + group lookups (history list).
	//migrator:schema:index name="idx_currency_migrations_tenant_group" fields="tenant_id,group_id" table="currency_migrations"
	_ int

	// Composite index for worker queries (group + status, e.g. ClaimNextPending).
	//migrator:schema:index name="idx_currency_migrations_group_status" fields="group_id,status" table="currency_migrations"
	_ int

	// Partial index that backs the daily-cap query
	// (CompletedTodayForGroup): scans only the small completed-today
	// slice instead of the full history.
	//migrator:schema:index name="idx_currency_migrations_group_completed" fields="group_id,completed_at" condition="status = 'completed'" table="currency_migrations"
	_ int

	// Partial unique index — at most one pending|running row per group.
	// Closes the simultaneous-start race; PG unique-violation maps to
	// ErrMigrationInFlight at the registry layer.
	//migrator:schema:index name="idx_currency_migrations_group_in_flight" fields="group_id" unique="true" condition="status IN ('pending', 'running')" table="currency_migrations"
	_ int
}

// NewCurrencyMigrationFromUserInput sanitises the input row and stamps
// the server-side defaults (status=pending, created_at=now). The user
// only supplies (from, to, rate); everything else is cleared.
func NewCurrencyMigrationFromUserInput(m *CurrencyMigration) CurrencyMigration {
	result := *m

	SanitizeUserInput(&result)

	result.CreatedAt = time.Now().UTC()
	result.Status = CurrencyMigrationStatusPending
	return result
}

func (*CurrencyMigration) Validate() error {
	return ErrMustUseValidateWithContext
}

func (m *CurrencyMigration) ValidateWithContext(ctx context.Context) error {
	fields := []*validation.FieldRules{
		validation.Field(&m.Status, validation.Required),
		validation.Field(&m.FromCurrency, validation.Required),
		validation.Field(&m.ToCurrency, validation.Required, validation.By(func(any) error {
			if m.FromCurrency != "" && m.FromCurrency == m.ToCurrency {
				return validation.NewError("validation_currency_migration_same_currency", "from and to currencies must differ")
			}
			return nil
		})),
		validation.Field(&m.ExchangeRate, validation.Required, validation.By(func(any) error {
			if m.ExchangeRate.IsZero() || m.ExchangeRate.Sign() < 0 {
				return validation.NewError("validation_currency_migration_rate_positive", "exchange rate must be positive")
			}
			return nil
		})),
		validation.Field(&m.ErrorMessage, validation.Length(0, 2000)),
	}

	return validation.ValidateStructWithContext(ctx, m, fields...)
}

var (
	_ TenantGroupAwareIDable = (*CurrencyMigrationAuditRow)(nil)
)

// CurrencyMigrationAuditRow is the per-commodity before/after image
// written inside the same TX2 that mutates the commodity. Audit-only,
// kept forever (volume is bounded by the daily cap; retention is not a
// concern at personal-inventory scale). The commodity FK is
// ON DELETE SET NULL so the audit row outlives the commodity.
//
//migrator:schema:rls:enable table="currency_migration_audit_rows" comment="Enable RLS for currency migration audit rows"
//migrator:schema:rls:policy name="currency_migration_audit_isolation" table="currency_migration_audit_rows" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures currency migration audit rows are isolated by tenant and group"
//migrator:schema:rls:policy name="currency_migration_audit_background_worker_access" table="currency_migration_audit_rows" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows the background worker to insert audit rows in TX2"

//migrator:schema:table name="currency_migration_audit_rows"
type CurrencyMigrationAuditRow struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID

	//migrator:schema:field name="migration_id" type="TEXT" not_null="true" foreign="currency_migrations(id)" foreign_key_name="fk_currency_migration_audit_migration" on_delete="CASCADE"
	MigrationID string `json:"migration_id" db:"migration_id"`

	// CommodityID is nullable on purpose — see ON DELETE SET NULL on the
	// FK. The migration that produced the row is preserved verbatim even
	// if the commodity is deleted later.
	//migrator:schema:field name="commodity_id" type="TEXT" foreign="commodities(id)" foreign_key_name="fk_currency_migration_audit_commodity" on_delete="SET NULL"
	CommodityID *string `json:"commodity_id,omitempty" db:"commodity_id"`

	// Before / After images of the four price-related fields. Stored
	// regardless of which Case (A/B/C in #202 §2) applied so the audit
	// is self-describing without joining to the commodities table.
	//migrator:schema:field name="original_price_before" type="DECIMAL(15,2)"
	OriginalPriceBefore *decimal.Decimal `json:"original_price_before,omitempty" db:"original_price_before"`
	//migrator:schema:field name="original_price_after" type="DECIMAL(15,2)"
	OriginalPriceAfter *decimal.Decimal `json:"original_price_after,omitempty" db:"original_price_after"`
	//migrator:schema:field name="original_currency_before" type="TEXT"
	OriginalCurrencyBefore *Currency `json:"original_currency_before,omitempty" db:"original_currency_before"`
	//migrator:schema:field name="original_currency_after" type="TEXT"
	OriginalCurrencyAfter *Currency `json:"original_currency_after,omitempty" db:"original_currency_after"`
	//migrator:schema:field name="converted_before" type="DECIMAL(15,2)"
	ConvertedBefore *decimal.Decimal `json:"converted_before,omitempty" db:"converted_before"`
	//migrator:schema:field name="converted_after" type="DECIMAL(15,2)"
	ConvertedAfter *decimal.Decimal `json:"converted_after,omitempty" db:"converted_after"`
	//migrator:schema:field name="current_before" type="DECIMAL(15,2)"
	CurrentBefore *decimal.Decimal `json:"current_before,omitempty" db:"current_before"`
	//migrator:schema:field name="current_after" type="DECIMAL(15,2)"
	CurrentAfter *decimal.Decimal `json:"current_after,omitempty" db:"current_after"`

	// AcquisitionFilledInThisRun is true iff this migration was the one
	// that populated commodities.acquisition_price / acquisition_currency
	// (i.e. they were NULL before and became non-NULL after). Used by
	// the metrics counter `inventario_currency_migration_acquisition_fills_total`.
	//migrator:schema:field name="acquisition_filled_in_this_run" type="BOOLEAN" not_null="true" default="false"
	AcquisitionFilledInThisRun bool `json:"acquisition_filled_in_this_run" db:"acquisition_filled_in_this_run"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`
}

// CurrencyMigrationAuditRowIndexes defines indexes on currency_migration_audit_rows.
type CurrencyMigrationAuditRowIndexes struct {
	//migrator:schema:index name="idx_currency_migration_audit_uuid" fields="uuid" unique="true" table="currency_migration_audit_rows"
	_ int

	//migrator:schema:index name="idx_currency_migration_audit_migration" fields="migration_id" table="currency_migration_audit_rows"
	_ int

	//migrator:schema:index name="idx_currency_migration_audit_commodity" fields="commodity_id" table="currency_migration_audit_rows"
	_ int

	//migrator:schema:index name="idx_currency_migration_audit_tenant_group" fields="tenant_id,group_id" table="currency_migration_audit_rows"
	_ int
}
