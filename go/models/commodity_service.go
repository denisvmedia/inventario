package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"
	"golang.org/x/text/currency"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*CommodityService)(nil)
	_ validation.ValidatableWithContext = (*CommodityService)(nil)
	_ TenantGroupAwareIDable            = (*CommodityService)(nil)
)

// CommodityService tracks a single send-out of a commodity to a workshop
// or service center. Sibling to CommodityLoan (#1452) and follows the
// same shape: a commodity has at most one OPEN service row
// (`returned_at IS NULL`) at any time, and "in service" is **derived**
// at read time rather than denormalised onto commodities. See #1508 for
// the motivation and the cross-kind invariant (a commodity cannot be
// simultaneously lent out and in service — enforced at the service
// layer via the shared OpenHoldingChecker).
//
// The schema deviates from CommodityLoan in two places:
//   - `provider_*` instead of `borrower_*`. UI emphasises the workshop
//     name + a free-form reason ("screen replacement"), not a side
//     note.
//   - `cost_amount` + `cost_currency` capture the repair bill for
//     cost-of-ownership reporting later. Currency stored alongside so
//     we don't assume the user's default at write time. Both fields
//     are optional but locked together: present iff the other is —
//     a cost without a currency is meaningless and a currency without
//     an amount is dead metadata.
//
// Enable RLS for multi-tenant isolation
//
//migrator:schema:rls:enable table="commodity_services" comment="Enable RLS for multi-tenant commodity service isolation"
//migrator:schema:rls:policy name="commodity_service_isolation" table="commodity_services" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures commodity services can only be accessed and modified by their tenant and group with required contexts"
//migrator:schema:rls:policy name="commodity_service_background_worker_access" table="commodity_services" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all commodity services for processing"
//migrator:schema:table name="commodity_services"
type CommodityService struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID

	// CommodityID — the item being serviced. ON DELETE CASCADE is added
	// manually to the generated migration; mirrors commodity_loans.
	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_commodity_service_commodity" on_delete="CASCADE"
	CommodityID string `json:"commodity_id" db:"commodity_id"`

	// ProviderName is required and free-form ("Apple Authorized Service",
	// "Bob's Repair Shop"). Capped at 200 chars to match similar BE text
	// caps and leave room for indexes if we later add one.
	//migrator:schema:field name="provider_name" type="TEXT" not_null="true"
	ProviderName string `json:"provider_name" db:"provider_name"`

	// ProviderContact is free-form (phone / email / address). No
	// validation — for the user's own reference.
	//migrator:schema:field name="provider_contact" type="TEXT"
	ProviderContact string `json:"provider_contact" db:"provider_contact"`

	// Reason is free-form ("screen replacement", "warranty diagnostics").
	// Optional, but UI-prominent. Capped at 1000 chars.
	//migrator:schema:field name="reason" type="TEXT"
	Reason string `json:"reason" db:"reason"`

	// SentAt is the date the item left for service. Required. Stored as
	// TEXT in YYYY-MM-DD format to match other date fields.
	//migrator:schema:field name="sent_at" type="TEXT" not_null="true"
	SentAt Date `json:"sent_at" db:"sent_at"`

	// ExpectedReturnAt is the optional ETA from the workshop. Nullable
	// for open-ended estimates ("we'll call when ready").
	//migrator:schema:field name="expected_return_at" type="TEXT"
	ExpectedReturnAt PDate `json:"expected_return_at" db:"expected_return_at"`

	// ReturnedAt closes out the service row. Nullable until the item
	// comes back. The "open vs returned" semantics derive from this
	// field alone.
	//migrator:schema:field name="returned_at" type="TEXT"
	ReturnedAt PDate `json:"returned_at" db:"returned_at"`

	// CostAmount is the optional repair bill. Bare decimal.Decimal —
	// zero means "no cost recorded" (per the codebase's existing
	// price-field convention). Pair-validated with CostCurrency:
	// either both unset or both set.
	//migrator:schema:field name="cost_amount" type="DECIMAL(14,2)"
	CostAmount decimal.Decimal `json:"cost_amount" db:"cost_amount"`

	// CostCurrency is the ISO 4217 code for CostAmount. Empty means
	// "no cost recorded" — see CostAmount. Stored as a plain string
	// (not Currency) so the empty-string "unset" form doesn't trip the
	// Currency type's auto-Validatable contract; ISO 4217 is enforced
	// in ValidateWithContext only when a value is supplied.
	//migrator:schema:field name="cost_currency" type="TEXT"
	CostCurrency string `json:"cost_currency" db:"cost_currency"`

	// ReminderSentOverdue + ReminderSentDueSoon are idempotency flags
	// for the reminder worker (separate sub-issue #1509-equivalent for
	// services). Set by the worker only — zero on every Create.
	//migrator:schema:field name="reminder_sent_overdue" type="BOOLEAN" not_null="true" default="false"
	ReminderSentOverdue bool `json:"reminder_sent_overdue" db:"reminder_sent_overdue" userinput:"false"`

	//migrator:schema:field name="reminder_sent_due_soon" type="BOOLEAN" not_null="true" default="false"
	ReminderSentDueSoon bool `json:"reminder_sent_due_soon" db:"reminder_sent_due_soon" userinput:"false"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// CommodityServiceIndexes defines the postgres indexes for commodity_services.
type CommodityServiceIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore).
	//migrator:schema:index name="idx_commodity_services_uuid" fields="uuid" unique="true" table="commodity_services"
	_ int

	// Index for tenant-based queries.
	//migrator:schema:index name="idx_commodity_services_tenant_id" fields="tenant_id" table="commodity_services"
	_ int

	// Composite index for tenant+group RLS-filtered queries.
	//migrator:schema:index name="idx_commodity_services_tenant_group" fields="tenant_id,group_id" table="commodity_services"
	_ int

	// Composite index for per-commodity history reads (the Service tab).
	//migrator:schema:index name="idx_commodity_services_commodity" fields="commodity_id,sent_at" table="commodity_services"
	_ int

	// Partial index for the "currently in service" group view — only
	// open rows. Cuts the index size to the working set.
	//migrator:schema:index name="idx_commodity_services_active" fields="group_id,expected_return_at" condition="returned_at IS NULL" table="commodity_services"
	_ int

	// Partial index for the reminder worker's overdue scan — open
	// services with an expected return in the past.
	//migrator:schema:index name="idx_commodity_services_due" fields="expected_return_at" condition="returned_at IS NULL AND expected_return_at IS NOT NULL" table="commodity_services"
	_ int
}

// IsOpen reports whether the service is currently active (no return logged).
func (s *CommodityService) IsOpen() bool {
	return s.ReturnedAt == nil || *s.ReturnedAt == ""
}

// IsOverdue reports whether the service is open AND past its expected
// return as of `now`. Open services with no expected return are never
// overdue. Mirrors CommodityLoan.IsOverdue.
func (s *CommodityService) IsOverdue(now time.Time) bool {
	if !s.IsOpen() || s.ExpectedReturnAt == nil || *s.ExpectedReturnAt == "" {
		return false
	}
	due := s.ExpectedReturnAt.ToTime()
	if due.IsZero() {
		return false
	}
	return now.After(due)
}

// HasCost reports whether a cost has been recorded. Both fields must be
// set together — see the type-level comment.
func (s *CommodityService) HasCost() bool {
	return !s.CostAmount.IsZero() || s.CostCurrency != ""
}

func (*CommodityService) Validate() error {
	return ErrMustUseValidateWithContext
}

func (s *CommodityService) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, s,
		validation.Field(&s.CommodityID, rules.NotEmpty),
		validation.Field(&s.ProviderName, rules.NotEmpty, validation.Length(1, 200)),
		validation.Field(&s.ProviderContact, validation.Length(0, 200)),
		validation.Field(&s.Reason, validation.Length(0, 1000)),
		validation.Field(&s.SentAt, validation.Required),
		validation.Field(&s.ExpectedReturnAt),
		validation.Field(&s.ReturnedAt),
		validation.Field(&s.CostAmount, validation.By(func(any) error {
			amountSet := !s.CostAmount.IsZero()
			currencySet := s.CostCurrency != ""
			if amountSet != currencySet {
				return validation.NewError("cost_currency_pair_required",
					"cost_amount and cost_currency must be set together")
			}
			if currencySet {
				if _, err := currency.ParseISO(s.CostCurrency); err != nil {
					return validation.NewError("cost_currency_iso_4217",
						"cost_currency must be a valid ISO 4217 code")
				}
			}
			return nil
		})),
	)
}
